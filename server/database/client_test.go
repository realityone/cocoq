package database

import (
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/realityone/cocoq/config"
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

func TestResolveDatabasePathUsesAbsolutePathDirectly(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.db")

	got, err := resolveDatabasePath(appconfig.DatabaseConfig{Path: want})
	if err != nil {
		t.Fatalf("resolveDatabasePath() error = %v", err)
	}
	if got != want {
		t.Fatalf("resolveDatabasePath() = %q, want %q", got, want)
	}
}

func TestResolveDatabasePathUsesCocoqDirForRelativePath(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "root")

	got, err := resolveDatabasePath(appconfig.DatabaseConfig{RootDir: rootDir, Path: "custom.db"})
	if err != nil {
		t.Fatalf("resolveDatabasePath() error = %v", err)
	}

	want := filepath.Join(rootDir, "custom.db")
	if got != want {
		t.Fatalf("resolveDatabasePath() = %q, want %q", got, want)
	}
}

func TestOpenClientUsesCocoqDirForRelativePath(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "root")

	client, err := OpenClient(appconfig.DatabaseConfig{RootDir: rootDir, Path: "custom.db"})
	if err != nil {
		t.Fatalf("OpenClient() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("client.Close() error = %v", err)
		}
	})

	path := filepath.Join(rootDir, "custom.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat resolved database path: %v", err)
	}
}

func TestOpenClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")

	client, err := OpenClient(appconfig.DatabaseConfig{Path: path})
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
