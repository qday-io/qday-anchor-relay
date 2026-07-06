// Package store is the Postgres persistence layer: anchor_jobs state machine,
// relay_coins, poller_cursor, and job_events audit trail (spec §6).
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Job mirrors one anchor_jobs row.
type Job struct {
	ID            int64
	BatchNumber   uint64
	StateRoot     [32]byte
	DABlockNumber uint64
	DATxHash      string
	AbelTxHash    string
	AbelHeight    int64
	RecordTxHash  string
	Status        Status
	RetryCount    int
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// JobPatch carries optional column updates applied atomically with a status
// transition. Nil fields are left untouched.
type JobPatch struct {
	AbelTxHash   *string
	AbelHeight   *int64
	RecordTxHash *string
	LastError    *string
	// Detail is stored on the job_events row (JSONB) for the portal timeline.
	Detail map[string]any
}

// Coin mirrors one relay_coins row.
type Coin struct {
	CoinID        string // "txid:index"
	TxHash        string
	OutIndex      int16
	ValueNeutrino int64
	BlockHeight   int64
	RingID        string
	RingIndex     int16
	Status        CoinStatus
	SpentByJob    *int64
}

type CoinStatus string

const (
	CoinUnspent CoinStatus = "UNSPENT"
	CoinLocked  CoinStatus = "LOCKED"
	CoinSpent   CoinStatus = "SPENT"
)
