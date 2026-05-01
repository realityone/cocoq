package anthropic

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"cocoq/server/proxy"

	"github.com/tidwall/gjson"
)

func TestForcePromptCacheControlTTL1hSetsEphemeralPromptCacheControlBlocks(t *testing.T) {
	c := caching1h{}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"billing header"},
			{"type":"text","text":"cached system","cache_control":{"type":"ephemeral"}},
			{"type":"text","text":"cached system 5m","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"cached system non-ephemeral","cache_control":{"type":"persistent"}}
		],
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"reminder"},
				{"type":"text","text":"cached user","cache_control":{"type":"ephemeral"}},
				{"type":"text","text":"cached user missing type","cache_control":{}}
			]},
			{"role":"assistant","content":[
				{"type":"text","text":"cached assistant","cache_control":{"type":"ephemeral","ttl":"5m"}}
			]}
		]
	}`)

	got, changed, err := c.forcePromptCacheControlTTL1h(body)
	if err != nil {
		t.Fatalf("forcePromptCacheControlTTL1h returned error: %v", err)
	}
	if !changed {
		t.Fatal("forcePromptCacheControlTTL1h changed = false, want true")
	}

	assertJSONEqual(t, got, "system.1.cache_control.ttl", "1h")
	assertJSONEqual(t, got, "system.2.cache_control.ttl", "1h")
	assertJSONEqual(t, got, "messages.0.content.1.cache_control.ttl", "1h")
	assertJSONEqual(t, got, "messages.1.content.0.cache_control.ttl", "1h")

	if gjson.GetBytes(got, "system.0.cache_control").Exists() {
		t.Fatal("block without cache_control unexpectedly gained cache_control")
	}
	if gjson.GetBytes(got, "messages.0.content.0.cache_control").Exists() {
		t.Fatal("message block without cache_control unexpectedly gained cache_control")
	}
	if gjson.GetBytes(got, "system.3.cache_control.ttl").Exists() {
		t.Fatal("non-ephemeral system cache_control unexpectedly gained ttl")
	}
	if gjson.GetBytes(got, "messages.0.content.2.cache_control.ttl").Exists() {
		t.Fatal("message cache_control without ephemeral type unexpectedly gained ttl")
	}
}

func TestForcePromptCacheControlTTL1hNoopWhenAlready1h(t *testing.T) {
	c := caching1h{}
	body := []byte(`{
		"system":[{"type":"text","text":"cached system","cache_control":{"type":"ephemeral","ttl":"1h"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"cached user","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]
	}`)

	got, changed, err := c.forcePromptCacheControlTTL1h(body)
	if err != nil {
		t.Fatalf("forcePromptCacheControlTTL1h returned error: %v", err)
	}
	if changed {
		t.Fatal("forcePromptCacheControlTTL1h changed = true, want false")
	}
	if string(got) != string(body) {
		t.Fatalf("body changed on noop\n got: %s\nwant: %s", got, body)
	}
}

func TestForcePromptCacheControlTTL1hSkipsNonBlockShapes(t *testing.T) {
	c := caching1h{}
	body := []byte(`{
		"system":"plain system prompt",
		"messages":[
			{"role":"user","content":"plain message content"},
			{"role":"assistant","content":[{"type":"text","text":"plain assistant"}]}
		]
	}`)

	got, changed, err := c.forcePromptCacheControlTTL1h(body)
	if err != nil {
		t.Fatalf("forcePromptCacheControlTTL1h returned error: %v", err)
	}
	if changed {
		t.Fatal("forcePromptCacheControlTTL1h changed = true, want false")
	}
	if string(got) != string(body) {
		t.Fatalf("body changed on skipped shapes\n got: %s\nwant: %s", got, body)
	}
}

func TestCaching1hHandleRequestRewritesRequestBody(t *testing.T) {
	body := `{
		"system":[{"type":"text","text":"cached system","cache_control":{"type":"ephemeral"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"cached user","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]
	}`
	req := newAnthropicMessagesRequest(t, body)
	ctx := &proxy.OnContext[proxy.ReqCtx]{
		Opaque: proxy.ReqCtx{Request: req},
	}

	(&caching1h{}).Handle(ctx)

	if ctx.Opaque.PostRequest != req {
		t.Fatal("HandleRequest did not set PostRequest to the rewritten request")
	}

	got := readRequestBody(t, ctx.Opaque.PostRequest)
	assertJSONEqual(t, got, "system.0.cache_control.ttl", "1h")
	assertJSONEqual(t, got, "messages.0.content.0.cache_control.ttl", "1h")
	if req.ContentLength != int64(len(got)) {
		t.Fatalf("ContentLength = %d, want %d", req.ContentLength, len(got))
	}
}

func TestCaching1hHandleRequestRestoresBodyOnNoop(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"uncached"}]}]}`
	req := newAnthropicMessagesRequest(t, body)
	ctx := &proxy.OnContext[proxy.ReqCtx]{
		Opaque: proxy.ReqCtx{Request: req},
	}

	(&caching1h{}).Handle(ctx)

	got := readRequestBody(t, ctx.Opaque.PostRequest)
	if string(got) != body {
		t.Fatalf("body = %s, want %s", got, body)
	}
}

func newAnthropicMessagesRequest(t *testing.T, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	return req
}

func readRequestBody(t *testing.T, req *http.Request) []byte {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	return body
}

func assertJSONEqual(t *testing.T, body []byte, path string, want string) {
	t.Helper()

	if got := gjson.GetBytes(body, path).String(); got != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
