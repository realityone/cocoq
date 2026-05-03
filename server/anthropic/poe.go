package anthropic

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// https://api.poe.com/v1/messages?beta=true

type poeProxy struct {
	*anthropicProxy
}

func NewPoeProxy(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) (*poeProxy, error) {
	p := &poeProxy{
		anthropicProxy: NewAnthropicProxy(ca, db, nil),
	}
	return p, nil
}

func (p *poeProxy) Domains() sets.Set[string] {
	domains := anthropicProxyDomains.Clone()
	domains.Insert("api.poe.com")
	return domains
}

func (p *poeProxy) Install(server *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Poe proxy for domains: %+v", p.Domains().UnsortedList())
	server.OnRequest(proxy.DstHostInSet(p.Domains())).HandleConnect(proxy.NewMitmConnectAction(p.ca))

	// Reject event logging requests
	server.OnRequest(anthropicEventLoggingCondition()).DoFunc(p.handleEventLogging)
	// Handle /v1/messages API requests
	server.OnRequest(poeV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(poeV1MessagesCondition()).DoFunc(p.handleResponse)
}

func poeV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "api.poe.com" &&
			strings.ToLower(req.URL.Path) == "/v1/messages"
	}
}
