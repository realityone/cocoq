package sse

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestForwardPassesThroughBodyAndSendsEvents(t *testing.T) {
	body := strings.Join([]string{
		"id: 1",
		"event: content_block_delta",
		"data: {\"type\":\"text_delta\"}",
		"",
		"event: message_stop",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	events := Forward(upstream)

	got, err := io.ReadAll(upstream.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != body {
		t.Fatalf("forwarded body = %q, want %q", got, body)
	}

	first := <-events
	if first.Id != "1" || first.Event != "content_block_delta" || first.Data != `{"type":"text_delta"}` {
		t.Fatalf("first event = %#v", first)
	}
	second := <-events
	if second.Event != "message_stop" || second.Data != "[DONE]" {
		t.Fatalf("second event = %#v", second)
	}
	if event, ok := <-events; ok {
		t.Fatalf("unexpected extra event = %#v", event)
	}
}

func TestForwardSendsUnterminatedEventAtEOF(t *testing.T) {
	upstream := &http.Response{
		Body: io.NopCloser(strings.NewReader("event: message\ndata: done")),
	}

	events := Forward(upstream)
	if _, err := io.ReadAll(upstream.Body); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	event := <-events
	if event.Event != "message" || event.Data != "done" {
		t.Fatalf("event = %#v", event)
	}
	if event, ok := <-events; ok {
		t.Fatalf("unexpected extra event = %#v", event)
	}
}
