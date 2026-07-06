package abel

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
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
		return nil, fmt.Errorf("no tx %s", txID)
	}
	return tx, nil
}

func (f *fakeChain) SendSignedTx(rawHex string) (string, error) { return "sent-tx", nil }
func (f *fakeChain) RingBlockGroup(h int64) ([][]byte, error)   { return nil, fmt.Errorf("not implemented") }
func (f *fakeChain) MempoolTxIDs() ([]string, error)            { return nil, nil }

// fakeReceiver claims ownership of outputs whose script data starts with "mine".
type fakeReceiver struct{}

func (fakeReceiver) ReceiveCoin(txVersion uint32, txOutData []byte) (bool, uint64, error) {
	if len(txOutData) >= 4 && string(txOutData[:4]) == "mine" {
		return true, 7_000_000, nil
	}
	return false, 0, nil
}

type fakeCoinSink struct {
	coins  []CoinRow
	cursor int64
	has    bool
}

func (f *fakeCoinSink) UpsertUnspentCoin(ctx context.Context, c CoinRow) error {
	f.coins = append(f.coins, c)
	return nil
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
	s := NewScanner(chain, fakeReceiver{}, sink, 0, scanLog)

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
	s := NewScanner(chain, fakeReceiver{}, sink, 0, scanLog)
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
	s := NewScanner(chain, fakeReceiver{}, sink, 0, scanLog)

	if err := s.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sink.cursor != maxBlocksPerScanTick {
		t.Errorf("cursor after one tick = %d, want %d", sink.cursor, maxBlocksPerScanTick)
	}
}
