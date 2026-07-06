package store

// Integration tests for the Postgres repository. They run only when
// TEST_DATABASE_URL points at a database that already has the migrations
// applied (see Makefile target test-db / README). Example:
//
//	TEST_DATABASE_URL=postgres://anchor:pw@localhost:5433/anchor?sslmode=disable go test ./internal/store/

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(),
			`TRUNCATE job_events, relay_coins, anchor_jobs RESTART IDENTITY CASCADE`)
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM poller_cursor`)
		s.Close()
	})
	_, _ = s.pool.Exec(context.Background(),
		`TRUNCATE job_events, relay_coins, anchor_jobs RESTART IDENTITY CASCADE`)
	_, _ = s.pool.Exec(context.Background(), `DELETE FROM poller_cursor`)
	return s
}

func testCommit(batch uint64) Commit {
	var root [32]byte
	copy(root[:], fmt.Sprintf("root-%d", batch))
	return Commit{BatchNumber: batch, StateRoot: root, DABlockNumber: batch * 10, DATxHash: fmt.Sprintf("0xda%d", batch)}
}

func TestCreateJobsAndAdvanceCursor_Idempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	commits := []Commit{testCommit(1), testCommit(2)}
	if err := s.CreateJobsAndAdvanceCursor(ctx, commits, 100); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Rescan of the same range must be a no-op for jobs but advance cursor.
	if err := s.CreateJobsAndAdvanceCursor(ctx, commits, 120); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	jobs, err := s.JobsByStatus(ctx, StatusPending, 10)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("jobs = %d (%v), want 2", len(jobs), err)
	}
	cur, err := s.Cursor(ctx, 0)
	if err != nil || cur != 120 {
		t.Fatalf("cursor = %d (%v), want 120", cur, err)
	}
}

func TestCursor_DefaultOnFirstBoot(t *testing.T) {
	s := testStore(t)
	cur, err := s.Cursor(context.Background(), 42)
	if err != nil || cur != 42 {
		t.Fatalf("cursor = %d (%v), want default 42", cur, err)
	}
}

func TestTransitionJob_HappyPathWritesEvent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.CreateJobsAndAdvanceCursor(ctx, []Commit{testCommit(7)}, 70); err != nil {
		t.Fatal(err)
	}
	jobs, _ := s.JobsByStatus(ctx, StatusPending, 1)
	job := jobs[0]

	txHash := "abeltx123"
	err := s.TransitionJob(ctx, job.ID, StatusPending, StatusAbelSubmitted,
		JobPatch{AbelTxHash: &txHash, Detail: map[string]any{"memo": "QDAY2ANCHOR|v1|7|0x…"}})
	if err != nil {
		t.Fatalf("transition: %v", err)
	}

	got, err := s.JobByID(ctx, job.ID)
	if err != nil || got.Status != StatusAbelSubmitted || got.AbelTxHash != "abeltx123" {
		t.Fatalf("job after transition = %+v (%v)", got, err)
	}

	var n int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM job_events WHERE job_id = $1 AND to_status = 'ABEL_SUBMITTED'`, job.ID).Scan(&n)
	if n != 1 {
		t.Errorf("job_events rows = %d, want 1", n)
	}
}

func TestTransitionJob_StaleAndIllegal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_ = s.CreateJobsAndAdvanceCursor(ctx, []Commit{testCommit(8)}, 80)
	jobs, _ := s.JobsByStatus(ctx, StatusPending, 1)
	job := jobs[0]

	if err := s.TransitionJob(ctx, job.ID, StatusAbelSubmitted, StatusAbelConfirmed, JobPatch{}); !errors.Is(err, ErrStaleTransition) {
		t.Errorf("stale transition error = %v, want ErrStaleTransition", err)
	}
	if err := s.TransitionJob(ctx, job.ID, StatusPending, StatusDone, JobPatch{}); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("illegal transition error = %v, want ErrIllegalTransition", err)
	}
}

func TestFailOrRetry_ExhaustsToFailed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_ = s.CreateJobsAndAdvanceCursor(ctx, []Commit{testCommit(9)}, 90)
	jobs, _ := s.JobsByStatus(ctx, StatusPending, 1)
	job := jobs[0]

	retryMax := 2
	for i := 0; i < retryMax; i++ {
		if err := s.FailOrRetry(ctx, job, retryMax, errors.New("boom")); err != nil {
			t.Fatalf("FailOrRetry #%d: %v", i, err)
		}
		got, _ := s.JobByID(ctx, job.ID)
		if got.Status != StatusPending {
			t.Fatalf("after retry %d status = %s, want PENDING", i, got.Status)
		}
	}
	// One past the limit → FAILED.
	if err := s.FailOrRetry(ctx, job, retryMax, errors.New("boom final")); err != nil {
		t.Fatal(err)
	}
	got, _ := s.JobByID(ctx, job.ID)
	if got.Status != StatusFailed || got.RetryCount != retryMax+1 || got.LastError != "boom final" {
		t.Fatalf("after exhaustion: %+v", got)
	}
}

func TestCoinLockSpendRelease(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_ = s.CreateJobsAndAdvanceCursor(ctx, []Commit{testCommit(10)}, 100)
	jobs, _ := s.JobsByStatus(ctx, StatusPending, 1)
	jobID := jobs[0].ID

	for i, v := range []int64{500, 2_000_000, 5_000_000} {
		c := &Coin{CoinID: fmt.Sprintf("tx%d:0", i), TxHash: fmt.Sprintf("tx%d", i),
			OutIndex: 0, ValueNeutrino: v, BlockHeight: int64(100 + i), Status: CoinUnspent}
		if err := s.UpsertCoin(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	// Smallest sufficient coin picked: 2_000_000 (500 is below fee).
	coin, err := s.SelectAndLockCoin(ctx, jobID, 1_000_000)
	if err != nil || coin.ValueNeutrino != 2_000_000 || coin.Status != CoinLocked {
		t.Fatalf("SelectAndLockCoin = %+v (%v)", coin, err)
	}

	locked, err := s.CoinsLockedByJob(ctx, jobID)
	if err != nil || len(locked) != 1 || locked[0].CoinID != coin.CoinID {
		t.Fatalf("CoinsLockedByJob = %+v (%v)", locked, err)
	}

	if err := s.MarkCoinSpent(ctx, coin.CoinID, jobID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.CoinByID(ctx, coin.CoinID)
	if got.Status != CoinSpent {
		t.Fatalf("coin status = %s, want SPENT", got.Status)
	}

	// Upsert rescan must NOT resurrect a spent coin.
	if err := s.UpsertCoin(ctx, &Coin{CoinID: coin.CoinID, TxHash: coin.TxHash,
		OutIndex: 0, ValueNeutrino: coin.ValueNeutrino, BlockHeight: coin.BlockHeight, Status: CoinUnspent}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CoinByID(ctx, coin.CoinID)
	if got.Status != CoinSpent {
		t.Fatalf("coin status after rescan = %s, want SPENT (no resurrection)", got.Status)
	}

	// Lock then release returns coin to pool.
	coin2, err := s.SelectAndLockCoin(ctx, jobID, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseCoin(ctx, coin2.CoinID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CoinByID(ctx, coin2.CoinID)
	if got.Status != CoinUnspent {
		t.Fatalf("released coin status = %s, want UNSPENT", got.Status)
	}

	// No coin large enough → ErrNoSpendableCoin.
	if _, err := s.SelectAndLockCoin(ctx, jobID, 100_000_000); !errors.Is(err, ErrNoSpendableCoin) {
		t.Errorf("err = %v, want ErrNoSpendableCoin", err)
	}
}
