package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }

var testLog = slog.New(slog.NewTextHandler(nullWriter{}, nil))

func TestRunner_TicksAndStops(t *testing.T) {
	var ticks atomic.Int64
	r := NewRunner(testLog, Worker{
		Name:     "counter",
		Interval: 10 * time.Millisecond,
		Tick: func(ctx context.Context) error {
			ticks.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after ctx cancel")
	}
	if n := ticks.Load(); n < 2 {
		t.Errorf("ticks = %d, want ≥ 2 (immediate + periodic)", n)
	}
}

func TestRunner_PanicDoesNotKillWorker(t *testing.T) {
	var ticks atomic.Int64
	r := NewRunner(testLog, Worker{
		Name:     "panicky",
		Interval: 5 * time.Millisecond,
		Tick: func(ctx context.Context) error {
			if ticks.Add(1) == 1 {
				panic("boom")
			}
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	r.Run(ctx)

	if n := ticks.Load(); n < 2 {
		t.Errorf("ticks = %d — worker died after panic", n)
	}
}

func TestRunner_ErrorsAreNotFatal(t *testing.T) {
	var ticks atomic.Int64
	r := NewRunner(testLog, Worker{
		Name:     "flaky",
		Interval: 5 * time.Millisecond,
		Tick: func(ctx context.Context) error {
			ticks.Add(1)
			return errors.New("transient")
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	r.Run(ctx)
	if n := ticks.Load(); n < 2 {
		t.Errorf("ticks = %d — worker stopped on error", n)
	}
}

func TestRunner_MultipleWorkersIndependent(t *testing.T) {
	var a, b atomic.Int64
	r := NewRunner(testLog,
		Worker{Name: "a", Interval: 5 * time.Millisecond, Tick: func(ctx context.Context) error { a.Add(1); return nil }},
		Worker{Name: "b", Interval: 5 * time.Millisecond, Tick: func(ctx context.Context) error { b.Add(1); return nil }},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	r.Run(ctx)
	if a.Load() < 2 || b.Load() < 2 {
		t.Errorf("a=%d b=%d", a.Load(), b.Load())
	}
}
