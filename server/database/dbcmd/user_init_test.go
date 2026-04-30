package dbcmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRootUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")

	user, err := initRootUser(context.Background(), path)
	if err != nil {
		t.Fatalf("initRootUser() error = %v", err)
	}

	if user.ID == 0 {
		t.Fatalf("initRootUser() user ID = %d, want non-zero", user.ID)
	}
	if user.State != initialUserState {
		t.Fatalf("initRootUser() state = %d, want %d", user.State, initialUserState)
	}
	if user.OauthStateToken == "" {
		t.Fatal("initRootUser() oauth_state_token is empty")
	}
	if !strings.Contains(user.OauthStateToken, "#") {
		t.Fatalf("initRootUser() oauth_state_token = %q, want token with separator", user.OauthStateToken)
	}
}

func TestInitRootUserRefusesExistingUserOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")

	_, err := initRootUser(context.Background(), path)
	if err != nil {
		t.Fatalf("first initRootUser() error = %v", err)
	}

	_, err = initRootUser(context.Background(), path)
	if err == nil {
		t.Fatal("second initRootUser() error = nil, want refusal")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Fatalf("second initRootUser() error = %v, want already initialized", err)
	}
}
