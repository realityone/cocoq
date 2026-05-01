package anthropic

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/tidwall/gjson"
)

func TestOpenrouterStickProviderAddsProviderBody(t *testing.T) {
	body := `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`
	req := newAnthropicMessagesRequest(t, body)
	ctx := &proxy.OnContext[proxy.ReqCtx]{
		Opaque: proxy.ReqCtx{Request: req},
	}
	p := &openrouterProxy{
		provider: defaultOpenRouterProvider,
	}

	p.stickProvider(ctx)

	if ctx.Opaque.PostRequest != req {
		t.Fatal("stickProvider did not set PostRequest to the rewritten request")
	}

	got := readRequestBody(t, ctx.Opaque.PostRequest)
	assertJSONEqual(t, got, "provider.only.0", defaultOpenRouterProvider)
	if value := gjson.GetBytes(got, "provider.allow_fallbacks"); !value.Exists() || value.Bool() {
		t.Fatalf("provider.allow_fallbacks = %v, want false", value.Value())
	}
	assertJSONEqual(t, got, "model", "claude-sonnet-4-6")
}

func TestOpenrouterStickProviderOverwritesProviderBody(t *testing.T) {
	body := `{"provider":{"only":["openai"],"allow_fallbacks":true},"model":"claude-sonnet-4-6"}`
	got, changed, err := (&openrouterProxy{provider: defaultOpenRouterProvider}).stickProviderInBody([]byte(body))
	if err != nil {
		t.Fatalf("stickProviderInBody returned error: %v", err)
	}
	if !changed {
		t.Fatal("stickProviderInBody changed = false, want true")
	}

	assertJSONEqual(t, got, "provider.only.0", defaultOpenRouterProvider)
	if value := gjson.GetBytes(got, "provider.allow_fallbacks"); !value.Exists() || value.Bool() {
		t.Fatalf("provider.allow_fallbacks = %v, want false", value.Value())
	}
}

func TestOpenrouterExtractUsageFromNonStreamResponse(t *testing.T) {
	body := `{
		"id": "gen-1777663336-lfpI0TKtuVzXxksXmAIg",
		"usage": {
			"input_tokens": 3,
			"output_tokens": 117,
			"cache_creation_input_tokens": 24804,
			"cache_read_input_tokens": 0,
			"cache_creation": null
		},
		"provider": "Anthropic"
	}`
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: &goproxy.ProxyCtx{UserData: &UserData{}},
			Response: resp,
		},
	}

	(&openrouterProxy{}).extactUsage(ctx)

	usage := ctx.Opaque.Metrics.Usage
	if usage.InputTokens != 3 {
		t.Fatalf("InputTokens = %d, want 3", usage.InputTokens)
	}
	if usage.OutputTokens != 117 {
		t.Fatalf("OutputTokens = %d, want 117", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 24804 {
		t.Fatalf("CacheCreationInputTokens = %d, want 24804", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 0 {
		t.Fatalf("CacheReadInputTokens = %d, want 0", usage.CacheReadInputTokens)
	}
	if usage.CacheCreation.Ephemeral5mInputTokens != 0 {
		t.Fatalf("CacheCreation.Ephemeral5mInputTokens = %d, want 0", usage.CacheCreation.Ephemeral5mInputTokens)
	}
	if usage.CacheCreation.Ephemeral1hInputTokens != 24804 {
		t.Fatalf("CacheCreation.Ephemeral1hInputTokens = %d, want 24804", usage.CacheCreation.Ephemeral1hInputTokens)
	}
	if len(usage.GetRaw()) == 0 {
		t.Fatal("raw usage JSON was not recorded")
	}
	if ctx.Opaque.PostResponse != resp {
		t.Fatal("PostResponse was not set to the restored response")
	}
	if restored := readResponseBody(t, resp); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}
}

func TestOpenrouterExtractUsageTreatsCacheCreationAs1h(t *testing.T) {
	body := `{
		"usage": {
			"input_tokens": 4,
			"output_tokens": 8,
			"cache_creation_input_tokens": 12,
			"cache_read_input_tokens": 16,
			"cache_creation": {
				"ephemeral_5m_input_tokens": 20,
				"ephemeral_1h_input_tokens": 24
			}
		}
	}`
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: &goproxy.ProxyCtx{UserData: &UserData{}},
			Response: resp,
		},
	}

	(&openrouterProxy{}).extactUsage(ctx)

	usage := ctx.Opaque.Metrics.Usage
	if usage.InputTokens != 4 {
		t.Fatalf("InputTokens = %d, want 4", usage.InputTokens)
	}
	if usage.OutputTokens != 8 {
		t.Fatalf("OutputTokens = %d, want 8", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 12 {
		t.Fatalf("CacheCreationInputTokens = %d, want 12", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 16 {
		t.Fatalf("CacheReadInputTokens = %d, want 16", usage.CacheReadInputTokens)
	}
	if usage.CacheCreation.Ephemeral5mInputTokens != 0 {
		t.Fatalf("CacheCreation.Ephemeral5mInputTokens = %d, want 0", usage.CacheCreation.Ephemeral5mInputTokens)
	}
	if usage.CacheCreation.Ephemeral1hInputTokens != 12 {
		t.Fatalf("CacheCreation.Ephemeral1hInputTokens = %d, want 12", usage.CacheCreation.Ephemeral1hInputTokens)
	}
}

func TestOpenrouterExtractUsageSkipsStreamResponse(t *testing.T) {
	body := `{"usage":{"input_tokens":3}}`
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: &goproxy.ProxyCtx{UserData: &UserData{Stream: true}},
			Response: resp,
		},
	}

	(&openrouterProxy{}).extactUsage(ctx)

	if len(ctx.Opaque.Metrics.Usage.GetRaw()) != 0 {
		t.Fatal("usage was parsed for stream response")
	}
	if restored := readResponseBody(t, resp); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}
}

func newOpenrouterResponse(body string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}
}

func readResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	return string(body)
}
