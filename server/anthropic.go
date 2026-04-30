package server

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

var anthropicProxyDomains = sets.New(
	"api.anthropic.com",
	"downloads.claude.ai",
	"platform.claude.com",
)

type anthropicProxy struct {
	ca tls.Certificate

	onRequest  []HandlerFunc[reqCtx]
	onResponse []HandlerFunc[respCtx]
}

func newAnthropicProxy(ca tls.Certificate) *anthropicProxy {
	return &anthropicProxy{ca: ca}
}

func (p *anthropicProxy) install(proxy *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Anthropic proxy for domains: %+v", anthropicProxyDomains.UnsortedList())
	proxy.OnRequest(DstHostInSet(anthropicProxyDomains)).HandleConnect(newMitmConnectAction(p.ca))

	// Reject event logging requests
	proxy.OnRequest(anthropicEventLoggingCondition()).
		DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			logrus.WithFields(logrus.Fields{
				"session": ctx.Session,
				"service": "anthropic",
				"host":    req.Host,
				"url":     req.URL.String(),
				"method":  req.Method,
			}).Info("rejected anthropic event logging request")
			return req, goproxy.NewResponse(req, "application/json", http.StatusNotFound, http.StatusText(http.StatusNotFound))
		})
	// Handle /v1/messages API requests
	proxy.OnRequest(anthropicV1MessagesCondition()).DoFunc(p.handleRequest)
	proxy.OnResponse(anthropicV1MessagesCondition()).DoFunc(p.handleResponse)
}

func (p *anthropicProxy) handleRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return handleOnRequest(req, ctx, p.onRequest...)
}

func (p *anthropicProxy) handleResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	return handleOnResponse(resp, ctx, p.onResponse...)
}

func anthropicEventLoggingCondition() goproxy.ReqConditionFunc {
	paths := sets.New(
		"/api/event_logging/batch",
		"/api/event_logging/v2/batch",
	)
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "api.anthropic.com" &&
			paths.Has(strings.ToLower(req.URL.Path))
	}
}

func anthropicV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "api.anthropic.com" &&
			strings.ToLower(req.URL.Path) == "/v1/messages"
	}
}

func DstHostInSet(hostSet sets.Set[string]) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return hostSet.Has(strings.ToLower(req.URL.Hostname()))
	}
}
