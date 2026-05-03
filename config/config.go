package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	defaultDirName            = ".cocoq"
	defaultFileName           = "config.yaml"
	defaultAddr               = "127.0.0.1:8888"
	defaultDatabase           = "database.db"
	defaultCACertFile         = "ca.crt"
	defaultCAKeyFile          = "ca.key"
	defaultOpenRouterProvider = "anthropic"
	defaultAPIService         = APIServiceOpenRouter
)

const (
	APIServiceAnthropic  = "anthropic"
	APIServiceOpenRouter = "openrouter"
	APIServicePoe        = "poe"
)

type Config struct {
	Global   GlobalConfig   `mapstructure:"global" yaml:"global"`
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Database DatabaseConfig `mapstructure:"database" yaml:"database"`
}

type GlobalConfig struct {
	RootDir string `mapstructure:"root_dir" yaml:"root_dir"`
}

type ServerConfig struct {
	RootDir     string             `mapstructure:"-" yaml:"-"`
	Addr        string             `mapstructure:"addr" yaml:"addr"`
	APIServices []APIServiceConfig `mapstructure:"api_services" yaml:"api_services"`
	HARFile     string             `mapstructure:"har_file" yaml:"har_file"`
	Verbose     bool               `mapstructure:"verbose" yaml:"verbose"`
	CA          CAConfig           `mapstructure:"ca" yaml:"ca"`
}

type APIServiceConfig struct {
	Name    string          `mapstructure:"name" yaml:"name"`
	Options json.RawMessage `mapstructure:"options" yaml:"options,omitempty"`
}

type DatabaseConfig struct {
	RootDir string `mapstructure:"-" yaml:"-"`
	Path    string `mapstructure:"path" yaml:"path"`
}

type CAConfig struct {
	CertFile string `mapstructure:"cert_file" yaml:"cert_file"`
	KeyFile  string `mapstructure:"key_file" yaml:"key_file"`
}

func Default() Config {
	rootDir, _ := DefaultRootDir()
	return Config{
		Global: GlobalConfig{
			RootDir: rootDir,
		},
		Server: ServerConfig{
			RootDir:     rootDir,
			Addr:        defaultAddr,
			APIServices: defaultAPIServices(),
			CA: CAConfig{
				CertFile: defaultCACertFile,
				KeyFile:  defaultCAKeyFile,
			},
		},
		Database: DatabaseConfig{
			RootDir: rootDir,
			Path:    defaultDatabase,
		},
	}
}

func defaultAPIServices() []APIServiceConfig {
	options := struct {
		Provider string `json:"provider"`
	}{
		Provider: defaultOpenRouterProvider,
	}
	raw, _ := json.Marshal(options)
	return []APIServiceConfig{
		{
			Name:    APIServiceOpenRouter,
			Options: json.RawMessage(raw),
		},
	}
}

func DefaultYAML() string {
	cfg := Default()
	defaultService := cfg.Server.APIServices[0]
	return fmt.Sprintf(`# Default cocoq configuration.
# Global settings shared by all commands.
global:
  # Root directory for runtime files. Relative file paths below are resolved under this directory.
  root_dir: %s

# Proxy server settings.
server:
  # HTTP listen address for the local proxy server.
  addr: %s
  # API services to install. Supported names: "openrouter", "anthropic", "poe".
  api_services:
    - name: %s
      # Service-specific options. OpenRouter supports "provider".
      options:
        provider: %s
  # HAR output file path. Empty disables HAR export.
  har_file: %s
  # Enable verbose proxy logging.
  verbose: %t
  # Root CA files used for MITM TLS.
  ca:
    # Root CA certificate file. Absolute paths are used directly.
    cert_file: %s
    # Root CA private key file. Absolute paths are used directly.
    key_file: %s

# Database settings.
database:
  # SQLite database file path. Absolute paths are used directly.
  path: %s
`, yamlString(cfg.Global.RootDir),
		yamlString(cfg.Server.Addr),
		yamlString(defaultService.Name),
		yamlString(defaultOpenRouterProvider),
		yamlString(cfg.Server.HARFile),
		cfg.Server.Verbose,
		yamlString(cfg.Server.CA.CertFile),
		yamlString(cfg.Server.CA.KeyFile),
		yamlString(cfg.Database.Path),
	)
}

func DefaultPath() (string, error) {
	rootDir, err := DefaultRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootDir, defaultFileName), nil
}

func DefaultRootDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", pkgerrors.Wrap(err, "resolve home directory")
	}
	return filepath.Join(homeDir, defaultDirName), nil
}

func FilePath(rootDir, path string) string {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "file:") {
		return path
	}
	return filepath.Join(rootDir, path)
}

func yamlString(s string) string {
	return strconv.Quote(s)
}

func Load(path string) (Config, error) {
	v, configPath, err := newViper(path)
	if err != nil {
		return Config{}, err
	}

	err = v.ReadInConfig()
	var notFound viper.ConfigFileNotFoundError
	if err != nil {
		if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
			return Config{}, pkgerrors.Wrapf(err, "read config file %q", configPath)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(rawMessageDecodeHook())); err != nil {
		return Config{}, pkgerrors.Wrapf(err, "parse config file %q", configPath)
	}
	applyDerivedFields(&cfg)
	return cfg, nil
}

func newViper(path string) (*viper.Viper, string, error) {
	if path == "" {
		defaultPath, err := DefaultPath()
		if err != nil {
			return nil, "", err
		}
		path = defaultPath
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	setDefaults(v)
	return v, path, nil
}

func setDefaults(v *viper.Viper) {
	cfg := Default()
	v.SetDefault("global.root_dir", cfg.Global.RootDir)
	v.SetDefault("server.addr", cfg.Server.Addr)
	v.SetDefault("server.api_services", cfg.Server.APIServices)
	v.SetDefault("server.har_file", cfg.Server.HARFile)
	v.SetDefault("server.verbose", cfg.Server.Verbose)
	v.SetDefault("database.path", cfg.Database.Path)
	v.SetDefault("server.ca.cert_file", cfg.Server.CA.CertFile)
	v.SetDefault("server.ca.key_file", cfg.Server.CA.KeyFile)
}

func rawMessageDecodeHook() mapstructure.DecodeHookFunc {
	rawMessageType := reflect.TypeOf(json.RawMessage{})
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if to != rawMessageType {
			return data, nil
		}
		switch value := data.(type) {
		case nil:
			return nil, nil
		case json.RawMessage:
			return value, nil
		case []byte:
			return json.RawMessage(value), nil
		case string:
			if value == "" {
				return json.RawMessage(nil), nil
			}
			if json.Valid([]byte(value)) {
				return json.RawMessage(value), nil
			}
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	}
}

func applyDerivedFields(cfg *Config) {
	if cfg.Global.RootDir == "" {
		cfg.Global.RootDir, _ = DefaultRootDir()
	}
	cfg.Server.RootDir = cfg.Global.RootDir
	cfg.Database.RootDir = cfg.Global.RootDir
}
