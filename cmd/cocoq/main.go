package main

import (
	"fmt"
	"os"

	appconfig "github.com/realityone/cocoq/config"
	"github.com/realityone/cocoq/server"
	"github.com/realityone/cocoq/server/database"
	"github.com/realityone/cocoq/server/database/dbcmd"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cocoq",
	Short: "cocoq root command",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var configPath string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server commands",
}

var dbCmd = newDBCmd()
var defaultConfigCmd = newDefaultConfigCmd()

var serverRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the HTTP proxy server",
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		db, err := database.OpenClient(cfg.Database)
		if err != nil {
			return err
		}
		defer db.Close()

		srv, err := server.New(cfg.Server, db)
		if err != nil {
			return err
		}

		return srv.Run()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default $HOME/.cocoq/config.yaml)")

	serverCmd.AddCommand(serverRunCmd)

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(defaultConfigCmd)
}

func loadConfig() (appconfig.Config, error) {
	return appconfig.Load(configPath)
}

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "db",
		Aliases: []string{"database"},
		Short:   "Database commands",
	}
	dbcmd.Register(cmd, func() (appconfig.DatabaseConfig, error) {
		cfg, err := loadConfig()
		if err != nil {
			return appconfig.DatabaseConfig{}, err
		}
		return cfg.Database, nil
	})
	return cmd
}

func newDefaultConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default-config",
		Short: "Print the default config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), appconfig.DefaultYAML())
			return err
		},
	}
}
