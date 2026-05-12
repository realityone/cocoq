package anthropic

import (
	"context"
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

var anthropicProxyDomains = sets.New(
	"api.anthropic.com",
	"downloads.claude.ai",
	"platform.claude.com",
	"raw.githubusercontent.com",
	"mcp-proxy.anthropic.com",
)

type anthropicProxy struct {
	ca tls.Certificate
	db *dbrt.Client

	onRequest  []proxy.Handler[proxy.ReqCtx]
	onResponse []proxy.Handler[proxy.RespCtx]
}

func NewAnthropicProxy(ca tls.Certificate, db *dbrt.Client, options json.RawMessage) *anthropicProxy {
	p := &anthropicProxy{
		ca: ca,
		db: db,
	}
	p.onRequest = append(
		p.onRequest,
		proxy.HandlerFunc[proxy.ReqCtx](p.prelude),
		caching1h{},
		currentDateUTC{},
	)
	p.onResponse = append(
		p.onResponse,
		proxy.HandlerFunc[proxy.RespCtx](p.extactUsage),
	)
	return p
}

func (p *anthropicProxy) Domains() sets.Set[string] {
	return anthropicProxyDomains.Clone()
}

func (p *anthropicProxy) Install(server *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Anthropic proxy for domains: %+v", p.Domains().UnsortedList())
	server.OnRequest(proxy.DstHostInSet(p.Domains())).HandleConnect(proxy.NewMitmConnectAction(p.ca))

	// Reject event logging requests
	server.OnRequest(anthropicEventLoggingCondition()).DoFunc(p.handleEventLogging)
	// Handle /v1/messages API requests
	server.OnRequest(anthropicV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(anthropicV1MessagesCondition()).DoFunc(p.handleResponse)
}

type UserData struct {
	SessionContext context.Context
	sessionCancel  context.CancelFunc

	AccountUUID string
	DeviceID    string
	Model       string
	SessionID   string
	Stream      bool
}

func getUserData(ctx *goproxy.ProxyCtx) *UserData {
	return ctx.UserData.(*UserData)
}

func (data *UserData) context() context.Context {
	if data.SessionContext != nil {
		return data.SessionContext
	}
	return context.Background()
}

func (data *UserData) cancelSession() {
	if data.sessionCancel != nil {
		data.sessionCancel()
	}
}

func (p *anthropicProxy) handleEventLogging(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	response := `{"accepted_count": 0, "rejected_count": 0}`
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read anthropic request body for event logging")
		return req, goproxy.NewResponse(req, "application/json", http.StatusOK, response)
	}
	defer req.Body.Close()

	events := gjson.GetBytes(reqBody, "events")
	if !events.IsArray() {
		logrus.Warn("anthropic event logging request body does not contain events array")
		return req, goproxy.NewResponse(req, "application/json", http.StatusOK, response)
	}
	logrus.WithFields(logrus.Fields{
		"session":     ctx.Session,
		"service":     "anthropic",
		"host":        req.Host,
		"url":         req.URL.String(),
		"method":      req.Method,
		"event_count": len(events.Array()),
	}).Info("sent synthetic Anthropic event logging response")
	response1, err := sjson.Set(response, "accepted_count", len(events.Array()))
	if err != nil {
		logrus.WithError(err).Warn("failed to set accepted_count in anthropic event logging response")
		return req, goproxy.NewResponse(req, "application/json", http.StatusOK, response)
	}
	return req, goproxy.NewResponse(req, "application/json", http.StatusOK, response1)
}

func (p *anthropicProxy) prelude(ctx *proxy.OnContext[proxy.ReqCtx]) {
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	data := &UserData{
		SessionContext: sessionCtx,
		sessionCancel:  sessionCancel,
	}
	ctx.Opaque.ProxyCtx.UserData = data
	req := ctx.Opaque.Request
	if req.Body == nil {
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read anthropic request body for metadata")
		return
	}
	defer req.Body.Close()
	proxy.ReplaceRequestBody(req, body)

	data.Stream = gjson.GetBytes(body, "stream").Bool()
	data.Model = gjson.GetBytes(body, "model").String()
	metadataUserID := gjson.GetBytes(body, "metadata.user_id").String()
	metadata, ok := parseRequestMetadata(metadataUserID)
	if ok {
		data.DeviceID = metadata.DeviceID
		data.AccountUUID = metadata.AccountUUID
		data.SessionID = metadata.SessionID
	}
}

type requestMetadata struct {
	AccountUUID string `json:"account_uuid"`
	DeviceID    string `json:"device_id"`
	SessionID   string `json:"session_id"`
}

func parseRequestMetadata(value string) (requestMetadata, bool) {
	var metadata requestMetadata
	if value = strings.TrimSpace(value); value == "" {
		return requestMetadata{}, false
	}
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return requestMetadata{}, false
	}
	return metadata, true
}

