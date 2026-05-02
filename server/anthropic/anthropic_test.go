package anthropic

import (
	"crypto/tls"
	"testing"

	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
)

func TestPreludeReadsStreamAndRestoresBody(t *testing.T) {
	body := `{
		"stream": true,
		"model": "claude-sonnet-4-6",
		"metadata": {
			"user_id": "{\"device_id\":\"46f2488caedc085068ef12d939484e666710900cfb3b83274d6b5612143ce707\",\"account_uuid\":\"account-1\",\"session_id\":\"3bb1d3fd-f5d5-404e-90fc-042d486328f9\"}"
		}
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
	if data.Model != "claude-sonnet-4-6" {
		t.Fatalf("Model = %q, want claude-sonnet-4-6", data.Model)
	}
	if data.DeviceID != "46f2488caedc085068ef12d939484e666710900cfb3b83274d6b5612143ce707" {
		t.Fatalf("DeviceID = %q, want parsed device id", data.DeviceID)
	}
	if data.SessionID != "3bb1d3fd-f5d5-404e-90fc-042d486328f9" {
		t.Fatalf("SessionID = %q, want parsed session id", data.SessionID)
	}
	if data.AccountUUID != "account-1" {
		t.Fatalf("AccountUUID = %q, want account-1", data.AccountUUID)
	}

	if restored := string(readRequestBody(t, req)); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}
}

func TestPreludeIgnoresPlainUserIDMetadata(t *testing.T) {
	body := `{
		"metadata": {
			"user_id": "device-1",
			"account_uuid": "account-1",
			"session_id": "session-1"
		}
	}`
	req := newAnthropicMessagesRequest(t, body)
	proxyCtx := &goproxy.ProxyCtx{Session: 42}
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
	if data.DeviceID != "" {
		t.Fatalf("DeviceID = %q, want empty", data.DeviceID)
	}
	if data.SessionID != "" {
		t.Fatalf("SessionID = %q, want empty", data.SessionID)
	}
	if data.AccountUUID != "" {
		t.Fatalf("AccountUUID = %q, want empty", data.AccountUUID)
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
