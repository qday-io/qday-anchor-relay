package abel

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// Receiver is the coin-ownership test — satisfied by abelian.Account /
// ViewAccount (docs/sdk-verified-api.md: ReceiveCoin).
type Receiver interface {
	ReceiveCoin(txVersion uint32, txOutData []byte) (success bool, v uint64, err error)
}

// CoinRow is the scanner→store shape (mirrors store.Coin; keeps this package
// free of internal/store).
type CoinRow struct {
	CoinID        string
	TxHash        string
	OutIndex      int16
	ValueNeutrino int64
	BlockHeight   int64
}

// CoinSink persists discovered coins and the scan cursor.
type CoinSink interface {
	UpsertUnspentCoin(ctx context.Context, c CoinRow) error
	AbelCursor(ctx context.Context, defaultStart int64) (int64, error)
	SetAbelCursor(ctx context.Context, height int64) error
}

// Scanner walks Abelian blocks and records outputs belonging to the relay
// wallet (spec §7.2 coin 掃描).
//
// Coinbase outputs (block tx index 0) are intentionally skipped: the relay
// wallet is expected to be funded by transfer, and skipping sidesteps the
// 200-block coinbase maturity rule (F11). Documented in README.
type Scanner struct {
	client     Client
	receiver   Receiver
	sink       CoinSink
	startInMem int64
	log        *slog.Logger
}

func NewScanner(client Client, receiver Receiver, sink CoinSink, startHeight int64, log *slog.Logger) *Scanner {
	return &Scanner{client: client, receiver: receiver, sink: sink, startInMem: startHeight, log: log}
}

// maxBlocksPerScanTick bounds one tick's work so a cold start (deep catch-up)
// doesn't starve the other workers or hold a tick forever.
const maxBlocksPerScanTick = 500

// ScanOnce advances the coin scan by at most maxBlocksPerScanTick blocks.
func (s *Scanner) ScanOnce(ctx context.Context) error {
	tip, err := s.client.ChainHeight()
	if err != nil {
		return fmt.Errorf("abel: chain height: %w", err)
	}
	cursor, err := s.sink.AbelCursor(ctx, s.startInMem)
	if err != nil {
		return fmt.Errorf("abel: load scan cursor: %w", err)
	}

	end := min(tip, cursor+maxBlocksPerScanTick)
	for h := cursor + 1; h <= end; h++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.scanBlock(ctx, h); err != nil {
			return err
		}
		if err := s.sink.SetAbelCursor(ctx, h); err != nil {
			return fmt.Errorf("abel: advance scan cursor: %w", err)
		}
	}
	return nil
}

func (s *Scanner) scanBlock(ctx context.Context, height int64) error {
	block, err := s.client.BlockByHeight(height)
	if err != nil {
		return fmt.Errorf("abel: block %d: %w", height, err)
	}
	for i, txID := range block.TxHashes {
		if i == 0 {
			continue // coinbase (see type comment)
		}
		tx, err := s.client.RawTx(txID)
		if err != nil {
			return fmt.Errorf("abel: tx %s: %w", txID, err)
		}
		if err := s.scanTxOutputs(ctx, tx, block); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) scanTxOutputs(ctx context.Context, tx *abelian.Tx, block *abelian.Block) error {
	for idx, vout := range tx.Vout {
		txOutData, err := hex.DecodeString(vout.Script)
		if err != nil {
			return fmt.Errorf("abel: tx %s vout %d script not hex: %w", tx.TxID, idx, err)
		}
		ok, value, err := s.receiver.ReceiveCoin(tx.Version, txOutData)
		if err != nil {
			return fmt.Errorf("abel: ReceiveCoin tx %s vout %d: %w", tx.TxID, idx, err)
		}
		if !ok {
			continue
		}
		coin := CoinRow{
			CoinID:        abelian.NewCoinID(tx.TxID, uint8(idx)).String(),
			TxHash:        tx.TxID,
			OutIndex:      int16(idx),
			ValueNeutrino: int64(value),
			BlockHeight:   block.Height,
		}
		if err := s.sink.UpsertUnspentCoin(ctx, coin); err != nil {
			return fmt.Errorf("abel: persist coin %s: %w", coin.CoinID, err)
		}
		s.log.Info("abel: coin received", "coin", coin.CoinID, "value", coin.ValueNeutrino, "height", coin.BlockHeight)
	}
	return nil
}
