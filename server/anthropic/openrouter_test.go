package anthropic

import (
	"testing"

	"github.com/realityone/cocoq/server/proxy"

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
