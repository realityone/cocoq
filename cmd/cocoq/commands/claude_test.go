package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfig "github.com/realityone/cocoq/config"
)

func TestNewClaudeCmd(t *testing.T) {
	cmd := NewClaudeCmd(func() (appconfig.Config, error) {
		return appconfig.Config{}, nil
	})
	if cmd.Name() != "claude" {
		t.Fatalf("command name = %q, want %q", cmd.Name(), "claude")
	}
}

func TestClaudeCommandUsesConfigEnv(t *testing.T) {
	oldRunCommand := runCommand
	t.Cleanup(func() {
		runCommand = oldRunCommand
	})

	rootDir := t.TempDir()
	caPath := filepath.Join(rootDir, "ca.crt")
	if err := os.WriteFile(caPath, []byte("cert"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var gotName string
	var gotArgs []string
	var gotEnv []string
	runCommand = func(name string, args []string, env []string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		gotEnv = append([]string(nil), env...)
		return nil
	}

	cmd := newClaudeCmd(func() (appconfig.Config, error) {
		return appconfig.Config{
			Global: appconfig.GlobalConfig{
				RootDir: rootDir,
			},
			Server: appconfig.ServerConfig{
				RootDir: rootDir,
				Addr:    "127.0.0.1:9999",
				CA: appconfig.CAConfig{
					CertFile: "ca.crt",
				},
			},
		}, nil
	})
	cmd.SetArgs([]string{"--model", "sonnet"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
	}

	if gotName != "claude" {
		t.Fatalf("command name = %q, want %q", gotName, "claude")
	}
	if strings.Join(gotArgs, " ") != "--model sonnet" {
		t.Fatalf("args = %q, want %q", strings.Join(gotArgs, " "), "--model sonnet")
	}
	if got := envValue(gotEnv, "HTTP_PROXY"); got != "http://127.0.0.1:9999" {
		t.Fatalf("HTTP_PROXY = %q, want %q", got, "http://127.0.0.1:9999")
	}
	if got := envValue(gotEnv, "NODE_EXTRA_CA_CERTS"); got != caPath {
		t.Fatalf("NODE_EXTRA_CA_CERTS = %q, want %q", got, caPath)
	}
}

func TestClaudeCommandRequiresCACert(t *testing.T) {
	oldRunCommand := runCommand
	t.Cleanup(func() {
		runCommand = oldRunCommand
	})

	rootDir := t.TempDir()
	called := false
	runCommand = func(name string, args []string, env []string) error {
		called = true
		return nil
	}

	cmd := newClaudeCmd(func() (appconfig.Config, error) {
		return appconfig.Config{
			Global: appconfig.GlobalConfig{
				RootDir: rootDir,
			},
			Server: appconfig.ServerConfig{
				RootDir: rootDir,
				CA: appconfig.CAConfig{
					CertFile: "missing-ca.crt",
				},
			},
		}, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute() error = nil, want missing CA error")
	}
	if !strings.Contains(err.Error(), "NODE_EXTRA_CA_CERTS file does not exist") {
		t.Fatalf("Execute() error = %q, want NODE_EXTRA_CA_CERTS missing file error", err.Error())
	}
	if called {
		t.Fatal("runCommand was called with missing CA cert")
	}
}

func envValue(env []string, key string) string {
	for _, entry := range env {
		entryKey, value, ok := strings.Cut(entry, "=")
		if ok && entryKey == key {
			return value
		}
	}
	return ""
}
