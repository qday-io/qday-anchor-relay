package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrStaleTransition means the job was not in the expected `from` status when
// the transition ran — another worker advanced it first. Callers should reload.
var ErrStaleTransition = errors.New("store: job not in expected status (stale transition)")

// ErrIllegalTransition means the requested move violates the state machine.
var ErrIllegalTransition = errors.New("store: illegal state transition")

const jobColumns = `id, batch_number, state_root, da_block_number,
	COALESCE(da_tx_hash, ''), COALESCE(abel_tx_hash, ''), COALESCE(abel_height, 0),
	COALESCE(record_tx_hash, ''), status, retry_count, COALESCE(last_error, ''),
	created_at, updated_at`

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	var rootBytes []byte
	var statusRaw string
	if err := row.Scan(&j.ID, &j.BatchNumber, &rootBytes, &j.DABlockNumber,
		&j.DATxHash, &j.AbelTxHash, &j.AbelHeight, &j.RecordTxHash,
		&statusRaw, &j.RetryCount, &j.LastError, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, err
	}
	if len(rootBytes) != 32 {
		return nil, fmt.Errorf("store: job %d state_root is %d bytes, want 32", j.ID, len(rootBytes))
	}
	copy(j.StateRoot[:], rootBytes)
	status, err := ParseStatus(statusRaw)
	if err != nil {
		return nil, err
	}
	j.Status = status
	return &j, nil
}

// JobsByStatus returns jobs in the given status, oldest first.
func (s *Store) JobsByStatus(ctx context.Context, status Status, limit int) ([]*Job, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+jobColumns+` FROM anchor_jobs WHERE status = $1 ORDER BY batch_number ASC LIMIT $2`,
		string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// JobByID loads a single job.
func (s *Store) JobByID(ctx context.Context, id int64) (*Job, error) {
	return scanJob(s.pool.QueryRow(ctx,
		`SELECT `+jobColumns+` FROM anchor_jobs WHERE id = $1`, id))
}

// TransitionJob atomically moves a job from → to, applies the patch, and
// appends a job_events row. Fails with ErrIllegalTransition if the move is
// not in the state machine, ErrStaleTransition if the row is no longer in
// `from` (idempotency under concurrent workers / restarts).
func (s *Store) TransitionJob(ctx context.Context, jobID int64, from, to Status, patch JobPatch) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s → %s", ErrIllegalTransition, from, to)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE anchor_jobs SET
				status = $1,
				abel_tx_hash   = COALESCE($2, abel_tx_hash),
				abel_height    = COALESCE($3, abel_height),
				record_tx_hash = COALESCE($4, record_tx_hash),
				-- last_error is cleared on every transition unless the patch
				-- sets one; failure history lives in job_events.
				last_error     = $5,
				updated_at     = now()
			WHERE id = $6 AND status = $7`,
			string(to), patch.AbelTxHash, patch.AbelHeight, patch.RecordTxHash,
			patch.LastError, jobID, string(from))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: job %d, %s → %s", ErrStaleTransition, jobID, from, to)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO job_events (job_id, from_status, to_status, detail)
			VALUES ($1, $2, $3, $4)`,
			jobID, string(from), string(to), patch.Detail)
		return err
	})
}

// FailOrRetry bumps retry_count and records the error. When retry_count
// exceeds retryMax the job transitions to FAILED (with a job_events row);
// otherwise it stays in place for the next tick to retry.
func (s *Store) FailOrRetry(ctx context.Context, job *Job, retryMax int, cause error) error {
	msg := cause.Error()
	if len(msg) > 4000 {
		msg = msg[:4000]
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var retryCount int
		err := tx.QueryRow(ctx, `
			UPDATE anchor_jobs
			SET retry_count = retry_count + 1, last_error = $1, updated_at = now()
			WHERE id = $2 AND status = $3
			RETURNING retry_count`,
			msg, job.ID, string(job.Status)).Scan(&retryCount)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: job %d", ErrStaleTransition, job.ID)
		}
		if err != nil {
			return err
		}
		if retryCount <= retryMax {
			return nil
		}
		tag, err := tx.Exec(ctx, `
			UPDATE anchor_jobs SET status = $1, updated_at = now()
			WHERE id = $2 AND status = $3`,
			string(StatusFailed), job.ID, string(job.Status))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: job %d", ErrStaleTransition, job.ID)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO job_events (job_id, from_status, to_status, detail)
			VALUES ($1, $2, $3, $4)`,
			job.ID, string(job.Status), string(StatusFailed),
			map[string]any{"error": msg, "retry_count": retryCount})
		return err
	})
}

// SetJobAbelTxHash persists the locally computed Abelian txid BEFORE broadcast
// (idempotency rule 6.1-2: a crash between broadcast and the status transition
// must not lose the hash, or the next tick would double-send).
func (s *Store) SetJobAbelTxHash(ctx context.Context, jobID int64, txHash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE anchor_jobs SET abel_tx_hash = $1, updated_at = now() WHERE id = $2`,
		txHash, jobID)
	return err
}

// RetryCandidates returns non-terminal jobs whose next retry is due
// (updated_at + backoff(retry_count) <= now), oldest first.
func (s *Store) RetryCandidates(ctx context.Context, status Status, baseSeconds int, limit int) ([]*Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+jobColumns+` FROM anchor_jobs
		WHERE status = $1
		  AND (retry_count = 0 OR
		       updated_at + make_interval(secs => LEAST($2 * power(2, retry_count), 1800)) <= now())
		ORDER BY batch_number ASC LIMIT $3`,
		string(status), baseSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
