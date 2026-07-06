// Package da polls the QDAY DA node (EVM) for new batch commits (spec §7.1).
package da

import "context"

// Commit is one batch commit event read from the DA chain.
type Commit struct {
	BatchNumber uint64
	StateRoot   [32]byte
	DABlock     uint64
	DATxHash    string
}

// CommitSource abstracts where commits come from. The event ABI is still
// 【TO-CONFIRM】(C1, Vike) — when the real ABI lands, only the
// EventLogCommitSource implementation file changes.
type CommitSource interface {
	// FetchCommits scans the inclusive block range [from, to] for commit events.
	FetchCommits(ctx context.Context, from, to uint64) ([]Commit, error)
	// LatestBlock returns the DA chain head number.
	LatestBlock(ctx context.Context) (uint64, error)
}
