package abel

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

type fakeChain struct {
	height int64
	blocks map[int64]*abelian.Block
	txs    map[string]*abelian.Tx
}

func (f *fakeChain) ChainHeight() (int64, error) { return f.height, nil }

func (f *fakeChain) BlockByHeight(h int64) (*abelian.Block, error) {
	b, ok := f.blocks[h]
	if !ok {
		return nil, fmt.Errorf("no block %d", h)
	}
	return b, nil
}

func (f *fakeChain) Block(id string) (*abelian.Block, error) {
	for _, b := range f.blocks {
		if b.BlockHash == id {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no block %s", id)
}

func (f *fakeChain) RawTx(txID string) (*abelian.Tx, error) {
	tx, ok := f.txs[txID]
	if !ok {
		// Mirror the real node: an unknown tx is a JSON-RPC level error.
		return nil, &abelian.RPCError{Code: -5, Message: "No information available about transaction " + txID}
	}
	return tx, nil
}

func (f *fakeChain) SendSignedTx(rawHex string) (string, error) { return "sent-tx", nil }
func (f *fakeChain) RingBlockGroup(h int64) ([][]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeChain) MempoolTxIDs() ([]string, error) { return nil, nil }

// fakeReceiver claims ownership of outputs whose script data starts with "mine".
type fakeReceiver struct{}

func (fakeReceiver) ReceiveCoin(txVersion uint32, txOutData []byte) (bool, uint64, error) {
	if len(txOutData) >= 4 && string(txOutData[:4]) == "mine" {
		return true, 7_000_000, nil
	}
	return false, 0, nil
}

func (fakeReceiver) GenerateSerialNumberWithRing(coinID *abelian.CoinID, serializedRing []byte) ([]byte, error) {
	return []byte("serial-" + coinID.String()), nil
}

type fakeCoinSink struct {
	coins   []CoinRow
	serials map[string]string // coinID → serialHex
	spent   []string          // serials marked spent
	cursor  int64
	has     bool
}

func (f *fakeCoinSink) UpsertUnspentCoin(ctx context.Context, c CoinRow) error {
	f.coins = append(f.coins, c)
	return nil
}

func (f *fakeCoinSink) CoinsMissingSerial(ctx context.Context, upToHeight int64) ([]CoinRow, error) {
	var out []CoinRow
	for _, c := range f.coins {
		if _, ok := f.serials[c.CoinID]; !ok && c.BlockHeight <= upToHeight {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCoinSink) SetCoinSerial(ctx context.Context, coinID, serialHex, ringID string, ringIndex int16) error {
	if f.serials == nil {
		f.serials = map[string]string{}
	}
	f.serials[coinID] = serialHex
	return nil
}

func (f *fakeCoinSink) MarkCoinSpentBySerial(ctx context.Context, serialHex string) (bool, error) {
	f.spent = append(f.spent, serialHex)
	for _, s := range f.serials {
		if s == serialHex {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeCoinSink) AbelCursor(ctx context.Context, def int64) (int64, error) {
	if !f.has {
		return def, nil
	}
	return f.cursor, nil
}

func (f *fakeCoinSink) SetAbelCursor(ctx context.Context, h int64) error {
	f.cursor, f.has = h, true
	return nil
}

var scanLog = slog.New(slog.NewTextHandler(discardWriter{}, nil))

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func mkBlock(height int64, txIDs ...string) *abelian.Block {
	return &abelian.Block{Height: height, BlockHash: fmt.Sprintf("hash-%d", height), TxHashes: txIDs}
}

func mkTx(id string, scripts ...string) *abelian.Tx {
	tx := &abelian.Tx{TxID: id, Version: 1}
	for _, s := range scripts {
		tx.Vout = append(tx.Vout, &abelian.TxVout{Script: s})
	}
	return tx
}

func TestScanner_FindsOwnCoinsSkipsCoinbase(t *testing.T) {
	mine := hex.EncodeToString([]byte("mine-coin-data"))
	theirs := hex.EncodeToString([]byte("theirs"))

	chain := &fakeChain{
		height: 2,
		blocks: map[int64]*abelian.Block{
			1: mkBlock(1, "cb1", "t1"),
			2: mkBlock(2, "cb2", "t2"),
		},
		txs: map[string]*abelian.Tx{
			// cb1/cb2 are coinbase (index 0) and must never be fetched.
			"t1": mkTx("t1", theirs, mine), // vout 1 is ours
			"t2": mkTx("t2", theirs),
		},
	}
	sink := &fakeCoinSink{}
	s := NewScanner(chain, fakeReceiver{}, sink, &fakeRings{err: ErrRingNotReady}, 0, scanLog)

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.coins) != 1 {
		t.Fatalf("coins = %+v, want exactly 1", sink.coins)
	}
	c := sink.coins[0]
	if c.CoinID != "t1:1" || c.ValueNeutrino != 7_000_000 || c.BlockHeight != 1 || c.OutIndex != 1 {
		t.Errorf("coin = %+v", c)
	}
	if sink.cursor != 2 {
		t.Errorf("cursor = %d, want 2", sink.cursor)
	}
}

func TestScanner_ResumesFromCursor(t *testing.T) {
	chain := &fakeChain{
		height: 3,
		blocks: map[int64]*abelian.Block{3: mkBlock(3, "cb3")},
		txs:    map[string]*abelian.Tx{},
	}
	sink := &fakeCoinSink{cursor: 2, has: true}
	s := NewScanner(chain, fakeReceiver{}, sink, &fakeRings{err: ErrRingNotReady}, 0, scanLog)
	// Blocks 1-2 absent from the fake — touching them would error, proving
	// the scanner resumed at 3.
	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sink.cursor != 3 {
		t.Errorf("cursor = %d, want 3", sink.cursor)
	}
}

func TestScanner_BoundedCatchUp(t *testing.T) {
	blocks := map[int64]*abelian.Block{}
	for h := int64(1); h <= maxBlocksPerScanTick+10; h++ {
		blocks[h] = mkBlock(h, fmt.Sprintf("cb%d", h))
	}
	chain := &fakeChain{height: maxBlocksPerScanTick + 10, blocks: blocks, txs: map[string]*abelian.Tx{}}
	sink := &fakeCoinSink{}
	s := NewScanner(chain, fakeReceiver{}, sink, &fakeRings{err: ErrRingNotReady}, 0, scanLog)

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sink.cursor != maxBlocksPerScanTick {
		t.Errorf("cursor after one tick = %d, want %d", sink.cursor, maxBlocksPerScanTick)
	}
}

func TestScanner_BackfillsSerialAndDetectsSpend(t *testing.T) {
	mine := hex.EncodeToString([]byte("mine-coin-data"))
	hexTx := strings.Repeat("aa", 32) // ring serialization hex-decodes tx ids

	// Height chosen at a ring-group boundary: find h in [1,10] with h%n==n-1.
	var boundary int64 = -1
	for h := int64(1); h <= 10; h++ {
		n := int64(abelian.GetBlockNumPerRingGroupByBlockHeight(h))
		if n > 0 && h%n == n-1 {
			boundary = h
			break
		}
	}
	if boundary < 0 {
		t.Skip("no ring group boundary in test range")
	}

	blocks := map[int64]*abelian.Block{}
	txs := map[string]*abelian.Tx{}
	for h := int64(1); h <= boundary; h++ {
		blocks[h] = mkBlock(h, fmt.Sprintf("cb%d", h))
	}
	coinHeight := min(int64(1), boundary)
	blocks[coinHeight] = mkBlock(coinHeight, fmt.Sprintf("cb%d", coinHeight), hexTx)
	txs[hexTx] = mkTx(hexTx, mine)

	chain := &fakeChain{height: boundary, blocks: blocks, txs: txs}
	sink := &fakeCoinSink{}
	coin := &SpendableCoin{TxHash: hexTx, OutIndex: 0, BlockHeight: coinHeight}
	s := NewScanner(chain, fakeReceiver{}, sink, &fakeRings{ring: minimalRing(t, coin)}, 0, scanLog)

	// Real ring serialization needs genuine chain data — stub the seams.
	origSerialize, origRingID := serializeRing, ringIDOf
	serializeRing = func(ring *abelian.CoinRing) ([]byte, error) { return []byte("ring-bytes"), nil }
	ringIDOf = func(ring *abelian.CoinRing) (string, error) { return "ring-id-1", nil }
	t.Cleanup(func() { serializeRing, ringIDOf = origSerialize, origRingID })

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantSerial := hex.EncodeToString([]byte("serial-" + hexTx + ":0"))
	if got := sink.serials[hexTx+":0"]; got != wantSerial {
		t.Fatalf("serial = %q, want %q (serials=%v)", got, wantSerial, sink.serials)
	}

	// Next block spends it: Vin carries the serial.
	spendH := boundary + 1
	chain.height = spendH
	chain.blocks[spendH] = mkBlock(spendH, fmt.Sprintf("cb%d", spendH), "spendtx")
	spendTx := mkTx("spendtx")
	spendTx.Vin = []*abelian.TxVin{{SerialNumber: wantSerial}}
	chain.txs["spendtx"] = spendTx

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sp := range sink.spent {
		if sp == wantSerial {
			found = true
		}
	}
	if !found {
		t.Errorf("spend not detected; spent=%v", sink.spent)
	}
}
