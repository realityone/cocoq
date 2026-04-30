package anthropic

import (
	"crypto/tls"
	"net/http"
	"strings"

	"cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

var anthropicProxyDomains = sets.New(
	"api.anthropic.com",
	"downloads.claude.ai",
	"platform.claude.com",
	"openrouter.ai",
)

type anthropicProxy struct {
	ca tls.Certificate

	onRequest  []proxy.Handler[proxy.ReqCtx]
	onResponse []proxy.Handler[proxy.RespCtx]
}

func NewAnthropicProxy(ca tls.Certificate) *anthropicProxy {
	p := &anthropicProxy{
		ca: ca,
	}
	p.onRequest = append(
		p.onRequest,
		caching1h{},
		currentDateUTC{},
	)
	return p
}

func (p *anthropicProxy) Domains() sets.Set[string] {
	return anthropicProxyDomains
}

func (p *anthropicProxy) Install(server *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Anthropic proxy for domains: %+v", anthropicProxyDomains.UnsortedList())
	server.OnRequest(proxy.DstHostInSet(anthropicProxyDomains)).HandleConnect(proxy.NewMitmConnectAction(p.ca))

	// Reject event logging requests
	server.OnRequest(anthropicEventLoggingCondition()).
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
	server.OnRequest(anthropicV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnRequest(openRouterV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(anthropicV1MessagesCondition()).DoFunc(p.handleResponse)
}

func (p *anthropicProxy) handleRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return proxy.HandleOnRequest(req, ctx, p.onRequest...)
}

func (p *anthropicProxy) handleResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	return proxy.HandleOnResponse(resp, ctx, p.onResponse...)
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

func openRouterV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "openrouter.ai" &&
			strings.ToLower(req.URL.Path) == "/api/v1/messages"
	}
}
