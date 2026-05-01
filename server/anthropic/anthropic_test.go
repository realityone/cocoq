package anthropic

import (
	"crypto/tls"
	"testing"

	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
)

func TestPreludeReadsStreamAndRestoresBody(t *testing.T) {
	body := `{
		"stream": true
	}`
	req := newAnthropicMessagesRequest(t, body)
	proxyCtx := &goproxy.ProxyCtx{}
	ctx := &proxy.OnContext[proxy.ReqCtx]{
		Opaque: proxy.ReqCtx{
			ProxyCtx: proxyCtx,
			Request:  req,
		},
	}

	(&anthropicProxy{}).prelude(ctx)

	data, ok := proxyCtx.UserData.(*UserData)
	if !ok {
		t.Fatalf("ctx.UserData = %T, want *UserData", proxyCtx.UserData)
	}
	if !data.Stream {
		t.Fatal("Stream = false, want true")
	}

	if restored := string(readRequestBody(t, req)); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}
}

func TestHandleRequestStoresRequestMetadata(t *testing.T) {
	body := `{"stream":true}`
	req := newAnthropicMessagesRequest(t, body)
	ctx := &goproxy.ProxyCtx{}

	gotReq, gotResp := NewAnthropicProxy(tls.Certificate{}).handleRequest(req, ctx)
	if gotResp != nil {
		t.Fatalf("handleRequest response = %v, want nil", gotResp)
	}
	if gotReq != req {
		t.Fatal("handleRequest returned a different request")
	}

	data, ok := ctx.UserData.(*UserData)
	if !ok {
		t.Fatalf("ctx.UserData = %T, want *UserData", ctx.UserData)
	}
	if !data.Stream {
		t.Fatal("ctx.UserData.Stream = false, want true")
	}
	if restored := string(readRequestBody(t, gotReq)); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}
}
