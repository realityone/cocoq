package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/realityone/cocoq/server/anthropic"
	"github.com/realityone/cocoq/server/database"
	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/har"
	"github.com/pkg/errors"
	utls "github.com/refraction-networking/utls"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	cocoqDirName = ".cocoq"
	caCertFile   = "ca.crt"
	caKeyFile    = "ca.key"
)

var defaultUpstreamTLSClientHelloID = utls.HelloChrome_Auto

type Config struct {
	Addr         string
	HARFile      string
	Verbose      bool
	DatabasePath string
	Logger       *logrus.Logger
}

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

func New(cfg Config) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	ca, err := proxy.LoadOrCreateCA(cocoqDirName, caCertFile, caKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "load or create root CA")
	}

	db, err := database.OpenClient(cfg.DatabasePath)
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	server := goproxy.NewProxyHttpServer()
	configureUpstreamTLS(server)
	server.Verbose = cfg.Verbose
	server.Logger = &proxyLogger{logger: logger}
	proxyServices := []ProxyService{
		anthropic.NewOpenrouterProxy(ca, db),
		newExampleProxy(ca),
	}
	proxyDomains := sets.New[string]()
	for _, ps := range proxyServices {
		proxyDomains = proxyDomains.Union(ps.Domains())
		ps.Install(server)
	}

	if cfg.HARFile != "" {
		harLogger := har.NewLogger(
			func(entries []har.Entry) {
				logrus.Infof("HAR exported %d entries", len(entries))
				if len(entries) == 0 {
					return
				}
				fp, err := os.OpenFile(cfg.HARFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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

func configureUpstreamTLS(server *goproxy.ProxyHttpServer) {
	server.UpstreamTLSClientHelloID = &defaultUpstreamTLSClientHelloID
	server.ConfigureTransport()
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
