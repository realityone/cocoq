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
}

func newAnthropicProxy(ca tls.Certificate) *anthropicProxy {
	return &anthropicProxy{ca: ca}
}

func (p *anthropicProxy) install(proxy *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Anthropic proxy for domains: %+v", anthropicProxyDomains.UnsortedList())
	proxy.OnRequest(DstHostInSet(anthropicProxyDomains)).HandleConnect(newMitmConnectAction(p.ca))

	// Reject plaintext requests to non-Anthropic domains
	proxy.OnRequest(goproxy.Not(DstHostInSet(anthropicProxyDomains))).
		DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			logrus.WithFields(requestLogFields(req, ctx)).Infof("Rejecting plaintext request to %s", req.URL.String())
			return req, goproxy.NewResponse(req, "application/json", http.StatusNotAcceptable, http.StatusText(http.StatusNotAcceptable))
		})
	// Reject connect requests to non-Anthropic domains
	proxy.OnRequest(goproxy.Not(DstHostInSet(anthropicProxyDomains))).
		HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			logrus.WithFields(requestLogFields(ctx.Req, ctx)).Infof("Rejecting connect to %s", host)
			return goproxy.RejectConnect, host
		})

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
	// Handle API requests
	proxy.OnRequest(DstHostInSet(anthropicProxyDomains)).DoFunc(p.handleAPIRequest)
	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response { return resp })
}

func (p *anthropicProxy) handleAPIRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	logrus.WithFields(requestLogFields(req, ctx)).Infof("Handling API request to %s", req.URL.String())
	return req, nil
}

func anthropicEventLoggingCondition() goproxy.ReqConditionFunc {
	paths := sets.New(
		"/api/event_logging/batch",
		"/api/event_logging/v2/batch",
	)
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "api.anthropic.com" && paths.Has(req.URL.Path)
	}
}

func DstHostInSet(hostSet sets.Set[string]) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return hostSet.Has(strings.ToLower(req.URL.Hostname()))
	}
}
