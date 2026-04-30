package dbcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProviderAccountImport(t *testing.T) {
	credentialsContent := []byte(`{"claudeAiOauth":{"accessToken":"VVV","refreshToken":"KKK","expiresAt":1775166055914,"scopes":["user:file_upload"],"subscriptionType":"pro","rateLimitTier":"default_claude_ai"}}`)
	claudeContent := []byte(`{"oauthAccount":{"accountUuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","emailAddress":"pqEandrewaverypXe9@publicist.com","organizationUuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","organizationName":"pqEandrewaverypXe9@publicist.com's Organization"}}`)

	payload, err := parseProviderAccountCredentials(credentialsContent)
	if err != nil {
		t.Fatalf("parseProviderAccountCredentials() error = %v", err)
	}
	accountUUID, err := parseProviderAccountClaude(claudeContent)
	if err != nil {
		t.Fatalf("parseProviderAccountClaude() error = %v", err)
	}
	if accountUUID != "5c2f5d10-0d29-4bf8-b073-e620af17980d" {
		t.Fatalf("parseProviderAccountClaude() accountUUID = %q, want expected uuid", accountUUID)
	}
	if payload.AccessToken != "VVV" {
		t.Fatalf("parseProviderAccountCredentials() access token = %q, want %q", payload.AccessToken, "VVV")
	}
	if payload.RefreshToken != "KKK" {
		t.Fatalf("parseProviderAccountCredentials() refresh token = %q, want %q", payload.RefreshToken, "KKK")
	}

	constructedCredentials, err := constructProviderAccountCredentials(credentialsContent, claudeContent)
	if err != nil {
		t.Fatalf("constructProviderAccountCredentials() error = %v", err)
	}
	expectedCredentials := `{"token_type":"Bearer","expires_in":28800,"scope":"user:file_upload","organization":{"uuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","name":"pqEandrewaverypXe9@publicist.com's Organization"},"account":{"uuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","email_address":"pqEandrewaverypXe9@publicist.com"}}`
	if constructedCredentials != expectedCredentials {
		t.Fatalf("constructProviderAccountCredentials() = %q, want %q", constructedCredentials, expectedCredentials)
	}
}

func TestImportProviderAccount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database")
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	claudePath := filepath.Join(t.TempDir(), "claude.json")
	credentialsContent := `{"claudeAiOauth":{"accessToken":"VVV","refreshToken":"KKK","expiresAt":1775166055914,"scopes":["user:file_upload"],"subscriptionType":"pro","rateLimitTier":"default_claude_ai"}}`
	claudeContent := `{"oauthAccount":{"accountUuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","emailAddress":"pqEandrewaverypXe9@publicist.com","organizationUuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","organizationName":"pqEandrewaverypXe9@publicist.com's Organization"}}`
	if err := os.WriteFile(credentialsPath, []byte(credentialsContent), 0o600); err != nil {
		t.Fatalf("write credentials file error = %v", err)
	}
	if err := os.WriteFile(claudePath, []byte(claudeContent), 0o600); err != nil {
		t.Fatalf("write claude file error = %v", err)
	}

	item, err := importProviderAccount(credentialsPath, claudePath, &dbPath)
	if err != nil {
		t.Fatalf("importProviderAccount() error = %v", err)
	}

	if item.AccountUUID != "5c2f5d10-0d29-4bf8-b073-e620af17980d" {
		t.Fatalf("importProviderAccount() account_uuid = %q, want expected uuid", item.AccountUUID)
	}
	if item.AccessToken != "VVV" {
		t.Fatalf("importProviderAccount() access_token = %q, want %q", item.AccessToken, "VVV")
	}
	if item.RefreshToken != "KKK" {
		t.Fatalf("importProviderAccount() refresh_token = %q, want %q", item.RefreshToken, "KKK")
	}
	expectedCredentials := `{"token_type":"Bearer","expires_in":28800,"scope":"user:file_upload","organization":{"uuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","name":"pqEandrewaverypXe9@publicist.com's Organization"},"account":{"uuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","email_address":"pqEandrewaverypXe9@publicist.com"}}`
	if item.Credentials != expectedCredentials {
		t.Fatalf("importProviderAccount() credentials = %q, want %q", item.Credentials, expectedCredentials)
	}
}

func TestImportProviderAccountUpdatesExistingRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database")
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	claudePath := filepath.Join(t.TempDir(), "claude.json")
	claudeContent := `{"oauthAccount":{"accountUuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","emailAddress":"pqEandrewaverypXe9@publicist.com","organizationUuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","organizationName":"pqEandrewaverypXe9@publicist.com's Organization"}}`
	first := `{"claudeAiOauth":{"accessToken":"VVV","refreshToken":"KKK","scopes":["user:file_upload"]}}`
	if err := os.WriteFile(credentialsPath, []byte(first), 0o600); err != nil {
		t.Fatalf("write first credentials file error = %v", err)
	}
	if err := os.WriteFile(claudePath, []byte(claudeContent), 0o600); err != nil {
		t.Fatalf("write claude file error = %v", err)
	}

	item1, err := importProviderAccount(credentialsPath, claudePath, &dbPath)
	if err != nil {
		t.Fatalf("first importProviderAccount() error = %v", err)
	}

	second := `{"claudeAiOauth":{"accessToken":"VVV2","refreshToken":"KKK2","scopes":["user:file_upload","user:inference"]}}`
	if err := os.WriteFile(credentialsPath, []byte(second), 0o600); err != nil {
		t.Fatalf("write second credentials file error = %v", err)
	}

	item2, err := importProviderAccount(credentialsPath, claudePath, &dbPath)
	if err != nil {
		t.Fatalf("second importProviderAccount() error = %v", err)
	}

	if item2.ID != item1.ID {
		t.Fatalf("updated provider account id = %d, want %d", item2.ID, item1.ID)
	}
	if item2.AccessToken != "VVV2" {
		t.Fatalf("updated access_token = %q, want %q", item2.AccessToken, "VVV2")
	}
	if item2.RefreshToken != "KKK2" {
		t.Fatalf("updated refresh_token = %q, want %q", item2.RefreshToken, "KKK2")
	}
	expectedCredentials := `{"token_type":"Bearer","expires_in":28800,"scope":"user:file_upload user:inference","organization":{"uuid":"c34ac4d7-5ac9-4923-bc78-2f3012183105","name":"pqEandrewaverypXe9@publicist.com's Organization"},"account":{"uuid":"5c2f5d10-0d29-4bf8-b073-e620af17980d","email_address":"pqEandrewaverypXe9@publicist.com"}}`
	if item2.Credentials != expectedCredentials {
		t.Fatalf("updated credentials = %q, want %q", item2.Credentials, expectedCredentials)
	}
}