func (p *anthropicProxy) handleRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return proxy.HandleOnRequest(req, ctx, p.onRequest...)
}

func (p *anthropicProxy) handleResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	userData := getUserData(ctx)
	if !userData.Stream || len(p.onResponse) == 0 {
		defer userData.cancelSession()
	}
	return proxy.HandleOnResponse(resp, ctx, p.onResponse...)
}

func (p *anthropicProxy) saveUsage(data *UserData, usage proxy.AnthropicUsage) {
	record, err := p.db.AnthropicUsage.Create().
		SetDeviceID(data.DeviceID).
		SetSessionID(data.SessionID).
		SetAccountUUID(data.AccountUUID).
		SetModel(data.Model).
		SetInputTokens(usage.InputTokens).
		SetCacheReadInputTokens(usage.CacheReadInputTokens).
		SetCacheCreationInputTokens(usage.CacheCreationInputTokens).
		SetOutputTokens(usage.OutputTokens).
		SetCacheCreationEphemeral5mInputTokens(usage.CacheCreation.Ephemeral5mInputTokens).
		SetCacheCreationEphemeral1hInputTokens(usage.CacheCreation.Ephemeral1hInputTokens).
		SetCacheHitRate(usage.CacheHitRate()).
		SetRaw(usage.RawString()).
		Save(data.context())
	if err != nil {
		logrus.WithError(err).
			WithFields(usageLoggingFields(usage)).
			Warn("failed to save anthropic usage")
		return
	}

	logrus.WithFields(logrus.Fields{
		"id":           record.ID,
		"device_id":    record.DeviceID,
		"session_id":   record.SessionID,
		"account_uuid": record.AccountUUID,
		"model":        record.Model,
	}).Debug("saved anthropic usage")
}

func (p *anthropicProxy) extactUsage(ctx *proxy.OnContext[proxy.RespCtx]) {
	data := getUserData(ctx.Opaque.ProxyCtx)
	ctx.Opaque.Metrics.Model = data.Model

	if data.Stream {
		p.extactUsageFromSSE(ctx)
		return
	}
	p.extactUsageFromBody(ctx)
}

func (p *anthropicProxy) extactUsageFromBody(ctx *proxy.OnContext[proxy.RespCtx]) {
	resp := ctx.Opaque.Response
	if resp.Body == nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read anthropic api service response body for usage")
		return
	}
	defer resp.Body.Close()
	proxy.ReplaceResponseBody(resp, body)
	ctx.Opaque.PostResponse = resp

	usage := proxy.AnthropicUsage{}
	if done := p.parseUsageTo(body, &usage); !done {
		logrus.Warn("failed to parse usage from anthropic api service response body")
		return
	}
	p.recordUsage(ctx, usage)
}

func (p *anthropicProxy) extactUsageFromSSE(ctx *proxy.OnContext[proxy.RespCtx]) {
	resp := ctx.Opaque.Response
	events := sse.Forward(resp)
	userData := getUserData(ctx.Opaque.ProxyCtx)

	usage := proxy.AnthropicUsage{}
	go func() {
		defer userData.cancelSession()
		for event := range events {
			data, ok := event.Data.(string)
			if !ok || !sseUsageData([]byte(data)).Exists() {
				continue
			}
			if done := p.parseSSEUsageTo([]byte(data), &usage); !done {
				continue
			}
			p.recordUsage(ctx, usage)
			logrus.WithField("model", ctx.Opaque.Metrics.Model).
				WithFields(usageLoggingFields(usage)).
				Info("extracted usage from anthropic SSE response")
		}
	}()
	ctx.Opaque.PostResponse = resp
}

func (p *anthropicProxy) recordUsage(ctx *proxy.OnContext[proxy.RespCtx], usage proxy.AnthropicUsage) {
	userData := getUserData(ctx.Opaque.ProxyCtx)
	p.saveUsage(userData, usage)
	ctx.Opaque.Metrics.Usage = usage
}

func (p *anthropicProxy) parseUsageTo(body []byte, usage *proxy.AnthropicUsage) bool {
	usageData := gjson.GetBytes(body, "usage")
	if !usageData.Exists() {
		logrus.Warn("usage data not found in anthropic api service response body")
		return false
	}
	inputTokens := usageData.Get("input_tokens").Int()
	outputTokens := usageData.Get("output_tokens").Int()
	if inputTokens == 0 && outputTokens == 0 {
		logrus.Warn("input and output tokens are both zero in response usage data, skipping usage recording")
		return false
	}
	usage.InputTokens = inputTokens
	usage.OutputTokens = outputTokens
	usage.CacheReadInputTokens = usageData.Get("cache_read_input_tokens").Int()
	usage.CacheCreationInputTokens = usageData.Get("cache_creation_input_tokens").Int()
	if usage.CacheCreationInputTokens > 0 {
		updateCacheCreationBreakdown(usageData, usage)
	}
	if usage.CacheCreation.Ephemeral5mInputTokens == 0 && usage.CacheCreation.Ephemeral1hInputTokens == 0 {
		usage.CacheCreation.Ephemeral1hInputTokens = usage.CacheCreationInputTokens
	}
	usage.SetRaw(json.RawMessage(usageData.Raw))
	return true
}

