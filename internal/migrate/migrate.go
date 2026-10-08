// Package migrate applies the embedded schema migrations.
//
// Only the worker calls this. The API must never migrate: several API instances
// starting at once would race for the migration lock for no benefit.
package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/desmondhiew00/jma-weather-api/migrations"
)

// Up applies all pending migrations. It is a no-op when the schema is current.
func Up(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		_, _ = srcErr, dbErr
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

// trimScheme strips a postgres:// or postgresql:// prefix so the caller can use
// one DATABASE_URL for both pgx and golang-migrate.
func trimScheme(url string) string {
	for _, p := range []string{"postgresql://", "postgres://", "pgx5://", "pgx://"} {
		if len(url) >= len(p) && url[:len(p)] == p {
			return url[len(p):]
		}
	}

	return url
}

var _ = pgx.ErrNilConfig // keep the pgx5 migrate driver linked

// Down rolls back a single migration. Development convenience only; nothing in
// production calls it.
func Down(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		_, _ = srcErr, dbErr
	}()

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("roll back: %w", err)
	}

	return nil
}
