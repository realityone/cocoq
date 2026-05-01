package dbcmd

import (
	"context"
	"fmt"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/realityone/cocoq/server/database"
	"github.com/realityone/cocoq/server/database/dbrt"

	"github.com/spf13/cobra"
)

func Register(dbCmd *cobra.Command, databasePath *string) {
	dbCmd.AddCommand(newAnthropicUsageDBCmd(databasePath))
}

func newAnthropicUsageDBCmd(databasePath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "anthropic-usage",
		Aliases: []string{"anthropic_usage", "anthropicusage"},
		Short:   "Commands for Anthropic usage records",
	}
	cmd.AddCommand(
		newDBListCmd("list Anthropic usage records", func(cmd *cobra.Command) error {
			return runAnthropicUsageList(cmd, databasePath)
		}),
		newDBGetCmd("get Anthropic usage record", func(cmd *cobra.Command, id int) error {
			return runAnthropicUsageGet(cmd, databasePath, id)
		}),
		newDBDeleteCmd("delete Anthropic usage record", func(cmd *cobra.Command, id int) error {
			return runAnthropicUsageDelete(cmd, databasePath, id)
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

func newDBGetCmd(short string, run func(cmd *cobra.Command, id int) error) *cobra.Command {
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

func newDBDeleteCmd(short string, run func(cmd *cobra.Command, id int) error) *cobra.Command {
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

func runAnthropicUsageList(cmd *cobra.Command, databasePath *string) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		records, err := client.AnthropicUsage.Query().All(ctx)
		if err != nil {
			return err
		}
		printAnthropicUsages(cmd, records)
		return nil
	})
}

func runAnthropicUsageGet(cmd *cobra.Command, databasePath *string, id int) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		record, err := client.AnthropicUsage.Get(ctx, id)
		if err != nil {
			return err
		}
		printAnthropicUsages(cmd, []*dbrt.AnthropicUsage{record})
		return nil
	})
}

func runAnthropicUsageDelete(cmd *cobra.Command, databasePath *string, id int) error {
	return withClient(databasePath, func(ctx context.Context, client *dbrt.Client) error {
		return client.AnthropicUsage.DeleteOneID(id).Exec(ctx)
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

func parseIDArg(value string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", value, err)
	}
	return id, nil
}

func printAnthropicUsages(cmd *cobra.Command, records []*dbrt.AnthropicUsage) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDEVICE ID\tSESSION ID\tACCOUNT UUID\tINPUT TOKENS\tCACHE READ INPUT TOKENS\tCACHE CREATION INPUT TOKENS\tOUTPUT TOKENS\tCACHE CREATION 5M INPUT TOKENS\tCACHE CREATION 1H INPUT TOKENS\tCACHE HIT RATE\tRAW\tCREATED AT\tUPDATED AT")
	for _, record := range records {
		fmt.Fprintf(
			w,
			"%d\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%g\t%s\t%s\t%s\n",
			record.ID,
			record.DeviceID,
			record.SessionID,
			record.AccountUUID,
			record.InputTokens,
			record.CacheReadInputTokens,
			record.CacheCreationInputTokens,
			record.OutputTokens,
			record.CacheCreationEphemeral5mInputTokens,
			record.CacheCreationEphemeral1hInputTokens,
			record.CacheHitRate,
			record.Raw,
			record.CreatedAt.Format(time.RFC3339Nano),
			record.UpdatedAt.Format(time.RFC3339Nano),
		)
	}
	_ = w.Flush()
}
