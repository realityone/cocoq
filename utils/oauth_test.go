package utils

import (
	"strings"
	"testing"
)

func TestGenerateOAuthState(t *testing.T) {
	state, err := GenerateOAuthState()
	if err != nil {
		t.Fatalf("GenerateOAuthState() error = %v", err)
	}

	parts := strings.Split(state, "#")
	if len(parts) != 2 {
		t.Fatalf("GenerateOAuthState() = %q, want exactly one #", state)
	}

	if len(parts[0]) != 48 {
		t.Fatalf("left segment length = %d, want 48", len(parts[0]))
	}

	if len(parts[1]) != 43 {
		t.Fatalf("right segment length = %d, want 43", len(parts[1]))
	}

	for _, part := range parts {
		for _, r := range part {
			if !strings.ContainsRune(OAuthStateAlphabet, r) {
				t.Fatalf("unexpected rune %q in state %q", r, state)
			}
		}
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if !strings.HasPrefix(token, RefreshTokenPrefix) {
		t.Fatalf("GenerateRefreshToken() = %q, want prefix %q", token, RefreshTokenPrefix)
	}

	if len(token) != 108 {
		t.Fatalf("token length = %d, want 108", len(token))
	}

	for _, r := range token[len(RefreshTokenPrefix):] {
		if !strings.ContainsRune(OAuthStateAlphabet, r) {
			t.Fatalf("unexpected rune %q in token %q", r, token)
		}
	}
}

func TestGenerateAccessToken(t *testing.T) {
	token, err := GenerateAccessToken()
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	if !strings.HasPrefix(token, AccessTokenPrefix) {
		t.Fatalf("GenerateAccessToken() = %q, want prefix %q", token, AccessTokenPrefix)
	}

	if len(token) != 108 {
		t.Fatalf("token length = %d, want 108", len(token))
	}

	for _, r := range token[len(AccessTokenPrefix):] {
		if !strings.ContainsRune(OAuthStateAlphabet, r) {
			t.Fatalf("unexpected rune %q in token %q", r, token)
		}
	}
}
