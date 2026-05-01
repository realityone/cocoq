package proxy

import (
	"bytes"
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

type AnthropicUsage interface {
	InputTokens() int64
	OutputTokens() int64
	CacheReadInputTokens() int64
	CacheCreationInputTokens() int64
	CacheCreationEphemeral5mInputTokens() int64
	CacheCreationEphemeral1hInputTokens() int64
}

type RespCtx struct {
	ProxyCtx *goproxy.ProxyCtx
	Response *http.Response

	PostResponse *http.Response
	Metrics      struct {
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
