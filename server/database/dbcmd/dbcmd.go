package dbcmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"cocoq/server/database"
	"cocoq/server/database/dbrt"

	"github.com/spf13/cobra"
)

func Register(dbCmd *cobra.Command, databasePath *string) {
	dbCmd.AddCommand(
		newInitCmd(databasePath),
		newUserDBCmd(databasePath),
		newFakeTokenDBCmd(databasePath),
		newProviderAccountDBCmd(databasePath),
		newUserGrantedProviderAccountDBCmd(databasePath),
	)
}

func newInitCmd(databasePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Init a root user and print its oauth_state_token",
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := initRootUser(context.Background(), *databasePath)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "USER ID\tOAUTH STATE TOKEN")
			fmt.Fprintf(w, "%d\t%s\n", user.ID, user.OauthStateToken)
			return w.Flush()
		},
	}
}

func newUserDBCmd(databasePath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user",
		Aliases: []string{"users"},
		Short:   "CRUD commands for users",
	}
	cmd.AddCommand(
		newDBListCmd("list users", func(cmd *cobra.Command) error { return runUserList(cmd, databasePath) }),
		newDBGetCmd("get user", func(cmd *cobra.Command, id int64) error { return runUserGet(cmd, databasePath, id) }),
		newDBDeleteCmd("delete user", func(cmd *cobra.Command, id int64) error { return runUserDelete(cmd, databasePath, id) }),
		newDBCreateCmd("create user", func(cmd *cobra.Command, fields map[string]string) error {
			return runUserCreate(cmd, databasePath, fields)
		}),
		newDBUpdateCmd("update user", func(cmd *cobra.Command, id int64, fields map[string]string) error {
			return runUserUpdate(cmd, databasePath, id, fields)
		}),
	)
	return cmd
}

func newFakeTokenDBCmd(databasePath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fake-token",
		Aliases: []string{"fake_token", "faketoken"},
		Short:   "CRUD commands for fake tokens",
	}
	cmd.AddCommand(
		newDBListCmd("list fake tokens", func(cmd *cobra.Command) error { return runFakeTokenList(cmd, databasePath) }),
		newDBGetCmd("get fake token", func(cmd *cobra.Command, id int64) error { return runFakeTokenGet(cmd, databasePath, id) }),
		newDBDeleteCmd("delete fake token", func(cmd *cobra.Command, id int64) error { return runFakeTokenDelete(cmd, databasePath, id) }),
		newDBCreateCmd("create fake token", func(cmd *cobra.Command, fields map[string]string) error {
			return runFakeTokenCreate(cmd, databasePath, fields)
		}),
		newDBUpdateCmd("update fake token", func(cmd *cobra.Command, id int64, fields map[string]string) error {
			return runFakeTokenUpdate(cmd, databasePath, id, fields)
		}),
	)
	return cmd
}

func newProviderAccountDBCmd(databasePath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "provider-account",
		Aliases: []string{"provider_account", "provideraccount"},
		Short:   "CRUD commands for provider accounts",
	}
	cmd.AddCommand(
		newDBListCmd("list provider accounts", func(cmd *cobra.Command) error { return runProviderAccountList(cmd, databasePath) }),
		newDBGetCmd("get provider account", func(cmd *cobra.Command, id int64) error { return runProviderAccountGet(cmd, databasePath, id) }),
		newDBDeleteCmd("delete provider account", func(cmd *cobra.Command, id int64) error { return runProviderAccountDelete(cmd, databasePath, id) }),
		newProviderAccountImportCmd(databasePath),
		newProviderAccountImportOAuthCmd(databasePath),
		newDBCreateCmd("create provider account", func(cmd *cobra.Command, fields map[string]string) error {
			return runProviderAccountCreate(cmd, databasePath, fields)
		}),
		newDBUpdateCmd("update provider account", func(cmd *cobra.Command, id int64, fields map[string]string) error {
			return runProviderAccountUpdate(cmd, databasePath, id, fields)
		}),
	)
	return cmd
}

