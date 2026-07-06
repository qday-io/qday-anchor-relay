package da

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type fakeSource struct {
	latest  uint64
	commits map[uint64][]Commit // keyed by `from` block of the expected range
	calls   [][2]uint64
	err     error
}

func (f *fakeSource) LatestBlock(ctx context.Context) (uint64, error) {
	return f.latest, nil
}

func (f *fakeSource) FetchCommits(ctx context.Context, from, to uint64) ([]Commit, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls = append(f.calls, [2]uint64{from, to})
	return f.commits[from], nil
}

type fakeSink struct {
	cursor     uint64
	hasCursor  bool
	commits    []Commit
	persists   int
	failStores bool
}

func (f *fakeSink) Cursor(ctx context.Context, defaultStart uint64) (uint64, error) {
	if !f.hasCursor {
		return defaultStart, nil
	}
	return f.cursor, nil
}

func (f *fakeSink) CreateJobsAndAdvanceCursor(ctx context.Context, commits []Commit, newCursor uint64) error {
	if f.failStores {
		return errors.New("db down")
	}
	f.commits = append(f.commits, commits...)
	f.cursor = newCursor
	f.hasCursor = true
	f.persists++
	return nil
}

var testLog = slog.New(slog.NewTextHandler(discard{}, nil))

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func mkCommit(batch, block uint64) Commit {
	var root [32]byte
	root[0] = byte(batch)
	return Commit{BatchNumber: batch, StateRoot: root, DABlock: block, DATxHash: "0xt"}
}

func TestScanOnce_ScansPastConfirmDepthOnly(t *testing.T) {
	src := &fakeSource{latest: 112, commits: map[uint64][]Commit{
		1: {mkCommit(1, 50)},
	}}
	sink := &fakeSink{}
	p := NewPoller(src, sink, 12, 0, testLog)

	if err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// safeHead = 112-12 = 100; cursor starts 0 → scan [1,100].
	if len(src.calls) != 1 || src.calls[0] != [2]uint64{1, 100} {
		t.Fatalf("calls = %v, want [[1 100]]", src.calls)
	}
	if sink.cursor != 100 || len(sink.commits) != 1 {
		t.Fatalf("cursor=%d commits=%d", sink.cursor, len(sink.commits))
	}
}

func TestScanOnce_ChunksLargeRanges(t *testing.T) {
	src := &fakeSource{latest: 5012}
	sink := &fakeSink{}
	p := NewPoller(src, sink, 12, 0, testLog)

	if err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// safeHead 5000, chunk 2000: [1,2000] [2001,4000] [4001,5000]
	want := [][2]uint64{{1, 2000}, {2001, 4000}, {4001, 5000}}
	if len(src.calls) != len(want) {
		t.Fatalf("calls = %v", src.calls)
	}
	for i, w := range want {
		if src.calls[i] != w {
			t.Errorf("call %d = %v, want %v", i, src.calls[i], w)
		}
	}
	if sink.persists != 3 || sink.cursor != 5000 {
		t.Errorf("persists=%d cursor=%d", sink.persists, sink.cursor)
	}
}

func TestScanOnce_NoopWhenCaughtUp(t *testing.T) {
	src := &fakeSource{latest: 112}
	sink := &fakeSink{cursor: 100, hasCursor: true}
	p := NewPoller(src, sink, 12, 0, testLog)

	if err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(src.calls) != 0 || sink.persists != 0 {
		t.Errorf("expected no scans, got calls=%v persists=%d", src.calls, sink.persists)
	}
}

func TestScanOnce_ChainShorterThanDepth(t *testing.T) {
	src := &fakeSource{latest: 5}
	sink := &fakeSink{}
	p := NewPoller(src, sink, 12, 0, testLog)
	if err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(src.calls) != 0 {
		t.Errorf("expected no scans, got %v", src.calls)
	}
}

func TestScanOnce_StartBlockRespectedOnFirstBoot(t *testing.T) {
	src := &fakeSource{latest: 1012}
	sink := &fakeSink{} // no cursor row yet
	p := NewPoller(src, sink, 12, 500, testLog)
	if err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// DA_START_BLOCK itself must be included in the first scan.
	if len(src.calls) != 1 || src.calls[0] != [2]uint64{500, 1000} {
		t.Fatalf("calls = %v, want [[500 1000]]", src.calls)
	}
}

func TestScanOnce_StorePersistFailurePropagates(t *testing.T) {
	src := &fakeSource{latest: 112}
	sink := &fakeSink{failStores: true}
	p := NewPoller(src, sink, 12, 0, testLog)
	if err := p.ScanOnce(context.Background()); err == nil {
		t.Fatal("expected error when persistence fails")
	}
}
