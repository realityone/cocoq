package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	appconfig "github.com/realityone/cocoq/config"

	"github.com/spf13/cobra"
)

type ConfigLoader func() (appconfig.Config, error)

type commandRunner func(name string, args []string, env []string) error

var runCommand commandRunner = func(name string, args []string, env []string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	argv := append([]string{name}, args...)
	return syscall.Exec(path, argv, env)
}

func NewClaudeCmd(loadConfig ConfigLoader) *cobra.Command {
	return newClaudeCmd(loadConfig)
}

func newClaudeCmd(loadConfig ConfigLoader) *cobra.Command {
	return &cobra.Command{
		Use:                "claude [args...]",
		Short:              "Run Claude through the cocoq proxy",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			env, caCertPath := claudeEnv(cfg)
			if _, err := os.Stat(caCertPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("NODE_EXTRA_CA_CERTS file does not exist: %s", caCertPath)
				}
				return fmt.Errorf("stat NODE_EXTRA_CA_CERTS file %q: %w", caCertPath, err)
			}

			return runCommand("claude", args, env)
		},
	}
}

func claudeEnv(cfg appconfig.Config) ([]string, string) {
	rootDir := cfg.Server.RootDir
	if rootDir == "" {
		rootDir = cfg.Global.RootDir
	}
	caCertPath := appconfig.FilePath(rootDir, cfg.Server.CA.CertFile)
	env := withEnv(os.Environ(), map[string]string{
		"HTTP_PROXY":          proxyURL(cfg.Server.Addr),
		"NODE_EXTRA_CA_CERTS": caCertPath,
	})
	return env, caCertPath
}

func proxyURL(addr string) string {
	if strings.Contains(addr, "://") {
		return addr
	}
	return "http://" + addr
}

func withEnv(env []string, updates map[string]string) []string {
	result := make([]string, 0, len(env)+len(updates))
	seen := make(map[string]bool, len(updates))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			result = append(result, entry)
			continue
		}
		value, replace := updates[key]
		if !replace {
			result = append(result, entry)
			continue
		}
		result = append(result, key+"="+value)
		seen[key] = true
	}
	for key, value := range updates {
		if !seen[key] {
			result = append(result, key+"="+value)
		}
	}
	return result
}
