package anthropic

import (
	"strings"
	"testing"
	"time"

	"cocoq/server/proxy"

	"github.com/tidwall/gjson"
)

func TestForcePromptCurrentDateUTCRewritesCurrentDateBlock(t *testing.T) {
	c := currentDateUTC{}
	body := []byte(`{
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"<system-reminder>\nThe following skills are available for use with the Skill tool:\n</system-reminder>\n"},
				{"type":"text","text":"<system-reminder>\nAs you answer the user's questions, you can use the following context:\n# currentDate\nToday's date is 2026-04-31.\n\n      IMPORTANT: this context may or may not be relevant to your tasks. You should not respond to this context unless it is highly relevant to your task.\n</system-reminder>\n\n"},
				{"type":"text","text":"Hello","cache_control":{"type":"ephemeral"}}
			]}
		]
	}`)

	got, changed, err := c.forcePromptCurrentDateUTC(body, "2026-04-30")
	if err != nil {
		t.Fatalf("forcePromptCurrentDateUTC returned error: %v", err)
	}
	if !changed {
		t.Fatal("forcePromptCurrentDateUTC changed = false, want true")
	}

	text := gjson.GetBytes(got, "messages.0.content.1.text").String()
	if !strings.Contains(text, "Today's date is 2026-04-30.") {
		t.Fatalf("currentDate text was not rewritten: %q", text)
	}
	if strings.Contains(text, "2026-04-31") {
		t.Fatalf("currentDate text still contains old date: %q", text)
	}
	if gotType := gjson.GetBytes(got, "messages.0.content.2.cache_control.type").String(); gotType != "ephemeral" {
		t.Fatalf("cache_control.type = %q, want ephemeral", gotType)
	}
}

func TestForcePromptCurrentDateUTCNoopWhenAlreadyUTCDate(t *testing.T) {
	c := currentDateUTC{}
	body := []byte(`{
		"messages":[{"role":"user","content":[{"type":"text","text":"<system-reminder>\n# currentDate\nToday's date is 2026-04-30.\n</system-reminder>\n"}]}]
	}`)

	got, changed, err := c.forcePromptCurrentDateUTC(body, "2026-04-30")
	if err != nil {
		t.Fatalf("forcePromptCurrentDateUTC returned error: %v", err)
	}
	if changed {
		t.Fatal("forcePromptCurrentDateUTC changed = true, want false")
	}
	if string(got) != string(body) {
		t.Fatalf("body changed on noop\n got: %s\nwant: %s", got, body)
	}
}

func TestForcePromptCurrentDateUTCSkipsNonCurrentDateText(t *testing.T) {
	c := currentDateUTC{}
	body := []byte(`{
		"messages":[{"role":"user","content":[{"type":"text","text":"Today's date is 2026-04-31."}]}]
	}`)

	got, changed, err := c.forcePromptCurrentDateUTC(body, "2026-04-30")
	if err != nil {
		t.Fatalf("forcePromptCurrentDateUTC returned error: %v", err)
	}
	if changed {
		t.Fatal("forcePromptCurrentDateUTC changed = true, want false")
	}
	if string(got) != string(body) {
		t.Fatalf("body changed on skipped text\n got: %s\nwant: %s", got, body)
	}
}

func TestCurrentDateUTCHandleUsesUTCDate(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"<system-reminder>\n# currentDate\nToday's date is 2026-05-01.\n</system-reminder>\n"}]}]}`
	req := newAnthropicMessagesRequest(t, body)
	ctx := &proxy.OnContext[proxy.ReqCtx]{
		Opaque: proxy.ReqCtx{Request: req},
	}
	c := currentDateUTC{
		now: func() time.Time {
			return time.Date(2026, 5, 1, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
		},
	}

	c.Handle(ctx)

	got := readRequestBody(t, ctx.Opaque.PostRequest)
	text := gjson.GetBytes(got, "messages.0.content.0.text").String()
	if !strings.Contains(text, "Today's date is 2026-04-30.") {
		t.Fatalf("currentDate text did not use UTC date: %q", text)
	}
}

func TestCurrentDateUTCDateUsesLocalClock(t *testing.T) {
	c := currentDateUTC{
		now: func() time.Time {
			return time.Date(2026, 5, 1, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
		},
	}

	got := c.utcDate()
	if got != "2026-04-30" {
		t.Fatalf("utcDate = %q, want 2026-04-30", got)
	}
}
