package store

import (
	"os"
	"testing"
)

// TestMigrate_EmbeddedIdempotent runs the embedded migrations against the
// integration database: on an already-migrated DB (CLI-applied) it must
// no-op; a second run must also no-op.
func TestMigrate_EmbeddedIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate #1: %v", err)
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate #2 (idempotency): %v", err)
	}
}
