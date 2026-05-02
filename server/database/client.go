package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	appconfig "github.com/realityone/cocoq/config"
	"github.com/realityone/cocoq/server/database/dbrt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

const defaultDatabaseFile = "database.db"

func defaultDatabasePath() (string, error) {
	rootDir, err := appconfig.DefaultRootDir()
	if err != nil {
		return "", err
	}
	return appconfig.FilePath(rootDir, defaultDatabaseFile), nil
}

func resolveDatabasePath(cfg appconfig.DatabaseConfig) (string, error) {
	rootDir := cfg.RootDir
	if rootDir == "" {
		var err error
		rootDir, err = appconfig.DefaultRootDir()
		if err != nil {
			return "", err
		}
	}
	path := cfg.Path
	if path == "" {
		path = defaultDatabaseFile
	}
	return appconfig.FilePath(rootDir, path), nil
}

func OpenClient(cfg appconfig.DatabaseConfig) (*dbrt.Client, error) {
	path, err := resolveDatabasePath(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, errors.Wrap(err, "create database directory")
	}

	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, errors.Wrap(err, "open sqlite database")
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, errors.Wrap(err, "enable sqlite foreign keys")
	}
	client := dbrt.NewClient(dbrt.Driver(entsql.OpenDB(dialect.SQLite, db)))

	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, errors.Wrap(err, "create database schema")
	}

	return client, nil
}

func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") {
		if strings.Contains(path, "?") {
			return path + "&_fk=1"
		}
		return path + "?_fk=1"
	}
	return "file:" + path + "?_fk=1"
}