func newUserGrantedProviderAccountDBCmd(databasePath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user-granted-provider-account",
		Aliases: []string{"user_granted_provider_account", "ugpa"},
		Short:   "CRUD commands for user granted provider accounts",
	}
	cmd.AddCommand(
		newDBListCmd("list user granted provider accounts", func(cmd *cobra.Command) error { return runUGPAList(cmd, databasePath) }),
		newDBGetCmd("get user granted provider account", func(cmd *cobra.Command, id int64) error { return runUGPAGet(cmd, databasePath, id) }),
		newDBDeleteCmd("delete user granted provider account", func(cmd *cobra.Command, id int64) error { return runUGPADelete(cmd, databasePath, id) }),
		newDBCreateCmd("create user granted provider account", func(cmd *cobra.Command, fields map[string]string) error {
			return runUGPACreate(cmd, databasePath, fields)
		}),
		newDBUpdateCmd("update user granted provider account", func(cmd *cobra.Command, id int64, fields map[string]string) error {
			return runUGPAUpdate(cmd, databasePath, id, fields)
		}),
	)
	return cmd
}

func newDBListCmd(short string, run func(cmd *cobra.Command) error) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd)
		},
	}
}

func newDBGetCmd(short string, run func(cmd *cobra.Command, id int64) error) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIDArg(args[0])
			if err != nil {
				return err
			}
			return run(cmd, id)
		},
	}
}

func newDBDeleteCmd(short string, run func(cmd *cobra.Command, id int64) error) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIDArg(args[0])
			if err != nil {
				return err
			}
			return run(cmd, id)
		},
	}
}

func newDBCreateCmd(short string, run func(cmd *cobra.Command, fields map[string]string) error) *cobra.Command {
	var fieldArgs []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := parseFieldAssignments(fieldArgs)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field key=value is required")
			}
			return run(cmd, fields)
		},
	}
	cmd.Flags().StringArrayVar(&fieldArgs, "field", nil, "field assignment in key=value form")
	return cmd
}

func newDBUpdateCmd(short string, run func(cmd *cobra.Command, id int64, fields map[string]string) error) *cobra.Command {
	var fieldArgs []string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIDArg(args[0])
			if err != nil {
				return err
			}
			fields, err := parseFieldAssignments(fieldArgs)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field key=value is required")
			}
			return run(cmd, id, fields)
		},
	}
	cmd.Flags().StringArrayVar(&fieldArgs, "field", nil, "field assignment in key=value form")
	return cmd
}

func runUserList(cmd *cobra.Command, databasePath *string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		users, err := client.User.Query().All(ctx)
		if err != nil {
			return err
		}
		printUsers(cmd, users)
		return nil
	})
}

func runUserGet(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		user, err := client.User.Get(ctx, id)
		if err != nil {
			return err
		}
		printUsers(cmd, []*dbrt.User{user})
		return nil
	})
}

func runUserDelete(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		if err := client.User.DeleteOneID(id).Exec(ctx); err != nil {
			return err
		}
		printDeleteResult(cmd, "user", id)
		return nil
	})
}

func runUserCreate(cmd *cobra.Command, databasePath *string, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.User.Create()
		if err := applyUserCreateFields(builder, fields); err != nil {
			return err
		}
		user, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printUsers(cmd, []*dbrt.User{user})
		return nil
	})
}

func runUserUpdate(cmd *cobra.Command, databasePath *string, id int64, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.User.UpdateOneID(id)
		if err := applyUserUpdateFields(builder, fields); err != nil {
			return err
		}
		user, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printUsers(cmd, []*dbrt.User{user})
		return nil
	})
}

func runFakeTokenList(cmd *cobra.Command, databasePath *string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		items, err := client.FakeToken.Query().All(ctx)
		if err != nil {
			return err
		}
		printFakeTokens(cmd, items)
		return nil
	})
}

func runFakeTokenGet(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		item, err := client.FakeToken.Get(ctx, id)
		if err != nil {
			return err
		}
		printFakeTokens(cmd, []*dbrt.FakeToken{item})
		return nil
	})
}

func runFakeTokenDelete(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		if err := client.FakeToken.DeleteOneID(id).Exec(ctx); err != nil {
			return err
		}
		printDeleteResult(cmd, "fake_token", id)
		return nil
	})
}

func runFakeTokenCreate(cmd *cobra.Command, databasePath *string, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.FakeToken.Create()
		if err := applyFakeTokenCreateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printFakeTokens(cmd, []*dbrt.FakeToken{item})
		return nil
	})
}

func runFakeTokenUpdate(cmd *cobra.Command, databasePath *string, id int64, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.FakeToken.UpdateOneID(id)
		if err := applyFakeTokenUpdateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printFakeTokens(cmd, []*dbrt.FakeToken{item})
		return nil
	})
}

