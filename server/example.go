package server

import (
	"crypto/tls"
	"net/http"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

var exampleProxyDomains = sets.New("example.com", "example.org")

type exampleProxy struct {
	ca tls.Certificate
}

func newExampleProxy(ca tls.Certificate) *exampleProxy {
	return &exampleProxy{ca: ca}
}

func (p *exampleProxy) Domains() sets.Set[string] {
	return exampleProxyDomains
}

func (p *exampleProxy) install(proxy *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing example proxy for domains: %+v", exampleProxyDomains.UnsortedList())
	proxy.OnRequest(DstHostInSet(exampleProxyDomains)).HandleConnect(newMitmConnectAction(p.ca))
	proxy.OnRequest(DstHostInSet(exampleProxyDomains)).DoFunc(p.handleRequest)
	proxy.OnResponse(DstHostInSet(exampleProxyDomains)).DoFunc(p.handleResponse)
}

func (p *exampleProxy) handleRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	logrus.WithFields(requestLogFields(req, ctx)).Info("example proxy request")
	return req, nil
}

func (p *exampleProxy) handleResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	logrus.WithFields(responseLogFields(resp, ctx)).Info("example proxy response")
	return resp
}
