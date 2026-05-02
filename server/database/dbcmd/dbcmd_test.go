package dbcmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/realityone/cocoq/server/database"

	"github.com/spf13/cobra"
)

func TestAnthropicUsageCommandListGetAndDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database")
	id := seedAnthropicUsage(t, dbPath)

	listOut := executeDBCommand(t, dbPath, "anthropic-usage", "list")
	if !strings.Contains(listOut, "device-1") {
		t.Fatalf("list output = %q, want device id", listOut)
	}
	if !strings.Contains(listOut, "claude-sonnet-4-6") {
		t.Fatalf("list output = %q, want model", listOut)
	}

	getOut := executeDBCommand(t, dbPath, "anthropic-usage", "get", "1")
	if !strings.Contains(getOut, "0.25") {
		t.Fatalf("get output = %q, want cache hit rate", getOut)
	}

	executeDBCommand(t, dbPath, "anthropic-usage", "delete", "1")
	assertAnthropicUsageCount(t, dbPath, 0)

	if id != 1 {
		t.Fatalf("seeded ID = %d, want 1", id)
	}
}

func executeDBCommand(t *testing.T, dbPath string, args ...string) string {
	t.Helper()

	root := &cobra.Command{Use: "db"}
	Register(root, &dbPath)
	root.SetArgs(args)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v\noutput:\n%s", args, err, out.String())
	}
	return out.String()
}

func seedAnthropicUsage(t *testing.T, dbPath string) int {
	t.Helper()

	client, err := database.OpenClient(dbPath)
	if err != nil {
		t.Fatalf("OpenClient() error = %v", err)
	}
	defer client.Close()

	record, err := client.AnthropicUsage.Create().
		SetDeviceID("device-1").
		SetSessionID("session-1").
		SetAccountUUID("account-1").
		SetModel("claude-sonnet-4-6").
		SetInputTokens(100).
		SetCacheReadInputTokens(20).
		SetCacheCreationInputTokens(30).
		SetOutputTokens(200).
		SetCacheCreationEphemeral5mInputTokens(10).
		SetCacheCreationEphemeral1hInputTokens(15).
		SetCacheHitRate(0.25).
		SetRaw(`{"usage":true}`).
		Save(context.Background())
	if err != nil {
		t.Fatalf("create AnthropicUsage: %v", err)
	}
	return record.ID
}

func assertAnthropicUsageCount(t *testing.T, dbPath string, want int) {
	t.Helper()

	client, err := database.OpenClient(dbPath)
	if err != nil {
		t.Fatalf("OpenClient() error = %v", err)
	}
	defer client.Close()

	got, err := client.AnthropicUsage.Query().Count(context.Background())
	if err != nil {
		t.Fatalf("count AnthropicUsage: %v", err)
	}
	if got != want {
		t.Fatalf("AnthropicUsage count = %d, want %d", got, want)
	}
}