func runProviderAccountList(cmd *cobra.Command, databasePath *string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		items, err := client.ProviderAccount.Query().All(ctx)
		if err != nil {
			return err
		}
		printProviderAccounts(cmd, items)
		return nil
	})
}

func runProviderAccountGet(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		item, err := client.ProviderAccount.Get(ctx, id)
		if err != nil {
			return err
		}
		printProviderAccounts(cmd, []*dbrt.ProviderAccount{item})
		return nil
	})
}

func runProviderAccountDelete(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		if err := client.ProviderAccount.DeleteOneID(id).Exec(ctx); err != nil {
			return err
		}
		printDeleteResult(cmd, "provider_account", id)
		return nil
	})
}

func runProviderAccountCreate(cmd *cobra.Command, databasePath *string, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.ProviderAccount.Create()
		if err := applyProviderAccountCreateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printProviderAccounts(cmd, []*dbrt.ProviderAccount{item})
		return nil
	})
}

func runProviderAccountUpdate(cmd *cobra.Command, databasePath *string, id int64, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.ProviderAccount.UpdateOneID(id)
		if err := applyProviderAccountUpdateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printProviderAccounts(cmd, []*dbrt.ProviderAccount{item})
		return nil
	})
}

func runUGPAList(cmd *cobra.Command, databasePath *string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		items, err := client.UserGrantedProviderAccount.Query().All(ctx)
		if err != nil {
			return err
		}
		printUGPAs(cmd, items)
		return nil
	})
}

func runUGPAGet(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		item, err := client.UserGrantedProviderAccount.Get(ctx, id)
		if err != nil {
			return err
		}
		printUGPAs(cmd, []*dbrt.UserGrantedProviderAccount{item})
		return nil
	})
}

func runUGPADelete(cmd *cobra.Command, databasePath *string, id int64) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		if err := client.UserGrantedProviderAccount.DeleteOneID(id).Exec(ctx); err != nil {
			return err
		}
		printDeleteResult(cmd, "user_granted_provider_account", id)
		return nil
	})
}

func runUGPACreate(cmd *cobra.Command, databasePath *string, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.UserGrantedProviderAccount.Create()
		if err := applyUGPACreateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printUGPAs(cmd, []*dbrt.UserGrantedProviderAccount{item})
		return nil
	})
}

func runUGPAUpdate(cmd *cobra.Command, databasePath *string, id int64, fields map[string]string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		builder := client.UserGrantedProviderAccount.UpdateOneID(id)
		if err := applyUGPAUpdateFields(builder, fields); err != nil {
			return err
		}
		item, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		printUGPAs(cmd, []*dbrt.UserGrantedProviderAccount{item})
		return nil
	})
}

func withClient(databasePath *string, run func(context.Context, *dbrt.Client) error) error {
	client, err := database.OpenClient(*databasePath)
	if err != nil {
		return err
	}
	defer client.Close()
	return run(context.Background(), client)
}

func parseFieldAssignments(assignments []string) (map[string]string, error) {
	fields := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		key, value, ok := strings.Cut(assignment, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --field %q, want key=value", assignment)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid --field %q, empty key", assignment)
		}
		fields[key] = value
	}
	return fields, nil
}

func parseIDArg(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", arg, err)
	}
	return id, nil
}

func parseInt64Field(name, raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, raw, err)
	}
	return value, nil
}

func applyUserCreateFields(builder *dbrt.UserCreate, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetID(id)
		case "oauth_state_token":
			builder.SetOauthStateToken(value)
		case "state":
			state, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetState(state)
		default:
			return fmt.Errorf("unknown user field %q", key)
		}
	}
	return nil
}

func applyUserUpdateFields(builder *dbrt.UserUpdateOne, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "oauth_state_token":
			builder.SetOauthStateToken(value)
		case "state":
			state, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetState(state)
		default:
			return fmt.Errorf("unknown user field %q", key)
		}
	}
	return nil
}

func applyFakeTokenCreateFields(builder *dbrt.FakeTokenCreate, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetID(id)
		case "user_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetUserID(id)
		case "provider_account_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetProviderAccountID(id)
		case "access_token":
			builder.SetAccessToken(value)
		case "refresh_token":
			builder.SetRefreshToken(value)
		default:
			return fmt.Errorf("unknown fake_token field %q", key)
		}
	}
	return nil
}

