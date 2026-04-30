package main

import (
	"fmt"
	"os"

	"cocoq/server"
	"cocoq/server/database/dbcmd"

	"github.com/sirupsen/logrus"
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

var serverAddr string
var databasePath string
var serverHARFile string
var serverVerbose bool

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server commands",
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database commands",
}

var serverRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the HTTP proxy server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := logrus.New()

		srv, err := server.New(server.Config{
			Addr:    serverAddr,
			DBPath:  databasePath,
			HARFile: serverHARFile,
			Verbose: serverVerbose,
			Logger:  logger,
		})
		if err != nil {
			return err
		}

		return srv.Run()
	},
}

func init() {
	serverRunCmd.Flags().StringVar(&serverAddr, "addr", "127.0.0.1:8888", "HTTP listen address for proxy server")
	serverRunCmd.Flags().StringVar(&databasePath, "db-path", "", "database path, defaults to ~/.cocoq/database")
	serverRunCmd.Flags().StringVar(&serverHARFile, "har-file", "", "write accepted proxy sessions to this HAR file")
	serverRunCmd.Flags().BoolVar(&serverVerbose, "verbose", false, "enable verbose proxy logging")
	dbCmd.PersistentFlags().StringVar(&databasePath, "db-path", "", "database path, defaults to ~/.cocoq/database")
	serverCmd.AddCommand(serverRunCmd)
	dbcmd.Register(dbCmd, &databasePath)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(dbCmd)
}
