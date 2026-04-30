package proxy

import (
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

type RespCtx struct {
	ProxyCtx *goproxy.ProxyCtx
	Response *http.Response

	PostResponse *http.Response
}

type OnContext[T any] struct {
	Tuple T

	index    int8
	handlers HandlersChain[T]
}

func newContext[T any](val T, handlers ...HandlerFunc[T]) *OnContext[T] {
	return &OnContext[T]{
		Tuple:    val,
		index:    -1,
		handlers: handlers,
	}
}

func (c *OnContext[T]) Next() {
	c.index++
	for c.index < safeInt8(len(c.handlers)) {
		if c.handlers[c.index] != nil {
			c.handlers[c.index](c)
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

func HandleOnRequest(req *http.Request, proxyCtx *goproxy.ProxyCtx, handlers ...HandlerFunc[ReqCtx]) (*http.Request, *http.Response) {
	ctx := newContext(ReqCtx{
		ProxyCtx: proxyCtx,
		Request:  req,
	}, handlers...)
	ctx.Next()

	postRequest := ctx.Tuple.PostRequest
	if postRequest == nil {
		postRequest = ctx.Tuple.Request
	}
	return postRequest, ctx.Tuple.PostResponse
}

func HandleOnResponse(resp *http.Response, proxyCtx *goproxy.ProxyCtx, handlers ...HandlerFunc[RespCtx]) *http.Response {
	ctx := newContext(RespCtx{
		ProxyCtx: proxyCtx,
		Response: resp,
	}, handlers...)
	ctx.Next()

	if ctx.Tuple.PostResponse != nil {
		return ctx.Tuple.PostResponse
	}
	return ctx.Tuple.Response
}

type HandlerFunc[T any] func(*OnContext[T])
type HandlersChain[T any] []HandlerFunc[T]

type RequestHandlersChain = HandlersChain[ReqCtx]
type ResponseHandlersChain = HandlersChain[RespCtx]

type OnRequestContext = OnContext[ReqCtx]
type OnResponseContext = OnContext[RespCtx]

func DstHostInSet(hostSet sets.Set[string]) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return hostSet.Has(strings.ToLower(req.URL.Hostname()))
	}
}
