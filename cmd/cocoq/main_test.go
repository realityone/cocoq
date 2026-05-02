package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHasConfigFlag(t *testing.T) {
	if flag := rootCmd.PersistentFlags().Lookup("config"); flag == nil {
		t.Fatal("root command is missing --config flag")
	}
}

func TestLoadConfigUsesConfigPath(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() {
		configPath = oldConfigPath
	})

	configPath = filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
server:
  addr: 127.0.0.1:9999
  har_file: /tmp/cocoq.har
  verbose: true
  ca:
    cert_file: custom-ca.crt
    key_file: custom-ca.key
database:
  path: /tmp/cocoq.db
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.Server.Addr != "127.0.0.1:9999" {
		t.Fatalf("cfg.Server.Addr = %q, want %q", cfg.Server.Addr, "127.0.0.1:9999")
	}
	if cfg.Server.HARFile != "/tmp/cocoq.har" {
		t.Fatalf("cfg.Server.HARFile = %q, want %q", cfg.Server.HARFile, "/tmp/cocoq.har")
	}
	if !cfg.Server.Verbose {
		t.Fatal("cfg.Server.Verbose = false, want true")
	}
	if cfg.Database.Path != "/tmp/cocoq.db" {
		t.Fatalf("cfg.Database.Path = %q, want %q", cfg.Database.Path, "/tmp/cocoq.db")
	}
	if cfg.Server.CA.CertFile != "custom-ca.crt" {
		t.Fatalf("cfg.Server.CA.CertFile = %q, want %q", cfg.Server.CA.CertFile, "custom-ca.crt")
	}
	if cfg.Server.CA.KeyFile != "custom-ca.key" {
		t.Fatalf("cfg.Server.CA.KeyFile = %q, want %q", cfg.Server.CA.KeyFile, "custom-ca.key")
	}
}

func TestDBCommandUsesConfigDatabasePath(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() {
		configPath = oldConfigPath
	})

	dbPath := filepath.Join(t.TempDir(), "configured.db")
	configPath = filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("database:\n  path: " + dbPath + "\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Database.Path != dbPath {
		t.Fatalf("cfg.Database.Path = %q, want %q", cfg.Database.Path, dbPath)
	}

	cmd := newDBCmd()
	cmd.SetArgs([]string{"anthropic-usage", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("stat configured database path: %v", err)
	}
}

func TestDefaultConfigCommand(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	cmd := newDefaultConfigCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"global:",
		"  root_dir:",
		"server:",
		"  addr:",
		"  har_file:",
		"  verbose:",
		"  ca:",
		"    cert_file:",
		"    key_file:",
		"database:",
		"  path:",
		"# Global settings shared by all commands.",
		"# Proxy server settings.",
		"# Root CA files used for MITM TLS.",
		"# Database settings.",
		"# Root directory for runtime files.",
		"# SQLite database file path.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("default-config output missing %q in:\n%s", want, got)
		}
	}
	hasActiveYAMLLine := false
	for _, line := range strings.Split(got, "\n") {
		if line != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			hasActiveYAMLLine = true
		}
	}
	if !hasActiveYAMLLine {
		t.Fatalf("default-config output has no active YAML lines:\n%s", got)
	}
}
