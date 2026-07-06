// Package worker runs the relay's periodic loops: one goroutine per role
// (poller / coin-scanner / submitter / confirmer / recorder), each with its
// own ticker, panic recovery, and graceful shutdown (spec §7.5).
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// Tick is one unit of a worker's periodic job. Errors are logged, never fatal.
type Tick func(ctx context.Context) error

// Worker wraps a named Tick with its interval.
type Worker struct {
	Name     string
	Interval time.Duration
	Tick     Tick
}

// Runner owns the worker goroutines.
type Runner struct {
	log     *slog.Logger
	workers []Worker
}

func NewRunner(log *slog.Logger, workers ...Worker) *Runner {
	return &Runner{log: log, workers: workers}
}

// Run blocks until ctx is cancelled and every worker has drained.
func (r *Runner) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, w := range r.workers {
		wg.Add(1)
		go func(w Worker) {
			defer wg.Done()
			r.runWorker(ctx, w)
		}(w)
	}
	wg.Wait()
}

func (r *Runner) runWorker(ctx context.Context, w Worker) {
	log := r.log.With("worker", w.Name)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	// Immediate first tick — do not wait a full interval on boot.
	r.safeTick(ctx, w, log)
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return
		case <-ticker.C:
			r.safeTick(ctx, w, log)
		}
	}
}

// safeTick runs one tick, converting panics into logged errors so a bad tick
// can never take the process down (spec §7.5).
func (r *Runner) safeTick(ctx context.Context, w Worker, log *slog.Logger) {
	if ctx.Err() != nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			log.Error("worker tick panicked", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
		}
	}()
	if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
		log.Error("worker tick failed", "err", err)
	}
}
