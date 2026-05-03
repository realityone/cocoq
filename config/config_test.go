package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	want := filepath.Join("/tmp/cocoq-home", ".cocoq", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestDefaultRootDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	rootDir, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir() error = %v", err)
	}

	want := filepath.Join("/tmp/cocoq-home", ".cocoq")
	if rootDir != want {
		t.Fatalf("DefaultRootDir() = %q, want %q", rootDir, want)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantRootDir := filepath.Join("/tmp/cocoq-home", ".cocoq")
	if cfg.Global.RootDir != wantRootDir {
		t.Fatalf("cfg.Global.RootDir = %q, want %q", cfg.Global.RootDir, wantRootDir)
	}
	if cfg.Server.RootDir != wantRootDir {
		t.Fatalf("cfg.Server.RootDir = %q, want %q", cfg.Server.RootDir, wantRootDir)
	}
	if cfg.Database.RootDir != wantRootDir {
		t.Fatalf("cfg.Database.RootDir = %q, want %q", cfg.Database.RootDir, wantRootDir)
	}
	if cfg.Server.Addr != "127.0.0.1:8888" {
		t.Fatalf("cfg.Server.Addr = %q, want %q", cfg.Server.Addr, "127.0.0.1:8888")
	}
	assertDefaultAPIServices(t, cfg.Server.APIServices)
	if cfg.Server.HARFile != "" {
		t.Fatalf("cfg.Server.HARFile = %q, want empty", cfg.Server.HARFile)
	}
	if cfg.Server.Verbose {
		t.Fatal("cfg.Server.Verbose = true, want false")
	}
	if cfg.Database.Path != "database.db" {
		t.Fatalf("cfg.Database.Path = %q, want %q", cfg.Database.Path, "database.db")
	}
	if cfg.Server.CA.CertFile != "ca.crt" {
		t.Fatalf("cfg.Server.CA.CertFile = %q, want %q", cfg.Server.CA.CertFile, "ca.crt")
	}
	if cfg.Server.CA.KeyFile != "ca.key" {
		t.Fatalf("cfg.Server.CA.KeyFile = %q, want %q", cfg.Server.CA.KeyFile, "ca.key")
	}
}

func TestLoadConfigFile(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "root")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
global:
  root_dir: ` + rootDir + `
server:
  addr: 127.0.0.1:9999
  api_services:
    - name: anthropic
    - name: openrouter
      options:
        provider: openai
  har_file: /tmp/cocoq.har
  verbose: true
  ca:
    cert_file: custom-ca.crt
    key_file: custom-ca.key
database:
  path: /tmp/cocoq.db
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Addr != "127.0.0.1:9999" {
		t.Fatalf("cfg.Server.Addr = %q, want %q", cfg.Server.Addr, "127.0.0.1:9999")
	}
	if len(cfg.Server.APIServices) != 2 {
		t.Fatalf("len(cfg.Server.APIServices) = %d, want 2", len(cfg.Server.APIServices))
	}
	if cfg.Server.APIServices[0].Name != APIServiceAnthropic {
		t.Fatalf("cfg.Server.APIServices[0].Name = %q, want %q", cfg.Server.APIServices[0].Name, APIServiceAnthropic)
	}
	if cfg.Server.APIServices[1].Name != APIServiceOpenRouter {
		t.Fatalf("cfg.Server.APIServices[1].Name = %q, want %q", cfg.Server.APIServices[1].Name, APIServiceOpenRouter)
	}
	if provider := openRouterProviderFromOptions(t, cfg.Server.APIServices[1].Options); provider != "openai" {
		t.Fatalf("cfg.Server.APIServices[1].Options.provider = %q, want openai", provider)
	}
	if cfg.Global.RootDir != rootDir {
		t.Fatalf("cfg.Global.RootDir = %q, want %q", cfg.Global.RootDir, rootDir)
	}
	if cfg.Server.RootDir != rootDir {
		t.Fatalf("cfg.Server.RootDir = %q, want %q", cfg.Server.RootDir, rootDir)
	}
	if cfg.Database.RootDir != rootDir {
		t.Fatalf("cfg.Database.RootDir = %q, want %q", cfg.Database.RootDir, rootDir)
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

func TestLoadPartialConfigPreservesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
server:
  verbose: true
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Addr != "127.0.0.1:8888" {
		t.Fatalf("cfg.Server.Addr = %q, want %q", cfg.Server.Addr, "127.0.0.1:8888")
	}
	assertDefaultAPIServices(t, cfg.Server.APIServices)
	if !cfg.Server.Verbose {
		t.Fatal("cfg.Server.Verbose = false, want true")
	}
	if cfg.Database.Path != "database.db" {
		t.Fatalf("cfg.Database.Path = %q, want %q", cfg.Database.Path, "database.db")
	}
	if cfg.Server.CA.CertFile != "ca.crt" {
		t.Fatalf("cfg.Server.CA.CertFile = %q, want %q", cfg.Server.CA.CertFile, "ca.crt")
	}
}

