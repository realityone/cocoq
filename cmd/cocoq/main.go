package main

import (
	"fmt"
	"os"

	"github.com/realityone/cocoq/server"

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
var serverHARFile string
var serverVerbose bool

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server commands",
}

var serverRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the HTTP proxy server",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := logrus.New()

		srv, err := server.New(server.Config{
			Addr:    serverAddr,
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
	serverRunCmd.Flags().StringVar(&serverHARFile, "har-file", "", "write accepted proxy sessions to this HAR file")
	serverRunCmd.Flags().BoolVar(&serverVerbose, "verbose", false, "enable verbose proxy logging")
	serverCmd.AddCommand(serverRunCmd)
	rootCmd.AddCommand(serverCmd)
}
