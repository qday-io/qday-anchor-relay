package abel

import (
	"context"
	"testing"
	"time"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

type confirmChain struct {
	fakeChain
	mempool []string
}

func (c *confirmChain) MempoolTxIDs() ([]string, error) { return c.mempool, nil }

func newConfirmChain(tip int64) *confirmChain {
	return &confirmChain{fakeChain: fakeChain{
		height: tip,
		blocks: map[int64]*abelian.Block{},
		txs:    map[string]*abelian.Tx{},
	}}
}

func (c *confirmChain) addMinedTx(txID string, blockHeight int64, nodeConfs int64) {
	hash := "bh-" + txID
	c.blocks[blockHeight] = &abelian.Block{Height: blockHeight, BlockHash: hash, Confirmations: nodeConfs}
	c.txs[txID] = &abelian.Tx{TxID: txID, BlockHash: hash, Version: 1}
}

const depth = 6

func TestConfirmer_ConfirmedAtDepth(t *testing.T) {
	chain := newConfirmChain(100)
	chain.addMinedTx("tx1", 95, 6) // node reports 6 confs
	c := NewConfirmer(chain, depth, time.Hour, scanLog)

	res, err := c.Check(context.Background(), "tx1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != Confirmed || res.AbelHeight != 95 || res.Confirmations != 6 {
		t.Errorf("res = %+v", res)
	}
}

func TestConfirmer_PendingBelowDepth(t *testing.T) {
	chain := newConfirmChain(100)
	chain.addMinedTx("tx1", 98, 3)
	c := NewConfirmer(chain, depth, time.Hour, scanLog)

	res, err := c.Check(context.Background(), "tx1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != ConfirmPending || res.Confirmations != 3 {
		t.Errorf("res = %+v", res)
	}
}

// Node-reported Block.Confirmations may be zero (or missing); the confirmer
// must fall back to tip - height + 1. This is the workaround for the SDK's
// Tx.Confirmations bson-tag bug — never trust Tx.Confirmations.
func TestConfirmer_FallbackConfirmationsFromTip(t *testing.T) {
	chain := newConfirmChain(100)
	chain.addMinedTx("tx1", 94, 0) // node gives no confirmation count
	c := NewConfirmer(chain, depth, time.Hour, scanLog)

	res, err := c.Check(context.Background(), "tx1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// 100 - 94 + 1 = 7 ≥ 6
	if res.Status != Confirmed || res.Confirmations != 7 {
		t.Errorf("res = %+v", res)
	}
}

func TestConfirmer_MempoolTxPending(t *testing.T) {
	chain := newConfirmChain(100)
	chain.txs["tx1"] = &abelian.Tx{TxID: "tx1", BlockHash: ""} // known, unmined
	c := NewConfirmer(chain, depth, time.Hour, scanLog)

	res, err := c.Check(context.Background(), "tx1", time.Now())
	if err != nil || res.Status != ConfirmPending {
		t.Errorf("res = %+v err = %v", res, err)
	}
}

func TestConfirmer_UnknownTxInMempoolListPending(t *testing.T) {
	chain := newConfirmChain(100)
	chain.mempool = []string{"tx1"} // GetRawTx fails but mempool lists it
	c := NewConfirmer(chain, depth, time.Hour, scanLog)

	res, err := c.Check(context.Background(), "tx1", time.Now())
	if err != nil || res.Status != ConfirmPending {
		t.Errorf("res = %+v err = %v", res, err)
	}
}

func TestConfirmer_LostAfterGrace(t *testing.T) {
	chain := newConfirmChain(100) // tx nowhere
	c := NewConfirmer(chain, depth, time.Minute, scanLog)

	// Within grace → pending.
	res, err := c.Check(context.Background(), "ghost", time.Now())
	if err != nil || res.Status != ConfirmPending {
		t.Fatalf("within grace: res = %+v err = %v", res, err)
	}
	// Past grace → lost.
	res, err = c.Check(context.Background(), "ghost", time.Now().Add(-2*time.Minute))
	if err != nil || res.Status != ConfirmLost {
		t.Fatalf("past grace: res = %+v err = %v", res, err)
	}
}
