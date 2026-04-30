package dbcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cocoq/server/database"
	"cocoq/server/database/dbrt"
	"cocoq/server/database/dbrt/provideraccount"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type providerAccountImportPayload struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func newProviderAccountImportCmd(databasePath *string) *cobra.Command {
	var credentialsPath string
	var claudePath string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a provider account from credentials and Claude JSON files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if credentialsPath == "" {
				return fmt.Errorf("--credentials is required")
			}
			if claudePath == "" {
				return fmt.Errorf("--claude is required")
			}
			return runProviderAccountImport(cmd, databasePath, credentialsPath, claudePath)
		},
	}
	cmd.Flags().StringVar(&credentialsPath, "credentials", "", "path to the credentials JSON file")
	cmd.Flags().StringVar(&claudePath, "claude", "", "path to the Claude account JSON file")
	return cmd
}

func runProviderAccountImport(cmd *cobra.Command, databasePath *string, credentialsPath, claudePath string) error {
	item, err := importProviderAccount(credentialsPath, claudePath, databasePath)
	if err != nil {
		return err
	}
	printProviderAccounts(cmd, []*dbrt.ProviderAccount{item})
	return nil
}

func importProviderAccount(credentialsPath, claudePath string, databasePath *string) (*dbrt.ProviderAccount, error) {
	credentialsContent, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}
	claudeContent, err := os.ReadFile(claudePath)
	if err != nil {
		return nil, fmt.Errorf("read claude file: %w", err)
	}

	accountUUID, err := parseProviderAccountClaude(claudeContent)
	if err != nil {
		return nil, err
	}
	payload, err := parseProviderAccountCredentials(credentialsContent)
	if err != nil {
		return nil, err
	}
	constructedCredentials, err := constructProviderAccountCredentials(credentialsContent, claudeContent)
	if err != nil {
		return nil, err
	}

	return upsertProviderAccount(databasePath, accountUUID, payload, constructedCredentials)
}

func upsertProviderAccount(databasePath *string, accountUUID string, payload providerAccountImportPayload, credentials string) (*dbrt.ProviderAccount, error) {
	return withClientResult(databasePath, func(ctx context.Context, client *dbrt.Client) (*dbrt.ProviderAccount, error) {
		existing, err := client.ProviderAccount.Query().
			Where(provideraccount.AccountUUID(accountUUID)).
			Only(ctx)
		switch {
		case err == nil:
			return client.ProviderAccount.UpdateOneID(existing.ID).
				SetAccessToken(payload.AccessToken).
				SetRefreshToken(payload.RefreshToken).
				SetCredentials(credentials).
				Save(ctx)
		case dbrt.IsNotFound(err):
			return client.ProviderAccount.Create().
				SetAccountUUID(accountUUID).
				SetAccessToken(payload.AccessToken).
				SetRefreshToken(payload.RefreshToken).
				SetCredentials(credentials).
				Save(ctx)
		default:
			return nil, err
		}
	})
}

func parseProviderAccountCredentials(content []byte) (providerAccountImportPayload, error) {
	if !gjson.ValidBytes(content) {
		return providerAccountImportPayload{}, fmt.Errorf("parse credentials file: invalid JSON")
	}

	accessToken := gjson.GetBytes(content, "claudeAiOauth.accessToken").String()
	if accessToken == "" {
		return providerAccountImportPayload{}, fmt.Errorf("missing claudeAiOauth.accessToken")
	}
	refreshToken := gjson.GetBytes(content, "claudeAiOauth.refreshToken").String()
	if refreshToken == "" {
		return providerAccountImportPayload{}, fmt.Errorf("missing claudeAiOauth.refreshToken")
	}

	return providerAccountImportPayload{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func parseProviderAccountClaude(content []byte) (string, error) {
	if !gjson.ValidBytes(content) {
		return "", fmt.Errorf("parse claude file: invalid JSON")
	}

	accountUUID := gjson.GetBytes(content, "oauthAccount.accountUuid").String()
	if accountUUID == "" {
		return "", fmt.Errorf("missing oauthAccount.accountUuid")
	}
	return accountUUID, nil
}

func constructProviderAccountCredentials(credentialsContent, claudeContent []byte) (string, error) {
	if !gjson.ValidBytes(credentialsContent) {
		return "", fmt.Errorf("parse credentials file: invalid JSON")
	}
	if !gjson.ValidBytes(claudeContent) {
		return "", fmt.Errorf("parse claude file: invalid JSON")
	}

	scopesValue := gjson.GetBytes(credentialsContent, "claudeAiOauth.scopes")
	if !scopesValue.Exists() || !scopesValue.IsArray() {
		return "", fmt.Errorf("missing claudeAiOauth.scopes")
	}
	scopes := make([]string, 0, len(scopesValue.Array()))
	for _, scope := range scopesValue.Array() {
		value := scope.String()
		if value == "" {
			return "", fmt.Errorf("claudeAiOauth.scopes contains empty value")
		}
		scopes = append(scopes, value)
	}

	organizationUUID := gjson.GetBytes(claudeContent, "oauthAccount.organizationUuid").String()
	if organizationUUID == "" {
		return "", fmt.Errorf("missing oauthAccount.organizationUuid")
	}
	organizationName := gjson.GetBytes(claudeContent, "oauthAccount.organizationName").String()
	if organizationName == "" {
		return "", fmt.Errorf("missing oauthAccount.organizationName")
	}
	accountUUID := gjson.GetBytes(claudeContent, "oauthAccount.accountUuid").String()
	if accountUUID == "" {
		return "", fmt.Errorf("missing oauthAccount.accountUuid")
	}
	emailAddress := gjson.GetBytes(claudeContent, "oauthAccount.emailAddress").String()
	if emailAddress == "" {
		return "", fmt.Errorf("missing oauthAccount.emailAddress")
	}

	var err error
	result := []byte(`{}`)
	result, err = sjson.SetBytes(result, "token_type", "Bearer")
	if err != nil {
		return "", fmt.Errorf("set token_type: %w", err)
	}
	result, err = sjson.SetBytes(result, "expires_in", 28800)
	if err != nil {
		return "", fmt.Errorf("set expires_in: %w", err)
	}
	result, err = sjson.SetBytes(result, "scope", strings.Join(scopes, " "))
	if err != nil {
		return "", fmt.Errorf("set scope: %w", err)
	}
	result, err = sjson.SetBytes(result, "organization.uuid", organizationUUID)
	if err != nil {
		return "", fmt.Errorf("set organization.uuid: %w", err)
	}
	result, err = sjson.SetBytes(result, "organization.name", organizationName)
	if err != nil {
		return "", fmt.Errorf("set organization.name: %w", err)
	}
	result, err = sjson.SetBytes(result, "account.uuid", accountUUID)
	if err != nil {
		return "", fmt.Errorf("set account.uuid: %w", err)
	}
	result, err = sjson.SetBytes(result, "account.email_address", emailAddress)
	if err != nil {
		return "", fmt.Errorf("set account.email_address: %w", err)
	}

	return string(result), nil
}

func withClientResult[T any](databasePath *string, run func(context.Context, *dbrt.Client) (T, error)) (T, error) {
	client, err := database.OpenClient(*databasePath)
	if err != nil {
		var zero T
		return zero, err
	}
	defer client.Close()
	return run(context.Background(), client)
}
