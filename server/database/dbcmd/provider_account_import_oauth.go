package dbcmd

import (
	"fmt"
	"os"

	"cocoq/server/database/dbrt"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

func newProviderAccountImportOAuthCmd(databasePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "import-oauth <path>",
		Short: "Import a provider account from an OAuth token response JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := importProviderAccountOAuth(args[0], databasePath)
			if err != nil {
				return err
			}
			printProviderAccounts(cmd, []*dbrt.ProviderAccount{item})
			return nil
		},
	}
}

func importProviderAccountOAuth(path string, databasePath *string) (*dbrt.ProviderAccount, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read oauth file: %w", err)
	}

	accountUUID, payload, err := parseProviderAccountOAuth(content)
	if err != nil {
		return nil, err
	}

	return upsertProviderAccount(databasePath, accountUUID, payload, string(content))
}

func parseProviderAccountOAuth(content []byte) (string, providerAccountImportPayload, error) {
	if !gjson.ValidBytes(content) {
		return "", providerAccountImportPayload{}, fmt.Errorf("parse oauth file: invalid JSON")
	}

	accountUUID := gjson.GetBytes(content, "account.uuid").String()
	if accountUUID == "" {
		return "", providerAccountImportPayload{}, fmt.Errorf("missing account.uuid")
	}

	accessToken := gjson.GetBytes(content, "access_token").String()
	if accessToken == "" {
		return "", providerAccountImportPayload{}, fmt.Errorf("missing access_token")
	}

	refreshToken := gjson.GetBytes(content, "refresh_token").String()
	if refreshToken == "" {
		return "", providerAccountImportPayload{}, fmt.Errorf("missing refresh_token")
	}

	return accountUUID, providerAccountImportPayload{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
