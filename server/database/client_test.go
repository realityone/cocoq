package database

import (
	"path/filepath"
	"testing"
)

func TestDefaultDatabasePath(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	path, err := defaultDatabasePath()
	if err != nil {
		t.Fatalf("defaultDatabasePath() error = %v", err)
	}

	if path != "/tmp/cocoq-home/.cocoq/database.db" {
		t.Fatalf("defaultDatabasePath() = %q, want %q", path, "/tmp/cocoq-home/.cocoq/database.db")
	}
}

func TestOpenClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")

	client, err := OpenClient(path)
	if err != nil {
		t.Fatalf("OpenClient() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("client.Close() error = %v", err)
		}
	})

	if client.AnthropicUsage == nil {
		t.Fatal("OpenClient() returned client with nil AnthropicUsage client")
	}
	if client.Schema == nil {
		t.Fatal("OpenClient() returned client with nil Schema")
	}
}
