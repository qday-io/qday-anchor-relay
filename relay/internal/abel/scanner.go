package abel

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// Receiver is the coin-ownership test plus serial-number derivation —
// satisfied by abelian.Account / ViewAccount (docs/sdk-verified-api.md).
type Receiver interface {
	ReceiveCoin(txVersion uint32, txOutData []byte) (success bool, v uint64, err error)
	GenerateSerialNumberWithRing(coinID *abelian.CoinID, serializedRing []byte) ([]byte, error)
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

// CoinSink persists discovered coins, their serial numbers, spend marks, and
// the scan cursor.
type CoinSink interface {
	UpsertUnspentCoin(ctx context.Context, c CoinRow) error
	// CoinsMissingSerial lists tracked coins without a serial number, up to
	// (and including) the given height.
	CoinsMissingSerial(ctx context.Context, upToHeight int64) ([]CoinRow, error)
	SetCoinSerial(ctx context.Context, coinID, serialHex, ringID string, ringIndex int16) error
	// MarkCoinSpentBySerial marks the coin with this serial SPENT; returns
	// whether one of our coins matched.
	MarkCoinSpentBySerial(ctx context.Context, serialHex string) (bool, error)
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
	rings      RingProvider
	startInMem int64
	log        *slog.Logger
}

func NewScanner(client Client, receiver Receiver, sink CoinSink, rings RingProvider, startHeight int64, log *slog.Logger) *Scanner {
	return &Scanner{client: client, receiver: receiver, sink: sink, rings: rings, startInMem: startHeight, log: log}
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
		// On the last block of each ring group, rings for the group become
		// buildable — backfill serial numbers for our coins (mirrors
		// examples/coin/main.go's HandleCoinMaturity gating).
		if n := int64(abelian.GetBlockNumPerRingGroupByBlockHeight(h)); n > 0 && h%n == n-1 {
			if err := s.backfillSerials(ctx, h); err != nil {
				return err
			}
		}
		if err := s.sink.SetAbelCursor(ctx, h); err != nil {
			return fmt.Errorf("abel: advance scan cursor: %w", err)
		}
	}
	return nil
}

// serializeRing / ringIDOf are seams over CoinRing methods (tests stub them —
// a fully serializable ring requires real chain data).
var serializeRing = func(ring *abelian.CoinRing) ([]byte, error) {
	var buf bytes.Buffer
	if err := ring.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var ringIDOf = func(ring *abelian.CoinRing) (string, error) {
	return ring.RingId()
}

// backfillSerials derives serial numbers for tracked coins whose ring group is
// now complete (needed for chain-side spend detection).
func (s *Scanner) backfillSerials(ctx context.Context, height int64) error {
	coins, err := s.sink.CoinsMissingSerial(ctx, height)
	if err != nil {
		return fmt.Errorf("abel: coins missing serial: %w", err)
	}
	for _, coin := range coins {
		if err := ctx.Err(); err != nil {
			return err
		}
		heights := abelian.GetRingBlockHeights(coin.BlockHeight)
		if heights[len(heights)-1] > height {
			continue // group not complete yet
		}
		if err := s.backfillCoinSerial(ctx, coin); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) backfillCoinSerial(ctx context.Context, coin CoinRow) error {
	want := abelian.NewCoinID(coin.TxHash, uint8(coin.OutIndex))
	ring, err := s.rings.RingFor(coin.TxHash, uint8(coin.OutIndex), coin.BlockHeight)
	if err != nil {
		if errors.Is(err, ErrRingNotReady) {
			return nil // next boundary tick will retry
		}
		return fmt.Errorf("abel: ring for coin %s: %w", coin.CoinID, err)
	}
	ringIndex := -1
	for idx, id := range ring.CoinIDRing.CoinIDs {
		if id.TxID == want.TxID && id.Index == want.Index {
			ringIndex = idx
			break
		}
	}
	if ringIndex < 0 {
		s.log.Warn("abel: coin not found in its ring group", "coin", coin.CoinID)
		return nil
	}
	serialized, err := serializeRing(ring)
	if err != nil {
		return fmt.Errorf("abel: serialize ring for coin %s: %w", coin.CoinID, err)
	}
	serial, err := s.receiver.GenerateSerialNumberWithRing(want, serialized)
	if err != nil {
		return fmt.Errorf("abel: serial number for coin %s: %w", coin.CoinID, err)
	}
	ringID, err := ringIDOf(ring)
	if err != nil {
		return fmt.Errorf("abel: ring id for coin %s: %w", coin.CoinID, err)
	}
	if err := s.sink.SetCoinSerial(ctx, coin.CoinID, hex.EncodeToString(serial), ringID, int16(ringIndex)); err != nil {
		return fmt.Errorf("abel: persist serial for coin %s: %w", coin.CoinID, err)
	}
	s.log.Info("abel: coin serial backfilled", "coin", coin.CoinID)
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
		if err := s.scanTxSpends(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// scanTxSpends marks our coins SPENT when their serial number appears in a
// chain tx input (spend detection per examples/coin/main.go TrackCoins).
func (s *Scanner) scanTxSpends(ctx context.Context, tx *abelian.Tx) error {
	for _, vin := range tx.Vin {
		if vin == nil || vin.SerialNumber == "" {
			continue
		}
		matched, err := s.sink.MarkCoinSpentBySerial(ctx, vin.SerialNumber)
		if err != nil {
			return fmt.Errorf("abel: mark spent by serial (tx %s): %w", tx.TxID, err)
		}
		if matched {
			s.log.Info("abel: coin spent on chain", "tx", tx.TxID, "serial", vin.SerialNumber[:16]+"…")
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
