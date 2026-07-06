package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Commit is the poller-facing shape of a new batch commit to persist.
// (Mirrors da.Commit; redeclared here to keep store free of the da package.)
type Commit struct {
	BatchNumber   uint64
	StateRoot     [32]byte
	DABlockNumber uint64
	DATxHash      string
}

// Cursor returns the last scanned DA block, or (defaultStart, nil) when the
// cursor row does not exist yet (first boot).
func (s *Store) Cursor(ctx context.Context, defaultStart uint64) (uint64, error) {
	var last int64
	err := s.pool.QueryRow(ctx,
		`SELECT last_scanned_block FROM poller_cursor WHERE id = 1`).Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultStart, nil
	}
	if err != nil {
		return 0, err
	}
	return uint64(last), nil
}

// CreateJobsAndAdvanceCursor inserts the scanned commits (ON CONFLICT DO
// NOTHING — idempotent under rescans) and advances the poller cursor, all in
// ONE transaction (spec §7.1: crash between the two must be impossible).
func (s *Store) CreateJobsAndAdvanceCursor(ctx context.Context, commits []Commit, newCursor uint64) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, c := range commits {
			if _, err := tx.Exec(ctx, `
				INSERT INTO anchor_jobs (batch_number, state_root, da_block_number, da_tx_hash)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING`,
				c.BatchNumber, c.StateRoot[:], c.DABlockNumber, c.DATxHash); err != nil {
				return fmt.Errorf("store: insert job batch=%d: %w", c.BatchNumber, err)
			}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO poller_cursor (id, last_scanned_block) VALUES (1, $1)
			ON CONFLICT (id) DO UPDATE SET last_scanned_block = EXCLUDED.last_scanned_block`,
			newCursor)
		return err
	})
}
