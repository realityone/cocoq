package anthropic

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/sets"
)

const defaultOpenRouterProvider = "anthropic"

type openrouterProxy struct {
	*anthropicProxy
	provider string
}

type OpenrouterOptions struct {
	Provider string
}

func NewOpenrouterProxy(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) (*openrouterProxy, error) {
	var opts OpenrouterOptions
	if len(options) > 0 {
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("decode openrouter API service options: %w", err)
		}
	}
	return newOpenrouterProxyWithOptions(ca, db, opts), nil
}

func newOpenrouterProxyWithOptions(ca tls.Certificate, db *dbrt.Client, opts OpenrouterOptions) *openrouterProxy {
	provider := opts.Provider
	if provider == "" {
		provider = defaultOpenRouterProvider
	}
	p := &openrouterProxy{
		anthropicProxy: NewAnthropicProxy(ca, db, nil),
		provider:       provider,
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
	server.OnRequest(anthropicEventLoggingCondition()).DoFunc(p.handleEventLogging)
	// Handle /v1/messages API requests
	server.OnRequest(openRouterV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(openRouterV1MessagesCondition()).DoFunc(p.handleResponse)
}

func (p *openrouterProxy) stickProvider(ctx *proxy.OnContext[proxy.ReqCtx]) {
	if p.provider == "" {
		logrus.Info("no provider configured for openrouter proxy, skipping provider stickiness")
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

func openRouterV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "openrouter.ai" &&
			strings.ToLower(req.URL.Path) == "/api/v1/messages"
	}
}
