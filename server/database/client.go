package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"cocoq/server/database/dbrt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

const defaultDatabaseDir = ".cocoq"
const defaultDatabaseFile = "database"

func defaultDatabasePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "resolve home directory")
	}
	return filepath.Join(homeDir, defaultDatabaseDir, defaultDatabaseFile), nil
}

func OpenClient(path string) (*dbrt.Client, error) {
	var err error
	if path == "" {
		path, err = defaultDatabasePath()
		if err != nil {
			return nil, err
		}
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
