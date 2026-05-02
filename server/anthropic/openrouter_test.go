package anthropic

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/realityone/cocoq/server/database"
	"github.com/realityone/cocoq/server/database/dbrt"
	"github.com/realityone/cocoq/server/proxy"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
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
	client := newOpenrouterTestDB(t)
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
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)

	usage := waitForUsage(t, ctx)
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

func TestOpenrouterExtractUsageSavesNonStreamUsage(t *testing.T) {
	client := newOpenrouterTestDB(t)
	body := `{
		"usage": {
			"input_tokens": 3,
			"output_tokens": 117,
			"cache_creation_input_tokens": 24804,
			"cache_read_input_tokens": 0
		}
	}`
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{
				DeviceID:    "device-1",
				SessionID:   "session-1",
				AccountUUID: "account-1",
				Model:       "claude-sonnet-4-6",
			}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)

	record := waitForUsageRecord(t, client)
	if record.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %q, want device-1", record.DeviceID)
	}
	if record.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", record.SessionID)
	}
	if record.AccountUUID != "account-1" {
		t.Fatalf("AccountUUID = %q, want account-1", record.AccountUUID)
	}
	if record.Model != "claude-sonnet-4-6" {
		t.Fatalf("Model = %q, want claude-sonnet-4-6", record.Model)
	}
	if record.InputTokens != 3 {
		t.Fatalf("InputTokens = %d, want 3", record.InputTokens)
	}
	if record.OutputTokens != 117 {
		t.Fatalf("OutputTokens = %d, want 117", record.OutputTokens)
	}
	if record.CacheCreationEphemeral1hInputTokens != 24804 {
		t.Fatalf("CacheCreationEphemeral1hInputTokens = %d, want 24804", record.CacheCreationEphemeral1hInputTokens)
	}
	if got := gjson.Get(record.Raw, "cache_creation_input_tokens").Int(); got != 24804 {
		t.Fatalf("Raw.cache_creation_input_tokens = %d, want 24804", got)
	}
	if record.CacheHitRate != 0 {
		t.Fatalf("CacheHitRate = %g, want 0", record.CacheHitRate)
	}
}

func TestOpenrouterExtractUsageTreatsCacheCreationAs1h(t *testing.T) {
	client := newOpenrouterTestDB(t)
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
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)

	usage := waitForUsage(t, ctx)
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

