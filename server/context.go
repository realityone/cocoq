package server

import (
	"math"
	"net/http"

	"github.com/elazarl/goproxy"
)

// abortIndex represents a typical value used in abort functions.
const abortIndex int8 = math.MaxInt8 >> 1

type RequestHandlersChain []func(*OnRequestContext)
type OnRequestContext struct {
	ProxyCtx *goproxy.ProxyCtx
	Request  *http.Request

	index    int8
	handlers RequestHandlersChain
}

func (c *OnRequestContext) Next() {
	c.index++
	for c.index < safeInt8(len(c.handlers)) {
		if c.handlers[c.index] != nil {
			c.handlers[c.index](c)
		}
		c.index++
	}
}

// IsAborted returns true if the current context was aborted.
func (c *OnRequestContext) IsAborted() bool {
	return c.index >= abortIndex
}

type ResponseHandlersChain []func(*OnResponseContext)
type OnResponseContext struct {
	ProxyCtx *goproxy.ProxyCtx
	Response *http.Response

	index    int8
	handlers ResponseHandlersChain
}

func (c *OnResponseContext) Next() {
	c.index++
	for c.index < safeInt8(len(c.handlers)) {
		if c.handlers[c.index] != nil {
			c.handlers[c.index](c)
		}
		c.index++
	}
}

// IsAborted returns true if the current context was aborted.
func (c *OnResponseContext) IsAborted() bool {
	return c.index >= abortIndex
}

// safeInt8 converts int to int8 safely, capping at math.MaxInt8
func safeInt8(n int) int8 {
	if n > math.MaxInt8 {
		return math.MaxInt8
	}
	return int8(n)
}
