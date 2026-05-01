package anthropic

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/sets"
)

const defaultOpenRouterProvider = "anthropic"

type openrouterProxy struct {
	*anthropicProxy
	provider string
}

func NewOpenrouterProxy(ca tls.Certificate) *openrouterProxy {
	p := &openrouterProxy{
		anthropicProxy: NewAnthropicProxy(ca),
		provider:       defaultOpenRouterProvider,
	}
	p.onRequest = append(
		p.onRequest,
		proxy.HandlerFunc[proxy.ReqCtx](p.stickProvider),
	)
	p.onResponse = []proxy.Handler[proxy.RespCtx]{
		proxy.HandlerFunc[proxy.RespCtx](p.extactUsage),
	}
	return p
}

func (p *openrouterProxy) Domains() sets.Set[string] {
	domains := p.anthropicProxy.Domains()
	domains.Insert("openrouter.ai")
	return domains
}

func (p *openrouterProxy) Install(server *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Openrouter proxy for domains: %+v", p.Domains().UnsortedList())
	server.OnRequest(proxy.DstHostInSet(p.Domains())).HandleConnect(proxy.NewMitmConnectAction(p.ca))

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
	server.OnRequest(openRouterV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(openRouterV1MessagesCondition()).DoFunc(p.handleResponse)
}

func (p *openrouterProxy) stickProvider(ctx *proxy.OnContext[proxy.ReqCtx]) {
	if p.provider == "" {
		return
	}
	req := ctx.Opaque.Request
	if req.Body == nil {
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read openrouter request body for provider stickiness")
		return
	}
	defer req.Body.Close()

	nextBody, changed, err := p.stickProviderInBody(body)
	if err != nil {
		logrus.WithError(err).Warn("failed to rewrite openrouter request body provider")
		nextBody = body
	}
	if changed {
		logrus.WithField("provider", p.provider).Debug("forced openrouter provider")
	}

	proxy.ReplaceRequestBody(req, nextBody)
	ctx.Opaque.PostRequest = req
}

func (p *openrouterProxy) stickProviderInBody(body []byte) ([]byte, bool, error) {
	if !gjson.ValidBytes(body) {
		return body, false, nil
	}

	provider, _ := json.Marshal(struct {
		Only           []string `json:"only"`
		AllowFallbacks bool     `json:"allow_fallbacks"`
	}{
		Only:           []string{p.provider},
		AllowFallbacks: false,
	})

	nextBody, err := sjson.SetRawBytes(body, "provider", provider)
	if err != nil {
		return nil, false, err
	}
	return nextBody, true, nil
}

func (p *openrouterProxy) extactUsage(ctx *proxy.OnContext[proxy.RespCtx]) {
	// TBD
}

func openRouterV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "openrouter.ai" &&
			strings.ToLower(req.URL.Path) == "/api/v1/messages"
	}
}
