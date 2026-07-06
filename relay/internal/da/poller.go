package da

import (
	"context"
	"fmt"
	"log/slog"
)

// maxBlocksPerScan bounds a single eth_getLogs range so huge catch-ups don't
// blow RPC limits; the poller just takes several ticks to catch up.
const maxBlocksPerScan = 2000

// JobSink persists scanned commits and the new cursor atomically (implemented
// by store.Store via a thin adapter in cmd/relay).
type JobSink interface {
	Cursor(ctx context.Context, defaultStart uint64) (uint64, error)
	CreateJobsAndAdvanceCursor(ctx context.Context, commits []Commit, newCursor uint64) error
}

// Poller scans the DA chain for commit events past the reorg-safe head.
type Poller struct {
	source       CommitSource
	sink         JobSink
	confirmDepth uint64
	startBlock   uint64
	log          *slog.Logger
}

func NewPoller(source CommitSource, sink JobSink, confirmDepth, startBlock uint64, log *slog.Logger) *Poller {
	return &Poller{source: source, sink: sink, confirmDepth: confirmDepth, startBlock: startBlock, log: log}
}

// ScanOnce performs one poll tick: scan (cursor, safeHead] in bounded chunks,
// persisting jobs + cursor per chunk in one DB transaction (spec §7.1).
func (p *Poller) ScanOnce(ctx context.Context) error {
	latest, err := p.source.LatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("da: latest block: %w", err)
	}
	if latest < p.confirmDepth {
		return nil // chain shorter than confirm depth; nothing safe to scan
	}
	safeHead := latest - p.confirmDepth

	cursor, err := p.sink.Cursor(ctx, p.startBlock)
	if err != nil {
		return fmt.Errorf("da: load cursor: %w", err)
	}

	for cursor < safeHead {
		from := cursor + 1
		to := min(safeHead, cursor+maxBlocksPerScan)

		commits, err := p.source.FetchCommits(ctx, from, to)
		if err != nil {
			return err
		}
		if err := p.sink.CreateJobsAndAdvanceCursor(ctx, commits, to); err != nil {
			return fmt.Errorf("da: persist scan [%d,%d]: %w", from, to, err)
		}
		if len(commits) > 0 {
			p.log.Info("da: new commits", "from", from, "to", to, "count", len(commits))
		}
		cursor = to
	}
	return nil
}
