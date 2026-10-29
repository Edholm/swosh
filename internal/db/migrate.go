package db

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies all pending migrations to the database at databaseURL. It
// does nothing if the schema is already up to date.
func Migrate(databaseURL string) (err error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		srcErr, dbErr := m.Close()
		err = errors.Join(err, srcErr, dbErr)
	}()

	from, err := schemaVersion(m)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	to, err := schemaVersion(m)
	if err != nil {
		return err
	}

	if from == to {
		slog.Info("database schema is up to date", slog.Uint64("version", uint64(to)))
	} else {
		slog.Info("applied database migrations", slog.Uint64("from", uint64(from)), slog.Uint64("to", uint64(to)))
	}
	return nil
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}
	u, err := migrateURL(databaseURL)
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, u)
	if err != nil {
		return nil, fmt.Errorf("connect migrator: %w", err)
	}
	return m, nil
}

// migrateURL turns a postgres:// URL into the pgx5:// form golang-migrate
// uses to select its pgx driver.
func migrateURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		// url.Error repeats the input, which may contain a password.
		return "", errors.New("parse database URL: invalid URL")
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		u.Scheme = "pgx5"
		return u.String(), nil
	default:
		return "", fmt.Errorf("parse database URL: unsupported scheme %q, want postgres://", u.Scheme)
	}
}

// schemaVersion returns the applied migration version, or 0 for an empty
// database.
func schemaVersion(m *migrate.Migrate) (uint, error) {
	v, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("read schema version: %w", err)
	case dirty:
		return 0, fmt.Errorf("schema version %d is dirty: a migration failed half-way and must be fixed by hand", v)
	}
	return v, nil
}
