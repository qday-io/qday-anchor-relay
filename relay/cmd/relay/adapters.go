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

func (a storeCoinSink) CoinsMissingSerial(ctx context.Context, upToHeight int64) ([]abel.CoinRow, error) {
	coins, err := a.s.CoinsMissingSerial(ctx, upToHeight)
	if err != nil {
		return nil, err
	}
	out := make([]abel.CoinRow, len(coins))
	for i, c := range coins {
		out[i] = abel.CoinRow{
			CoinID:        c.CoinID,
			TxHash:        c.TxHash,
			OutIndex:      c.OutIndex,
			ValueNeutrino: c.ValueNeutrino,
			BlockHeight:   c.BlockHeight,
		}
	}
	return out, nil
}

func (a storeCoinSink) SetCoinSerial(ctx context.Context, coinID, serialHex, ringID string, ringIndex int16) error {
	return a.s.SetCoinSerial(ctx, coinID, serialHex, ringID, ringIndex)
}

func (a storeCoinSink) MarkCoinSpentBySerial(ctx context.Context, serialHex string) (bool, error) {
	return a.s.MarkCoinSpentBySerial(ctx, serialHex)
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

func (a storeCoinSelector) CoinsLockedByJob(ctx context.Context, jobID int64) ([]*abel.SpendableCoin, error) {
	coins, err := a.s.CoinsLockedByJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	out := make([]*abel.SpendableCoin, len(coins))
	for i, c := range coins {
		out[i] = &abel.SpendableCoin{
			CoinID:        c.CoinID,
			TxHash:        c.TxHash,
			OutIndex:      uint8(c.OutIndex),
			ValueNeutrino: c.ValueNeutrino,
			BlockHeight:   c.BlockHeight,
		}
	}
	return out, nil
}

func (a storeCoinSelector) ReleaseCoin(ctx context.Context, coinID string) error {
	return a.s.ReleaseCoin(ctx, coinID)
}

func (a storeCoinSelector) MarkCoinSpent(ctx context.Context, coinID string, jobID int64) error {
	return a.s.MarkCoinSpent(ctx, coinID, jobID)
}
