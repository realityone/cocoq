package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"k8s.io/apimachinery/pkg/util/sets"
)

// abortIndex represents a typical value used in abort functions.
const abortIndex int8 = math.MaxInt8 >> 1

type ReqCtx struct {
	ProxyCtx *goproxy.ProxyCtx
	Request  *http.Request

	PostRequest  *http.Request
	PostResponse *http.Response
}

type AnthropicUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheCreation            struct {
		Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
		Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
	raw json.RawMessage `json:"-"`
}

func (u *AnthropicUsage) UnmarshalJSON(data []byte) error {
	type Alias AnthropicUsage
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	u.raw = data
	return nil
}

func (u *AnthropicUsage) MarshalJSON() ([]byte, error) {
	if u.raw != nil {
		return u.raw, nil
	}
	type Alias AnthropicUsage
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	})
}

func (u *AnthropicUsage) GetRaw() json.RawMessage {
	return u.raw
}

func (u *AnthropicUsage) SetRaw(raw json.RawMessage) {
	u.raw = raw
}

func (u AnthropicUsage) CacheHitRate() float64 {
	inputTokens := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	if inputTokens <= 0 {
		return 0
	}
	return float64(u.CacheReadInputTokens) / float64(inputTokens)
}

func (u AnthropicUsage) RawString() string {
	if raw := u.GetRaw(); len(raw) > 0 {
		return string(raw)
	}
	body, err := json.Marshal(u)
	if err != nil {
		return "{}"
	}
	return string(body)
}

type RespCtx struct {
	ProxyCtx *goproxy.ProxyCtx
	Response *http.Response

	PostResponse *http.Response
	Metrics      struct {
		Model string
		Usage AnthropicUsage
	}
}

type OnContext[T any] struct {
	Opaque T

	index    int8
	handlers []Handler[T]
}

func newContext[T any](val T, handlers ...Handler[T]) *OnContext[T] {
	return &OnContext[T]{
		Opaque:   val,
		index:    -1,
		handlers: handlers,
	}
}

func (c *OnContext[T]) Next() {
	c.index++
	for c.index < safeInt8(len(c.handlers)) {
		if c.handlers[c.index] != nil {
			c.handlers[c.index].Handle(c)
		}
		c.index++
	}
}

// IsAborted returns true if the current context was aborted.
func (c *OnContext[T]) IsAborted() bool {
	return c.index >= abortIndex
}

// safeInt8 converts int to int8 safely, capping at math.MaxInt8
func safeInt8(n int) int8 {
	if n > math.MaxInt8 {
		return math.MaxInt8
	}
	return int8(n)
}

func HandleOnRequest(req *http.Request, proxyCtx *goproxy.ProxyCtx, handlers ...Handler[ReqCtx]) (*http.Request, *http.Response) {
	ctx := newContext(ReqCtx{
		ProxyCtx: proxyCtx,
		Request:  req,
	}, handlers...)
	ctx.Next()

	postRequest := ctx.Opaque.PostRequest
	if postRequest == nil {
		postRequest = ctx.Opaque.Request
	}
	return postRequest, ctx.Opaque.PostResponse
}

func HandleOnResponse(resp *http.Response, proxyCtx *goproxy.ProxyCtx, handlers ...Handler[RespCtx]) *http.Response {
	ctx := newContext(RespCtx{
		ProxyCtx: proxyCtx,
		Response: resp,
	}, handlers...)
	ctx.Next()

	if ctx.Opaque.PostResponse != nil {
		return ctx.Opaque.PostResponse
	}
	return ctx.Opaque.Response
}

type Handler[T any] interface {
	Handle(*OnContext[T])
}
type HandlerFunc[T any] func(*OnContext[T])

func (f HandlerFunc[T]) Handle(ctx *OnContext[T]) {
	f(ctx)
}

type HandlersChain[T any] []HandlerFunc[T]

type OnRequestContext = OnContext[ReqCtx]
type OnResponseContext = OnContext[RespCtx]

func DstHostInSet(hostSet sets.Set[string]) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return hostSet.Has(strings.ToLower(req.URL.Hostname()))
	}
}

func ReplaceRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}

func ReplaceResponseBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
}
