package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	serverdb "cocoq/server/database"
	"cocoq/server/database/dbrt"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/har"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	cocoqDirName = ".cocoq"
	caCertFile   = "ca.crt"
	caKeyFile    = "ca.key"
)

type Config struct {
	Addr    string
	DBPath  string
	HARFile string
	Verbose bool
	Logger  *logrus.Logger
}

type Server struct {
	addr   string
	logger *logrus.Logger
	http   *http.Server
	db     *dbrt.Client
}

func New(cfg Config) (*Server, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	ca, err := loadOrCreateCA()
	if err != nil {
		return nil, errors.Wrap(err, "load or create root CA")
	}

	db, err := serverdb.OpenClient(cfg.DBPath)
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	proxy := goproxy.NewProxyHttpServer()
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
		proxy.OnRequest().DoFunc(harLogger.OnRequest)
		proxy.OnResponse().DoFunc(harLogger.OnResponse)
	}

	proxy.Verbose = cfg.Verbose
	proxy.Logger = &proxyLogger{logger: logger}
	anthropicProxy := newAnthropicProxy(ca, db)
	anthropicProxy.install(proxy)

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		logrus.WithFields(requestLogFields(req, ctx)).Info("proxy request")
		return req, nil
	})
	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp == nil {
			return nil
		}
		logrus.WithFields(responseLogFields(resp, ctx)).Info("proxy response")
		return resp
	})

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           proxy,
		ReadHeaderTimeout: 2 * time.Hour,
	}

	return &Server{
		addr:   cfg.Addr,
		logger: logger,
		http:   httpServer,
		db:     db,
	}, nil
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
	if s.http != nil {
		if err := s.http.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "shutdown HTTP server")
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return errors.Wrap(err, "close database")
		}
	}
	return nil
}

type proxyLogger struct {
	logger *logrus.Logger
}

func (l *proxyLogger) Printf(format string, args ...any) {
	l.logger.Printf(format, args...)
}
