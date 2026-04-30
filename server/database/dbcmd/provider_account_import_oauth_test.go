package dbcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProviderAccountOAuth(t *testing.T) {
	content := []byte(`{
		"token_type": "Bearer",
		"access_token": "XX",
		"expires_in": 28800,
		"refresh_token": "QQ",
		"scope": "user:file_upload user:inference",
		"organization": {
			"uuid": "c34ac4d7-5ac9-4923-bc78-2f3012183105",
			"name": "org"
		},
		"account": {
			"uuid": "5c2f5d10-0d29-4bf8-b073-e620af17980d",
			"email_address": "pqEandrewaverypXe9@publicist.com"
		}
	}`)

	accountUUID, payload, err := parseProviderAccountOAuth(content)
	if err != nil {
		t.Fatalf("parseProviderAccountOAuth() error = %v", err)
	}
	if accountUUID != "5c2f5d10-0d29-4bf8-b073-e620af17980d" {
		t.Fatalf("parseProviderAccountOAuth() accountUUID = %q, want expected uuid", accountUUID)
	}
	if payload.AccessToken != "XX" {
		t.Fatalf("parseProviderAccountOAuth() access token = %q, want %q", payload.AccessToken, "XX")
	}
	if payload.RefreshToken != "QQ" {
		t.Fatalf("parseProviderAccountOAuth() refresh token = %q, want %q", payload.RefreshToken, "QQ")
	}
}

func TestImportProviderAccountOAuth(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database")
	oauthPath := filepath.Join(t.TempDir(), "oauth.json")
	content := `{
		"token_type": "Bearer",
		"access_token": "XX",
		"expires_in": 28800,
		"refresh_token": "QQ",
		"scope": "user:file_upload user:inference",
		"organization": {
			"uuid": "c34ac4d7-5ac9-4923-bc78-2f3012183105",
			"name": "org"
		},
		"account": {
			"uuid": "5c2f5d10-0d29-4bf8-b073-e620af17980d",
			"email_address": "pqEandrewaverypXe9@publicist.com"
		}
	}`
	if err := os.WriteFile(oauthPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write oauth file error = %v", err)
	}

	item, err := importProviderAccountOAuth(oauthPath, &dbPath)
	if err != nil {
		t.Fatalf("importProviderAccountOAuth() error = %v", err)
	}

	if item.AccountUUID != "5c2f5d10-0d29-4bf8-b073-e620af17980d" {
		t.Fatalf("importProviderAccountOAuth() account_uuid = %q, want expected uuid", item.AccountUUID)
	}
	if item.AccessToken != "XX" {
		t.Fatalf("importProviderAccountOAuth() access_token = %q, want %q", item.AccessToken, "XX")
	}
	if item.RefreshToken != "QQ" {
		t.Fatalf("importProviderAccountOAuth() refresh_token = %q, want %q", item.RefreshToken, "QQ")
	}
	if item.Credentials != content {
		t.Fatalf("importProviderAccountOAuth() credentials = %q, want original content", item.Credentials)
	}
}

func TestImportProviderAccountOAuthUpdatesExistingRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database")
	oauthPath := filepath.Join(t.TempDir(), "oauth.json")
	first := `{"access_token":"XX","refresh_token":"QQ","account":{"uuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d"}}`
	if err := os.WriteFile(oauthPath, []byte(first), 0o600); err != nil {
		t.Fatalf("write first oauth file error = %v", err)
	}

	item1, err := importProviderAccountOAuth(oauthPath, &dbPath)
	if err != nil {
		t.Fatalf("first importProviderAccountOAuth() error = %v", err)
	}

	second := `{"access_token":"XX2","refresh_token":"QQ2","account":{"uuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d"}}`
	if err := os.WriteFile(oauthPath, []byte(second), 0o600); err != nil {
		t.Fatalf("write second oauth file error = %v", err)
	}

	item2, err := importProviderAccountOAuth(oauthPath, &dbPath)
	if err != nil {
		t.Fatalf("second importProviderAccountOAuth() error = %v", err)
	}

	if item2.ID != item1.ID {
		t.Fatalf("updated provider account id = %d, want %d", item2.ID, item1.ID)
	}
	if item2.AccessToken != "XX2" {
		t.Fatalf("updated access_token = %q, want %q", item2.AccessToken, "XX2")
	}
	if item2.RefreshToken != "QQ2" {
		t.Fatalf("updated refresh_token = %q, want %q", item2.RefreshToken, "QQ2")
	}
	if item2.Credentials != second {
		t.Fatalf("updated credentials = %q, want %q", item2.Credentials, second)
	}
}