func applyFakeTokenUpdateFields(builder *dbrt.FakeTokenUpdateOne, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "user_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetUserID(id)
		case "provider_account_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetProviderAccountID(id)
		case "access_token":
			builder.SetAccessToken(value)
		case "refresh_token":
			builder.SetRefreshToken(value)
		default:
			return fmt.Errorf("unknown fake_token field %q", key)
		}
	}
	return nil
}

func applyProviderAccountCreateFields(builder *dbrt.ProviderAccountCreate, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetID(id)
		case "account_uuid":
			builder.SetAccountUUID(value)
		case "access_token":
			builder.SetAccessToken(value)
		case "refresh_token":
			builder.SetRefreshToken(value)
		case "credentials":
			builder.SetCredentials(value)
		default:
			return fmt.Errorf("unknown provider_account field %q", key)
		}
	}
	return nil
}

func applyProviderAccountUpdateFields(builder *dbrt.ProviderAccountUpdateOne, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "account_uuid":
			builder.SetAccountUUID(value)
		case "access_token":
			builder.SetAccessToken(value)
		case "refresh_token":
			builder.SetRefreshToken(value)
		case "credentials":
			builder.SetCredentials(value)
		default:
			return fmt.Errorf("unknown provider_account field %q", key)
		}
	}
	return nil
}

func applyUGPACreateFields(builder *dbrt.UserGrantedProviderAccountCreate, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetID(id)
		case "user_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetUserID(id)
		case "provider_account_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetProviderAccountID(id)
		default:
			return fmt.Errorf("unknown user_granted_provider_account field %q", key)
		}
	}
	return nil
}

func applyUGPAUpdateFields(builder *dbrt.UserGrantedProviderAccountUpdateOne, fields map[string]string) error {
	for key, value := range fields {
		switch key {
		case "user_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetUserID(id)
		case "provider_account_id":
			id, err := parseInt64Field(key, value)
			if err != nil {
				return err
			}
			builder.SetProviderAccountID(id)
		default:
			return fmt.Errorf("unknown user_granted_provider_account field %q", key)
		}
	}
	return nil
}

func printUsers(cmd *cobra.Command, users []*dbrt.User) {
	rows := make([][]string, 0, len(users))
	for _, user := range users {
		rows = append(rows, []string{
			strconv.FormatInt(user.ID, 10),
			user.OauthStateToken,
			strconv.FormatInt(user.State, 10),
			user.CreatedAt.Format("2006-01-02 15:04:05"),
			user.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	printTable(cmd, []string{"ID", "OAUTH STATE TOKEN", "STATE", "CREATED AT", "UPDATED AT"}, rows)
}

func printFakeTokens(cmd *cobra.Command, items []*dbrt.FakeToken) {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			strconv.FormatInt(item.ID, 10),
			strconv.FormatInt(item.UserID, 10),
			strconv.FormatInt(item.ProviderAccountID, 10),
			item.AccessToken,
			item.RefreshToken,
		})
	}
	printTable(cmd, []string{"ID", "USER ID", "PROVIDER ACCOUNT ID", "ACCESS TOKEN", "REFRESH TOKEN"}, rows)
}

func printProviderAccounts(cmd *cobra.Command, items []*dbrt.ProviderAccount) {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			strconv.FormatInt(item.ID, 10),
			item.AccountUUID,
			item.AccessToken,
			item.RefreshToken,
			item.Credentials,
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	printTable(cmd, []string{"ID", "ACCOUNT UUID", "ACCESS TOKEN", "REFRESH TOKEN", "CREDENTIALS", "CREATED AT", "UPDATED AT"}, rows)
}

func printUGPAs(cmd *cobra.Command, items []*dbrt.UserGrantedProviderAccount) {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			strconv.FormatInt(item.ID, 10),
			strconv.FormatInt(item.UserID, 10),
			strconv.FormatInt(item.ProviderAccountID, 10),
		})
	}
	printTable(cmd, []string{"ID", "USER ID", "PROVIDER ACCOUNT ID"}, rows)
}

func printDeleteResult(cmd *cobra.Command, entity string, id int64) {
	printTable(cmd, []string{"ENTITY", "ID", "STATUS"}, [][]string{{entity, strconv.FormatInt(id, 10), "deleted"}})
}

func printTable(cmd *cobra.Command, headers []string, rows [][]string) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}