func (p *anthropicProxy) parseSSEUsageTo(data []byte, usage *proxy.AnthropicUsage) bool {
	usageData := sseUsageData(data)
	if !usageData.Exists() {
		logrus.Warn("usage data not found in anthropic SSE response event")
		return false
	}
	// The message_start event contains cache creation tokens but not input/output tokens,
	// while the message_end event contains input/output tokens but may not contain cache creation tokens.
	// We need to parse both events to get the complete usage data.
	if gjson.GetBytes(data, "type").String() == "message_start" {
		usage.CacheCreationInputTokens = usageData.Get("cache_creation_input_tokens").Int()
		usage.CacheReadInputTokens = usageData.Get("cache_read_input_tokens").Int()
		if usage.CacheCreationInputTokens > 0 {
			updateCacheCreationBreakdown(usageData, usage)
		}
		return false
	}
	inputTokens := usageData.Get("input_tokens").Int()
	outputTokens := usageData.Get("output_tokens").Int()
	if inputTokens == 0 && outputTokens == 0 {
		logrus.Warn("input and output tokens are both zero in anthropic SSE response event usage data, skipping usage recording")
		return false
	}
	usage.InputTokens = inputTokens
	usage.OutputTokens = outputTokens
	if cacheCreationInputTokens := usageData.Get("cache_creation_input_tokens").Int(); cacheCreationInputTokens > 0 {
		usage.CacheCreationInputTokens = cacheCreationInputTokens
		updateCacheCreationBreakdown(usageData, usage)
	}
	if usage.CacheCreationInputTokens > 0 &&
		usage.CacheCreation.Ephemeral5mInputTokens == 0 &&
		usage.CacheCreation.Ephemeral1hInputTokens == 0 {
		// Assume cache creation input tokens are 1h TTL tokens because some Anthropic API services (e.g. openrouter)
		// does not provide a reliable cache creation breakdown.
		usage.CacheCreation.Ephemeral1hInputTokens = usage.CacheCreationInputTokens
	}
	if cacheReadInputTokens := usageData.Get("cache_read_input_tokens").Int(); cacheReadInputTokens > 0 {
		usage.CacheReadInputTokens = cacheReadInputTokens
	}
	usage.SetRaw(json.RawMessage(usageData.Raw))
	return true
}

func sseUsageData(data []byte) gjson.Result {
	if usageData := gjson.GetBytes(data, "usage"); usageData.Exists() {
		return usageData
	}
	if gjson.GetBytes(data, "type").String() == "message_start" {
		return gjson.GetBytes(data, "message.usage")
	}
	return gjson.Result{}
}

func updateCacheCreationBreakdown(usageData gjson.Result, usage *proxy.AnthropicUsage) {
	cacheCreation := usageData.Get("cache_creation")
	if !cacheCreation.Exists() {
		return
	}
	if ephemeral5m := cacheCreation.Get("ephemeral_5m_input_tokens"); ephemeral5m.Exists() {
		usage.CacheCreation.Ephemeral5mInputTokens = ephemeral5m.Int()
	}
	if ephemeral1h := cacheCreation.Get("ephemeral_1h_input_tokens"); ephemeral1h.Exists() {
		usage.CacheCreation.Ephemeral1hInputTokens = ephemeral1h.Int()
	}
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
			strings.ToLower(req.URL.Path) == "/v1/messages" &&
			req.Method == http.MethodPost
	}
}

func usageLoggingFields(usage proxy.AnthropicUsage) logrus.Fields {
	fields := logrus.Fields{
		"input_tokens":                   usage.InputTokens,
		"output_tokens":                  usage.OutputTokens,
		"cache_read_input_tokens":        usage.CacheReadInputTokens,
		"cache_creation_input_tokens":    usage.CacheCreationInputTokens,
		"cache_creation_5m_input_tokens": usage.CacheCreation.Ephemeral5mInputTokens,
		"cache_creation_1h_input_tokens": usage.CacheCreation.Ephemeral1hInputTokens,
		"cache_hit_rate":                 usage.CacheHitRate(),
	}
	if raw := usage.GetRaw(); len(raw) > 0 {
		fields["usage"] = string(raw)
	}
	return fields
}