func TestDefaultYAMLIncludesCommentsAndAllFields(t *testing.T) {
	t.Setenv("HOME", "/tmp/cocoq-home")

	content := DefaultYAML()
	for _, want := range []string{
		"# Global settings shared by all commands.",
		"global:",
		"# Root directory for runtime files.",
		"  root_dir:",
		"# Proxy server settings.",
		"server:",
		"# HTTP listen address for the local proxy server.",
		"  addr:",
		"# API services to install.",
		"  api_services:",
		"    - name:",
		"      options:",
		"        provider:",
		"# HAR output file path.",
		"  har_file:",
		"# Enable verbose proxy logging.",
		"  verbose:",
		"# Root CA files used for MITM TLS.",
		"  ca:",
		"# Root CA certificate file.",
		"    cert_file:",
		"# Root CA private key file.",
		"    key_file:",
		"# Database settings.",
		"database:",
		"# SQLite database file path.",
		"  path:",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("DefaultYAML() missing %q in:\n%s", want, content)
		}
	}
	hasActiveYAMLLine := false
	for _, line := range strings.Split(content, "\n") {
		if line != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			hasActiveYAMLLine = true
		}
	}
	if !hasActiveYAMLLine {
		t.Fatalf("DefaultYAML() has no active YAML lines:\n%s", content)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantRootDir := filepath.Join("/tmp/cocoq-home", ".cocoq")
	if cfg.Global.RootDir != wantRootDir {
		t.Fatalf("cfg.Global.RootDir = %q, want %q", cfg.Global.RootDir, wantRootDir)
	}
	if cfg.Server.Addr != "127.0.0.1:8888" {
		t.Fatalf("cfg.Server.Addr = %q, want %q", cfg.Server.Addr, "127.0.0.1:8888")
	}
	assertDefaultAPIServices(t, cfg.Server.APIServices)
	if cfg.Server.HARFile != "" {
		t.Fatalf("cfg.Server.HARFile = %q, want empty", cfg.Server.HARFile)
	}
	if cfg.Server.Verbose {
		t.Fatal("cfg.Server.Verbose = true, want false")
	}
	if cfg.Server.CA.CertFile != "ca.crt" {
		t.Fatalf("cfg.Server.CA.CertFile = %q, want %q", cfg.Server.CA.CertFile, "ca.crt")
	}
	if cfg.Server.CA.KeyFile != "ca.key" {
		t.Fatalf("cfg.Server.CA.KeyFile = %q, want %q", cfg.Server.CA.KeyFile, "ca.key")
	}
	if cfg.Database.Path != "database.db" {
		t.Fatalf("cfg.Database.Path = %q, want %q", cfg.Database.Path, "database.db")
	}
}

func assertDefaultAPIServices(t *testing.T, services []APIServiceConfig) {
	t.Helper()

	if len(services) != 1 {
		t.Fatalf("len(APIServices) = %d, want 1", len(services))
	}
	if services[0].Name != APIServiceOpenRouter {
		t.Fatalf("APIServices[0].Name = %q, want %q", services[0].Name, APIServiceOpenRouter)
	}
	if provider := openRouterProviderFromOptions(t, services[0].Options); provider != defaultOpenRouterProvider {
		t.Fatalf("APIServices[0].Options.provider = %q, want %q", provider, defaultOpenRouterProvider)
	}
}

func openRouterProviderFromOptions(t *testing.T, raw json.RawMessage) string {
	t.Helper()

	var options struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		t.Fatalf("unmarshal options %s: %v", raw, err)
	}
	return options.Provider
}
