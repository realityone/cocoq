package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCAFilePathUsesCocoqDirForRelativePath(t *testing.T) {
	caDir := filepath.Join(t.TempDir(), ".cocoq")

	got := caFilePath(caDir, "ca.crt")
	want := filepath.Join(caDir, "ca.crt")
	if got != want {
		t.Fatalf("caFilePath() = %q, want %q", got, want)
	}
}

func TestCAFilePathUsesAbsolutePathDirectly(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom-ca.crt")

	got := caFilePath(filepath.Join(t.TempDir(), ".cocoq"), want)
	if got != want {
		t.Fatalf("caFilePath() = %q, want %q", got, want)
	}
}

func TestLoadOrCreateCAAllowsAbsoluteCertAndKeyPaths(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), ".cocoq")
	caDir := filepath.Join(t.TempDir(), "ca")
	certPath := filepath.Join(caDir, "custom-ca.crt")
	keyPath := filepath.Join(caDir, "custom-ca.key")

	if _, err := LoadOrCreateCA(rootDir, certPath, keyPath); err != nil {
		t.Fatalf("LoadOrCreateCA() error = %v", err)
	}

	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("stat absolute cert path: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("stat absolute key path: %v", err)
	}
	if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
		t.Fatalf("root dir stat error = %v, want not exist", err)
	}
}

func TestLoadOrCreateCAMixesRelativeAndAbsolutePaths(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), ".cocoq")
	absoluteKeyPath := filepath.Join(t.TempDir(), "keys", "custom-ca.key")

	if _, err := LoadOrCreateCA(rootDir, "ca.crt", absoluteKeyPath); err != nil {
		t.Fatalf("LoadOrCreateCA() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, "ca.crt")); err != nil {
		t.Fatalf("stat relative cert path: %v", err)
	}
	if _, err := os.Stat(absoluteKeyPath); err != nil {
		t.Fatalf("stat absolute key path: %v", err)
	}
}
