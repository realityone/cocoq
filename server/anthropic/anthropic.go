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

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/sets"
)

var anthropicProxyDomains = sets.New(
	"api.anthropic.com",
	"downloads.claude.ai",
	"platform.claude.com",
	"raw.githubusercontent.com",
)

type anthropicProxy struct {
	ca tls.Certificate
	db *dbrt.Client

	onRequest  []proxy.Handler[proxy.ReqCtx]
	onResponse []proxy.Handler[proxy.RespCtx]
}

func NewAnthropicProxy(ca tls.Certificate, db *dbrt.Client) *anthropicProxy {
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
	return p
}

func (p *anthropicProxy) Domains() sets.Set[string] {
	return anthropicProxyDomains.Clone()
}

func (p *anthropicProxy) Install(server *goproxy.ProxyHttpServer) {
	logrus.Infof("Installing Anthropic proxy for domains: %+v", p.Domains().UnsortedList())
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
	server.OnRequest(anthropicV1MessagesCondition()).DoFunc(p.handleRequest)
	server.OnResponse(anthropicV1MessagesCondition()).DoFunc(p.handleResponse)
}

type UserData struct {
	AccountUUID string
	DeviceID    string
	Model       string
	SessionID   string
	Stream      bool
}

func getUserData(ctx *goproxy.ProxyCtx) *UserData {
	return ctx.UserData.(*UserData)
}

func (p *anthropicProxy) prelude(ctx *proxy.OnContext[proxy.ReqCtx]) {
	data := &UserData{}
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
	return proxy.HandleOnResponse(resp, ctx, p.onResponse...)
}

func (p *anthropicProxy) saveUsage(ctx context.Context, data *UserData, usage proxy.AnthropicUsage) {
	record, err := p.db.AnthropicUsage.Create().
		SetDeviceID(data.DeviceID).
		SetSessionID(data.SessionID).
		SetAccountUUID(data.AccountUUID).
		SetInputTokens(usage.InputTokens).
		SetCacheReadInputTokens(usage.CacheReadInputTokens).
		SetCacheCreationInputTokens(usage.CacheCreationInputTokens).
		SetOutputTokens(usage.OutputTokens).
		SetCacheCreationEphemeral5mInputTokens(usage.CacheCreation.Ephemeral5mInputTokens).
		SetCacheCreationEphemeral1hInputTokens(usage.CacheCreation.Ephemeral1hInputTokens).
		SetCacheHitRate(usage.CacheHitRate()).
		SetRaw(usage.RawString()).
		Save(ctx)
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
	}).Debug("saved anthropic usage")
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

func usageLoggingFields(usage proxy.AnthropicUsage) logrus.Fields {
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
