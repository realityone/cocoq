package server

import (
	"math"
	"net/http"

	"github.com/elazarl/goproxy"
)

// abortIndex represents a typical value used in abort functions.
const abortIndex int8 = math.MaxInt8 >> 1

type OnRequestTuple struct {
	ProxyCtx *goproxy.ProxyCtx
	Request  *http.Request

	PostRequest  *http.Request
	PostResponse *http.Response
}

type OnResponseTuple struct {
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

func handleOnRequest(req *http.Request, proxyCtx *goproxy.ProxyCtx, handlers ...HandlerFunc[OnRequestTuple]) (*http.Request, *http.Response) {
	ctx := newContext(OnRequestTuple{
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

func handleOnResponse(resp *http.Response, proxyCtx *goproxy.ProxyCtx, handlers ...HandlerFunc[OnResponseTuple]) *http.Response {
	ctx := newContext(OnResponseTuple{
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

type RequestHandlersChain = HandlersChain[OnRequestTuple]
type ResponseHandlersChain = HandlersChain[OnResponseTuple]

type OnRequestContext = OnContext[OnRequestTuple]
type OnResponseContext = OnContext[OnResponseTuple]
