package main

import (
	"context"
	"errors"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
	"github.com/qday/qday-anchor-relay/relay/internal/da"
	"github.com/qday/qday-anchor-relay/relay/internal/store"
)

// storeJobSink adapts store.Store to da.JobSink (da.Commit ↔ store.Commit).
type storeJobSink struct{ s *store.Store }

func (a storeJobSink) Cursor(ctx context.Context, defaultStart uint64) (uint64, error) {
	return a.s.Cursor(ctx, defaultStart)
}

func (a storeJobSink) CreateJobsAndAdvanceCursor(ctx context.Context, commits []da.Commit, newCursor uint64) error {
	converted := make([]store.Commit, len(commits))
	for i, c := range commits {
		converted[i] = store.Commit{
			BatchNumber:   c.BatchNumber,
			StateRoot:     c.StateRoot,
			DABlockNumber: c.DABlock,
			DATxHash:      c.DATxHash,
		}
	}
	return a.s.CreateJobsAndAdvanceCursor(ctx, converted, newCursor)
}

// storeCoinSink adapts store.Store to abel.CoinSink.
type storeCoinSink struct{ s *store.Store }

func (a storeCoinSink) UpsertUnspentCoin(ctx context.Context, c abel.CoinRow) error {
	return a.s.UpsertCoin(ctx, &store.Coin{
		CoinID:        c.CoinID,
		TxHash:        c.TxHash,
		OutIndex:      c.OutIndex,
		ValueNeutrino: c.ValueNeutrino,
		BlockHeight:   c.BlockHeight,
		Status:        store.CoinUnspent,
	})
}

func (a storeCoinSink) AbelCursor(ctx context.Context, defaultStart int64) (int64, error) {
	return a.s.AbelCursor(ctx, defaultStart)
}

func (a storeCoinSink) SetAbelCursor(ctx context.Context, height int64) error {
	return a.s.SetAbelCursor(ctx, height)
}

// storeCoinSelector adapts store.Store to abel.CoinSelector.
type storeCoinSelector struct{ s *store.Store }

func (a storeCoinSelector) SelectAndLockCoin(ctx context.Context, jobID int64, minValue int64) (*abel.SpendableCoin, error) {
	coin, err := a.s.SelectAndLockCoin(ctx, jobID, minValue)
	if err != nil {
		if errors.Is(err, store.ErrNoSpendableCoin) {
			return nil, abel.ErrNoSpendableCoin
		}
		return nil, err
	}
	return &abel.SpendableCoin{
		CoinID:        coin.CoinID,
		TxHash:        coin.TxHash,
		OutIndex:      uint8(coin.OutIndex),
		ValueNeutrino: coin.ValueNeutrino,
		BlockHeight:   coin.BlockHeight,
	}, nil
}

func (a storeCoinSelector) ReleaseCoin(ctx context.Context, coinID string) error {
	return a.s.ReleaseCoin(ctx, coinID)
}

func (a storeCoinSelector) MarkCoinSpent(ctx context.Context, coinID string, jobID int64) error {
	return a.s.MarkCoinSpent(ctx, coinID, jobID)
}
