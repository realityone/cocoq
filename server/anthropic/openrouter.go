package anthropic

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"
	"github.com/realityone/cocoq/server/sse"

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

func NewOpenrouterProxy(ca tls.Certificate, db *dbrt.Client) *openrouterProxy {
	p := &openrouterProxy{
		anthropicProxy: NewAnthropicProxy(ca, db),
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

func (p *openrouterProxy) extactUsage(ctx *proxy.OnContext[proxy.RespCtx]) {
	data := getUserData(ctx.Opaque.ProxyCtx)
	ctx.Opaque.Metrics.Model = data.Model

	if data.Stream {
		p.extactUsageFromSSE(ctx)
		return
	}
	p.extactUsageFromBody(ctx)
}

func (p *openrouterProxy) extactUsageFromBody(ctx *proxy.OnContext[proxy.RespCtx]) {
	resp := ctx.Opaque.Response
	if resp.Body == nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read openrouter response body for usage")
		return
	}
	defer resp.Body.Close()
	proxy.ReplaceResponseBody(resp, body)
	ctx.Opaque.PostResponse = resp

	usage, ok := p.parseUsage(body)
	if !ok {
		logrus.Warn("failed to parse usage from openrouter response body")
		return
	}
	p.recordUsage(ctx, usage)
}

func (p *openrouterProxy) extactUsageFromSSE(ctx *proxy.OnContext[proxy.RespCtx]) {
	resp := ctx.Opaque.Response
	events := sse.Forward(resp)

	go func() {
		for event := range events {
			data, ok := event.Data.(string)
			if !ok || !gjson.Get(data, "usage").Exists() {
				continue
			}
			usage, ok := p.parseUsage([]byte(data))
			if !ok {
				continue
			}
			p.recordUsage(ctx, usage)
			logrus.WithField("model", ctx.Opaque.Metrics.Model).
				WithFields(p.usageFields(usage)).
				Info("extracted usage from openrouter SSE response")
		}
	}()
	ctx.Opaque.PostResponse = resp
}

func (p *openrouterProxy) recordUsage(ctx *proxy.OnContext[proxy.RespCtx], usage proxy.AnthropicUsage) {
	ctx.Opaque.Metrics.Usage = usage
	userData := getUserData(ctx.Opaque.ProxyCtx)
	reqCtx := ctx.Opaque.ProxyCtx.Req.Context()
	p.saveUsage(reqCtx, userData, usage)
}

func (p *openrouterProxy) usageFields(usage proxy.AnthropicUsage) logrus.Fields {
	fields := logrus.Fields{
		"input_tokens":                   usage.InputTokens,
		"output_tokens":                  usage.OutputTokens,
		"cache_read_input_tokens":        usage.CacheReadInputTokens,
		"cache_creation_input_tokens":    usage.CacheCreationInputTokens,
		"cache_creation_5m_input_tokens": usage.CacheCreation.Ephemeral5mInputTokens,
		"cache_creation_1h_input_tokens": usage.CacheCreation.Ephemeral1hInputTokens,
	}
	if raw := usage.GetRaw(); len(raw) > 0 {
		fields["usage"] = string(raw)
	}
	return fields
}

func (p *openrouterProxy) parseUsage(body []byte) (proxy.AnthropicUsage, bool) {
	usageData := gjson.GetBytes(body, "usage")
	if !usageData.Exists() {
		logrus.Warn("usage data not found in openrouter response body")
		return proxy.AnthropicUsage{}, false
	}
	inputTokens := usageData.Get("input_tokens").Int()
	outputTokens := usageData.Get("output_tokens").Int()
	if inputTokens == 0 && outputTokens == 0 {
		logrus.Warn("input and output tokens are both zero in response usage data, skipping usage recording")
		return proxy.AnthropicUsage{}, false
	}
	usage := proxy.AnthropicUsage{
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheReadInputTokens:     usageData.Get("cache_read_input_tokens").Int(),
		CacheCreationInputTokens: usageData.Get("cache_creation_input_tokens").Int(),
	}
	// Assume cache creation input tokens are 1h TTL tokens because OpenRouter
	// does not provide a reliable cache creation breakdown.
	usage.CacheCreation.Ephemeral1hInputTokens = usage.CacheCreationInputTokens
	usage.SetRaw(json.RawMessage(usageData.Raw))
	return usage, true
}

func openRouterV1MessagesCondition() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.ToLower(req.URL.Hostname()) == "openrouter.ai" &&
			strings.ToLower(req.URL.Path) == "/api/v1/messages"
	}
}