func TestOpenrouterExtractUsageFromStreamResponse(t *testing.T) {
	client := newOpenrouterTestDB(t)
	body := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":null,"cache_read_input_tokens":null,"cache_creation":null}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","usage":{"input_tokens":3,"output_tokens":195,"cache_creation_input_tokens":0,"cache_read_input_tokens":24804,"cost":0.0103752}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
		"event: data",
		"data: [DONE]",
		"",
	}, "\n")
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{
				DeviceID:    "device-2",
				SessionID:   "session-2",
				AccountUUID: "account-2",
				Model:       "claude-sonnet-4-6",
				Stream:      true,
			}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)

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
	if usage.InputTokens != 3 {
		t.Fatalf("InputTokens = %d, want 3", usage.InputTokens)
	}
	if usage.OutputTokens != 195 {
		t.Fatalf("OutputTokens = %d, want 195", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 0 {
		t.Fatalf("CacheCreationInputTokens = %d, want 0", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 24804 {
		t.Fatalf("CacheReadInputTokens = %d, want 24804", usage.CacheReadInputTokens)
	}
	if len(usage.GetRaw()) == 0 {
		t.Fatal("raw usage JSON was not recorded")
	}

	record := waitForUsageRecord(t, client)
	if record.DeviceID != "device-2" {
		t.Fatalf("DeviceID = %q, want device-2", record.DeviceID)
	}
	if record.SessionID != "session-2" {
		t.Fatalf("SessionID = %q, want session-2", record.SessionID)
	}
	if record.AccountUUID != "account-2" {
		t.Fatalf("AccountUUID = %q, want account-2", record.AccountUUID)
	}
	if record.Model != "claude-sonnet-4-6" {
		t.Fatalf("Model = %q, want claude-sonnet-4-6", record.Model)
	}
	if record.CacheReadInputTokens != 24804 {
		t.Fatalf("CacheReadInputTokens = %d, want 24804", record.CacheReadInputTokens)
	}
	if math.Abs(record.CacheHitRate-24804.0/24807.0) > 0.000001 {
		t.Fatalf("CacheHitRate = %g, want %g", record.CacheHitRate, 24804.0/24807.0)
	}
}

func TestOpenrouterExtractUsageFromStreamResponseSavesEveryUsage(t *testing.T) {
	client := newOpenrouterTestDB(t)
	body := strings.Join([]string{
		"event: message_delta",
		`data: {"type":"message_delta","usage":{"input_tokens":1,"output_tokens":2,"cache_creation_input_tokens":3,"cache_read_input_tokens":4}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","usage":{"input_tokens":5,"output_tokens":6,"cache_creation_input_tokens":7,"cache_read_input_tokens":8}}`,
		"",
		"event: data",
		"data: [DONE]",
		"",
	}, "\n")
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{
				DeviceID:    "device-3",
				SessionID:   "session-3",
				AccountUUID: "account-3",
				Model:       "claude-sonnet-4-6",
				Stream:      true,
			}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)
	if restored := readResponseBody(t, resp); restored != body {
		t.Fatalf("body = %s, want %s", restored, body)
	}

	records := waitForUsageRecordCount(t, client, 2)
	outputTokens := map[int64]bool{}
	for _, record := range records {
		if record.Model != "claude-sonnet-4-6" {
			t.Fatalf("Model = %q, want claude-sonnet-4-6", record.Model)
		}
		outputTokens[record.OutputTokens] = true
	}
	if !outputTokens[2] || !outputTokens[6] {
		t.Fatalf("saved output tokens = %v, want 2 and 6", outputTokens)
	}
}

func TestOpenrouterUsageLogFieldsFormatsRawUsageAsJSON(t *testing.T) {
	var out bytes.Buffer
	logger := logrus.StandardLogger()
	originalOut := logger.Out
	originalFormatter := logger.Formatter
	originalLevel := logger.Level
	logger.SetOutput(&out)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableQuote:     true,
	})
	logger.SetLevel(logrus.InfoLevel)
	defer func() {
		logger.SetOutput(originalOut)
		logger.SetFormatter(originalFormatter)
		logger.SetLevel(originalLevel)
	}()

	usage := proxy.AnthropicUsage{
		InputTokens:              1,
		OutputTokens:             199,
		CacheReadInputTokens:     57010,
		CacheCreationInputTokens: 338,
	}
	usage.CacheCreation.Ephemeral1hInputTokens = 338
	usage.SetRaw([]byte(`{"input_tokens":1,"output_tokens":199,"cache_creation_input_tokens":338,"cache_read_input_tokens":57010}`))

	logrus.WithFields(usageLoggingFields(usage)).Info("extracted usage from openrouter SSE response")

	got := out.String()
	if !strings.Contains(got, `usage={"input_tokens":1,"output_tokens":199,"cache_creation_input_tokens":338,"cache_read_input_tokens":57010}`) {
		t.Fatalf("log output = %q, want usage field to contain raw JSON", got)
	}
	if strings.Contains(got, "[123 34 105 110") {
		t.Fatalf("log output = %q, unexpectedly contained byte-slice dump", got)
	}
}

func TestOpenrouterExtractUsageSkipsStreamResponseWithoutUsage(t *testing.T) {
	client := newOpenrouterTestDB(t)
	body := `{"usage":{"input_tokens":3}}`
	resp := newOpenrouterResponse(body)
	ctx := &proxy.OnContext[proxy.RespCtx]{
		Opaque: proxy.RespCtx{
			ProxyCtx: newOpenrouterProxyCtx(t, &UserData{Stream: true}),
			Response: resp,
		},
	}

	newOpenrouterTestProxy(client).extactUsage(ctx)

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

func newOpenrouterTestProxy(client *dbrt.Client) *openrouterProxy {
	return &openrouterProxy{
		anthropicProxy: &anthropicProxy{db: client},
	}
}

func newOpenrouterProxyCtx(t *testing.T, data *UserData) *goproxy.ProxyCtx {
	t.Helper()

	return &goproxy.ProxyCtx{
		Req:      newAnthropicMessagesRequest(t, "{}"),
		UserData: data,
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

func waitForUsage(t *testing.T, ctx *proxy.OnContext[proxy.RespCtx]) proxy.AnthropicUsage {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		usage := ctx.Opaque.Metrics.Usage
		if len(usage.GetRaw()) > 0 {
			return usage
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for usage")
	return proxy.AnthropicUsage{}
}

func newOpenrouterTestDB(t *testing.T) *dbrt.Client {
	t.Helper()

	client, err := database.OpenClient(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("OpenClient() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return client
}

func waitForUsageRecord(t *testing.T, client *dbrt.Client) *dbrt.AnthropicUsage {
	t.Helper()

	records := waitForUsageRecordCount(t, client, 1)
	return records[0]
}

func waitForUsageRecordCount(t *testing.T, client *dbrt.Client, want int) []*dbrt.AnthropicUsage {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		records, err := client.AnthropicUsage.Query().All(context.Background())
		if err != nil {
			if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
				time.Sleep(time.Millisecond)
				continue
			}
			t.Fatalf("query AnthropicUsage: %v", err)
		}
		if len(records) == want {
			return records
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d usage records", want)
	return nil
}
