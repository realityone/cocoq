package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	defaultDirName  = ".cocoq"
	defaultFileName = "config.yaml"
	defaultAddr     = "127.0.0.1:8888"
	defaultDatabase = "database.db"

	DefaultCACertFile = "ca.crt"
	DefaultCAKeyFile  = "ca.key"
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
	RootDir string   `mapstructure:"-" yaml:"-"`
	Addr    string   `mapstructure:"addr" yaml:"addr"`
	HARFile string   `mapstructure:"har_file" yaml:"har_file"`
	Verbose bool     `mapstructure:"verbose" yaml:"verbose"`
	CA      CAConfig `mapstructure:"ca" yaml:"ca"`
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
	rootDir := defaultRootDir()
	return Config{
		Global: GlobalConfig{
			RootDir: rootDir,
		},
		Server: ServerConfig{
			RootDir: rootDir,
			Addr:    defaultAddr,
			CA: CAConfig{
				CertFile: DefaultCACertFile,
				KeyFile:  DefaultCAKeyFile,
			},
		},
		Database: DatabaseConfig{
			RootDir: rootDir,
			Path:    defaultDatabase,
		},
	}
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

func defaultRootDir() string {
	rootDir, err := DefaultRootDir()
	if err != nil {
		return ""
	}
	return rootDir
}

func FilePath(rootDir, path string) string {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "file:") {
		return path
	}
	return filepath.Join(rootDir, path)
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
	if err := v.Unmarshal(&cfg); err != nil {
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
	v.SetDefault("server.har_file", cfg.Server.HARFile)
	v.SetDefault("server.verbose", cfg.Server.Verbose)
	v.SetDefault("database.path", cfg.Database.Path)
	v.SetDefault("server.ca.cert_file", cfg.Server.CA.CertFile)
	v.SetDefault("server.ca.key_file", cfg.Server.CA.KeyFile)
}

func applyDerivedFields(cfg *Config) {
	if cfg.Global.RootDir == "" {
		cfg.Global.RootDir = defaultRootDir()
	}
	cfg.Server.RootDir = cfg.Global.RootDir
	cfg.Database.RootDir = cfg.Global.RootDir
}
