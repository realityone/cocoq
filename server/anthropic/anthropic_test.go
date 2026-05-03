package anthropic

import (
	"crypto/tls"
	"strings"
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

	gotReq, gotResp := NewAnthropicProxy(tls.Certificate{}, nil).handleRequest(req, ctx)
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

func TestParseUsageUsesNestedCacheCreationBreakdown(t *testing.T) {
	body := []byte(`{
		"usage": {
			"input_tokens": 4,
			"output_tokens": 8,
			"cache_creation_input_tokens": 44,
			"cache_read_input_tokens": 11,
			"cache_creation": {
				"ephemeral_5m_input_tokens": 12,
				"ephemeral_1h_input_tokens": 32
			}
		}
	}`)

	var usage proxy.AnthropicUsage
	if ok := (&anthropicProxy{}).parseUsageTo(body, &usage); !ok {
		t.Fatal("parseUsageTo returned false, want true")
	}

	if usage.InputTokens != 4 {
		t.Fatalf("InputTokens = %d, want 4", usage.InputTokens)
	}
	if usage.OutputTokens != 8 {
		t.Fatalf("OutputTokens = %d, want 8", usage.OutputTokens)
	}
	if usage.CacheReadInputTokens != 11 {
		t.Fatalf("CacheReadInputTokens = %d, want 11", usage.CacheReadInputTokens)
	}
	if usage.CacheCreationInputTokens != 44 {
		t.Fatalf("CacheCreationInputTokens = %d, want 44", usage.CacheCreationInputTokens)
	}
	if usage.CacheCreation.Ephemeral5mInputTokens != 12 {
		t.Fatalf("CacheCreation.Ephemeral5mInputTokens = %d, want 12", usage.CacheCreation.Ephemeral5mInputTokens)
	}
	if usage.CacheCreation.Ephemeral1hInputTokens != 32 {
		t.Fatalf("CacheCreation.Ephemeral1hInputTokens = %d, want 32", usage.CacheCreation.Ephemeral1hInputTokens)
	}
	if len(usage.GetRaw()) == 0 {
		t.Fatal("raw usage JSON was not recorded")
	}
}

func TestExtractUsageFromStreamResponseCombinesMessageStartUsage(t *testing.T) {
	client := newOpenrouterTestDB(t)
	body := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"usage":{"input_tokens":19,"output_tokens":99,"cache_creation_input_tokens":44,"cache_read_input_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":12,"ephemeral_1h_input_tokens":32}}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","usage":{"input_tokens":23,"output_tokens":7}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: &goproxy.ProxyCtx{
				Req: newAnthropicMessagesRequest(t, "{}"),
				UserData: &UserData{
					DeviceID:    "device-4",
					SessionID:   "session-4",
					AccountUUID: "account-4",
					Model:       "claude-sonnet-4-6",
					Stream:      true,
				},
			},
			Response: resp,
		},
	}

	(&anthropicProxy{db: client}).extactUsage(ctx)

	if ctx.Opaque.Metrics.Model != "claude-sonnet-4-6" {
		t.Fatalf("Metrics.Model = %q, want claude-sonnet-4-6", ctx.Opaque.Metrics.Model)
	}
	if ctx.Opaque.PostResponse != resp {
		t.Fatal("PostResponse was not set to the forwarded response")
	}
	if restored := readResponseBody(t, resp); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}

	usage := waitForUsage(t, ctx)
	if usage.InputTokens != 23 {
		t.Fatalf("InputTokens = %d, want 23", usage.InputTokens)
	}
	if usage.OutputTokens != 7 {
		t.Fatalf("OutputTokens = %d, want 7", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 44 {
		t.Fatalf("CacheCreationInputTokens = %d, want 44", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 5 {
		t.Fatalf("CacheReadInputTokens = %d, want 5", usage.CacheReadInputTokens)
	}
	if usage.CacheCreation.Ephemeral5mInputTokens != 12 {
		t.Fatalf("CacheCreation.Ephemeral5mInputTokens = %d, want 12", usage.CacheCreation.Ephemeral5mInputTokens)
	}
	if usage.CacheCreation.Ephemeral1hInputTokens != 32 {
		t.Fatalf("CacheCreation.Ephemeral1hInputTokens = %d, want 32", usage.CacheCreation.Ephemeral1hInputTokens)
	}

	record := waitForUsageRecord(t, client)
	if record.DeviceID != "device-4" {
		t.Fatalf("DeviceID = %q, want device-4", record.DeviceID)
	}
	if record.CacheCreationEphemeral5mInputTokens != 12 {
		t.Fatalf("CacheCreationEphemeral5mInputTokens = %d, want 12", record.CacheCreationEphemeral5mInputTokens)
	}
	if record.CacheCreationEphemeral1hInputTokens != 32 {
		t.Fatalf("CacheCreationEphemeral1hInputTokens = %d, want 32", record.CacheCreationEphemeral1hInputTokens)
	}
}
