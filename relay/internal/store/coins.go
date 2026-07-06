package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNoSpendableCoin means no UNSPENT coin with sufficient value exists.
var ErrNoSpendableCoin = errors.New("store: no spendable coin with sufficient value")

const coinColumns = `coin_id, tx_hash, out_index, value_neutrino, block_height,
	COALESCE(ring_id, ''), COALESCE(ring_index, 0), COALESCE(serial_number, ''), status, spent_by_job`

func scanCoin(row pgx.Row) (*Coin, error) {
	var c Coin
	var statusRaw string
	if err := row.Scan(&c.CoinID, &c.TxHash, &c.OutIndex, &c.ValueNeutrino,
		&c.BlockHeight, &c.RingID, &c.RingIndex, &c.SerialNumber, &statusRaw, &c.SpentByJob); err != nil {
		return nil, err
	}
	c.Status = CoinStatus(statusRaw)
	return &c, nil
}

// UpsertCoin records a discovered coin. Idempotent: rescanning the same
// blocks must not duplicate or reset state (spent coins stay spent).
func (s *Store) UpsertCoin(ctx context.Context, c *Coin) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO relay_coins (coin_id, tx_hash, out_index, value_neutrino, block_height, ring_id, ring_index, status)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8)
		ON CONFLICT (coin_id) DO UPDATE SET
			ring_id    = COALESCE(NULLIF(EXCLUDED.ring_id, ''), relay_coins.ring_id),
			ring_index = CASE WHEN NULLIF(EXCLUDED.ring_id, '') IS NOT NULL THEN EXCLUDED.ring_index ELSE relay_coins.ring_index END,
			updated_at = now()`,
		c.CoinID, c.TxHash, c.OutIndex, c.ValueNeutrino, c.BlockHeight,
		c.RingID, c.RingIndex, string(c.Status))
	return err
}

// SelectAndLockCoin picks the smallest UNSPENT coin with value >= minValue and
// marks it LOCKED for the job, atomically (FOR UPDATE SKIP LOCKED so parallel
// submitters never double-spend a coin).
func (s *Store) SelectAndLockCoin(ctx context.Context, jobID int64, minValue int64) (*Coin, error) {
	var coin *Coin
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT `+coinColumns+` FROM relay_coins
			WHERE status = 'UNSPENT' AND value_neutrino >= $1
			ORDER BY value_neutrino ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED`, minValue)
		c, err := scanCoin(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoSpendableCoin
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE relay_coins SET status = 'LOCKED', spent_by_job = $1, updated_at = now()
			WHERE coin_id = $2`, jobID, c.CoinID)
		if err != nil {
			return err
		}
		c.Status = CoinLocked
		c.SpentByJob = &jobID
		coin = c
		return nil
	})
	return coin, err
}

// CoinsLockedByJob returns coins currently LOCKED for a job (crash recovery:
// a restarted submitter re-uses or releases them).
func (s *Store) CoinsLockedByJob(ctx context.Context, jobID int64) ([]*Coin, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+coinColumns+` FROM relay_coins WHERE status = 'LOCKED' AND spent_by_job = $1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var coins []*Coin
	for rows.Next() {
		c, err := scanCoin(rows)
		if err != nil {
			return nil, err
		}
		coins = append(coins, c)
	}
	return coins, rows.Err()
}

// MarkCoinSpent finalizes a LOCKED coin after its spending tx was broadcast.
func (s *Store) MarkCoinSpent(ctx context.Context, coinID string, jobID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relay_coins SET status = 'SPENT', spent_by_job = $1, updated_at = now()
		WHERE coin_id = $2`, jobID, coinID)
	return err
}

// ReleaseCoin returns a LOCKED coin to UNSPENT (submit failed before broadcast).
func (s *Store) ReleaseCoin(ctx context.Context, coinID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relay_coins SET status = 'UNSPENT', spent_by_job = NULL, updated_at = now()
		WHERE coin_id = $1 AND status = 'LOCKED'`, coinID)
	return err
}

// CoinByID loads one coin, or nil when unknown.
func (s *Store) CoinByID(ctx context.Context, coinID string) (*Coin, error) {
	c, err := scanCoin(s.pool.QueryRow(ctx,
		`SELECT `+coinColumns+` FROM relay_coins WHERE coin_id = $1`, coinID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// CoinsMissingSerial returns tracked coins that do not have their serial
// number yet (ring group was incomplete when they were discovered).
func (s *Store) CoinsMissingSerial(ctx context.Context, upToHeight int64) ([]*Coin, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+coinColumns+` FROM relay_coins
		WHERE serial_number IS NULL AND block_height <= $1
		ORDER BY block_height ASC`, upToHeight)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var coins []*Coin
	for rows.Next() {
		c, err := scanCoin(rows)
		if err != nil {
			return nil, err
		}
		coins = append(coins, c)
	}
	return coins, rows.Err()
}

// SetCoinSerial backfills a coin's serial number and ring placement.
func (s *Store) SetCoinSerial(ctx context.Context, coinID, serialHex, ringID string, ringIndex int16) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relay_coins SET serial_number = $1, ring_id = $2, ring_index = $3, updated_at = now()
		WHERE coin_id = $4`, serialHex, ringID, ringIndex, coinID)
	return err
}

// MarkCoinSpentBySerial flags the coin carrying this serial number as SPENT —
// scanner-side spend detection from chain tx inputs. Returns whether a coin
// matched.
func (s *Store) MarkCoinSpentBySerial(ctx context.Context, serialHex string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE relay_coins SET status = 'SPENT', updated_at = now()
		WHERE serial_number = $1 AND status <> 'SPENT'`, serialHex)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ResurrectCoinsSpentByJob returns a job's SPENT/LOCKED coins to UNSPENT —
// used when the job's Abelian tx is declared lost (the coins were never
// actually consumed on chain).
func (s *Store) ResurrectCoinsSpentByJob(ctx context.Context, jobID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relay_coins SET status = 'UNSPENT', spent_by_job = NULL, updated_at = now()
		WHERE spent_by_job = $1 AND status IN ('SPENT', 'LOCKED')`, jobID)
	return err
}
