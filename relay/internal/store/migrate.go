package store

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/qday/qday-anchor-relay/relay/migrations"
)

// Migrate applies all pending embedded migrations. Idempotent: a fully
// migrated database is a no-op, so it is safe to run on every startup (and
// concurrent runners are serialized by golang-migrate's advisory lock).
func Migrate(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("store: load embedded migrations: %w", err)
	}
	// golang-migrate's pgx v5 driver registers the "pgx5" URL scheme.
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(dsn))
	if err != nil {
		return fmt.Errorf("store: init migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}

// trimScheme strips a leading postgres:// or postgresql:// from the DSN so it
// can be re-prefixed with the migrate driver scheme.
func trimScheme(dsn string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if len(dsn) > len(p) && dsn[:len(p)] == p {
			return dsn[len(p):]
		}
	}
	return dsn
}
