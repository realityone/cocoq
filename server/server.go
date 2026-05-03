package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/realityone/cocoq/config"
	"github.com/realityone/cocoq/server/anthropic"
	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/har"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Server struct {
	addr   string
	db     *dbrt.Client
	logger *logrus.Logger
	http   *http.Server
}

type ProxyService interface {
	Domains() sets.Set[string]
	Install(*goproxy.ProxyHttpServer)
}

type apiServiceFactory func(tls.Certificate, *dbrt.Client, json.RawMessage) (ProxyService, error)

var apiServiceFactories = map[string]apiServiceFactory{
	appconfig.APIServiceAnthropic: func(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) (ProxyService, error) {
		return anthropic.NewAnthropicProxy(ca, db, options), nil
	},
	appconfig.APIServiceOpenRouter: func(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) (ProxyService, error) {
		return anthropic.NewOpenrouterProxy(ca, db, options)
	},
	appconfig.APIServicePoe: func(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) (ProxyService, error) {
		return anthropic.NewPoeProxy(ca, db, options)
	},
}

func New(cfg appconfig.ServerConfig, db *dbrt.Client) (*Server, error) {
	if db == nil {
		return nil, errors.New("database client is required")
	}
	if err := validateAPIServiceConfigs(cfg.APIServices); err != nil {
		return nil, err
	}

	logger := logrus.New()

	ca, err := proxy.LoadOrCreateCA(cfg.RootDir, cfg.CA.CertFile, cfg.CA.KeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "load or create root CA")
	}

	server := goproxy.NewProxyHttpServer()
	server.Verbose = cfg.Verbose
	server.Logger = &proxyLogger{logger: logger}
	proxyServices, err := newAPIServices(ca, db, cfg.APIServices)
	if err != nil {
		return nil, err
	}
	proxyServices = append(proxyServices, newExampleProxy(ca))
	proxyDomains := sets.New[string]()
	for _, ps := range proxyServices {
		proxyDomains = proxyDomains.Union(ps.Domains())
		ps.Install(server)
	}

	harFile := appconfig.FilePath(cfg.RootDir, cfg.HARFile)
	if harFile != "" {
		harLogger := har.NewLogger(
			func(entries []har.Entry) {
				logrus.Infof("HAR exported %d entries", len(entries))
				if len(entries) == 0 {
					return
				}
				if err := os.MkdirAll(filepath.Dir(harFile), 0o700); err != nil {
					logrus.Errorf("failed to create HAR directory: %v", err)
					return
				}
				fp, err := os.OpenFile(harFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					logrus.Errorf("failed to open HAR file: %v", err)
					return
				}
				defer fp.Close()
				for _, e := range entries {
					if err := json.NewEncoder(fp).Encode(e); err != nil {
						logrus.Errorf("failed to encode HAR entry: %v", err)
						return
					}
				}
			},
			har.WithExportInterval(5*time.Second),
			har.WithExportThreshold(32),
		)
		server.OnRequest(proxy.DstHostInSet(proxyDomains)).DoFunc(harLogger.OnRequest)
		server.OnResponse(proxy.DstHostInSet(proxyDomains)).DoFunc(harLogger.OnResponse)
	}

	server.OnRequest(proxy.DstHostInSet(proxyDomains)).
		DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			logrus.WithFields(requestLogFields(req, ctx)).Info("received request")
			return req, nil
		})
	// Any request that reaches this point is not handled by any proxy service, so we reject it to prevent unintended proxying.
	server.OnRequest(goproxy.Not(proxy.DstHostInSet(proxyDomains))).
		DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			logrus.WithFields(requestLogFields(req, ctx)).Infof("Rejecting plaintext request to %s", req.URL.String())
			return req, goproxy.NewResponse(req, "application/json", http.StatusNotAcceptable, http.StatusText(http.StatusNotAcceptable))
		})
	server.OnRequest(goproxy.Not(proxy.DstHostInSet(proxyDomains))).
		HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			logrus.WithFields(requestLogFields(ctx.Req, ctx)).Infof("Rejecting connect to %s", host)
			return goproxy.RejectConnect, host
		})

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server,
		ReadHeaderTimeout: 2 * time.Hour,
	}

	return &Server{
		addr:   cfg.Addr,
		db:     db,
		logger: logger,
		http:   httpServer,
	}, nil
}

func newAPIServices(ca tls.Certificate, db *dbrt.Client, configs []appconfig.APIServiceConfig) ([]ProxyService, error) {
	if err := validateAPIServiceConfigs(configs); err != nil {
		return nil, err
	}
	configs = normalizeAPIServiceConfigs(configs)

	services := make([]ProxyService, 0, len(configs))
	for _, cfg := range configs {
		factory, err := resolveAPIServiceFactory(cfg.Name)
		if err != nil {
			return nil, err
		}
		service, err := factory(ca, db, cfg.Options)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

func validateAPIServiceConfigs(configs []appconfig.APIServiceConfig) error {
	seen := map[string]struct{}{}
	for _, cfg := range normalizeAPIServiceConfigs(configs) {
		name := normalizeAPIServiceName(cfg.Name)
		if _, err := resolveAPIServiceFactory(name); err != nil {
			return err
		}
		if _, ok := seen[name]; ok {
			return errors.Errorf("duplicate API service %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func normalizeAPIServiceConfigs(configs []appconfig.APIServiceConfig) []appconfig.APIServiceConfig {
	if len(configs) > 0 {
		return configs
	}
	return []appconfig.APIServiceConfig{{Name: appconfig.APIServiceOpenRouter}}
}

func resolveAPIServiceFactory(name string) (apiServiceFactory, error) {
	name = normalizeAPIServiceName(name)
	factory, ok := apiServiceFactories[name]
	if !ok {
		return nil, errors.Errorf("unsupported API service %q", name)
	}
	return factory, nil
}

func normalizeAPIServiceName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return appconfig.APIServiceOpenRouter
	}
	return name
}

func (s *Server) Run() error {
	s.logger.WithFields(logrus.Fields{
		"addr":     s.addr,
		"protocol": "http",
	}).Info("starting proxy server")

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Wrap(err, "serve HTTP proxy")
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error
	if s.http != nil {
		if err := s.http.Shutdown(ctx); err != nil {
			shutdownErr = errors.Wrap(err, "shutdown HTTP server")
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil && shutdownErr == nil {
			shutdownErr = errors.Wrap(err, "close database")
		}
		s.db = nil
	}
	return shutdownErr
}

type proxyLogger struct {
	logger *logrus.Logger
}

func (l *proxyLogger) Printf(format string, args ...any) {
	l.logger.Printf(format, args...)
}
