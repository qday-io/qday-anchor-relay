package abel

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// testAccount is a real (SDK-generated) account, shared across tests because
// PQ key generation is not free.
var (
	testAcctOnce sync.Once
	testAcct     abelian.Account
)

func submitterAccount(t *testing.T) abelian.Account {
	t.Helper()
	testAcctOnce.Do(func() {
		acct, err := abelian.NewAccount(abelian.TestNet, abelian.AccountPrivacyLevelPseudonym)
		if err != nil {
			t.Fatalf("NewAccount: %v", err)
		}
		testAcct = acct
	})
	return testAcct
}

type fakeSelector struct {
	coin       *SpendableCoin
	staleLocks []*SpendableCoin // pre-existing LOCKED coins from a "crash"
	locked     bool
	released   bool
	spent      bool
	reconciled []string // coinIDs reconciled: "release:<id>" or "spend:<id>"
}

func (f *fakeSelector) CoinsLockedByJob(ctx context.Context, jobID int64) ([]*SpendableCoin, error) {
	return f.staleLocks, nil
}

func (f *fakeSelector) SelectAndLockCoin(ctx context.Context, jobID int64, minValue int64) (*SpendableCoin, error) {
	if f.coin == nil {
		return nil, ErrNoSpendableCoin
	}
	if f.coin.ValueNeutrino < minValue {
		return nil, ErrNoSpendableCoin
	}
	f.locked = true
	return f.coin, nil
}

func (f *fakeSelector) ReleaseCoin(ctx context.Context, coinID string) error {
	f.released = true
	f.reconciled = append(f.reconciled, "release:"+coinID)
	return nil
}

func (f *fakeSelector) MarkCoinSpent(ctx context.Context, coinID string, jobID int64) error {
	f.spent = true
	f.reconciled = append(f.reconciled, "spend:"+coinID)
	return nil
}

// minimalRing builds a small but serializable CoinRing containing the coin.
func minimalRing(t *testing.T, coin *SpendableCoin) *abelian.CoinRing {
	t.Helper()
	// Block/tx ids must be valid 32-byte hex — Serialize hex-decodes them.
	ring, err := abelian.NewCoinRing(
		1, coin.BlockHeight,
		[]string{strings.Repeat("11", 32), strings.Repeat("22", 32), strings.Repeat("33", 32)},
		[]*abelian.CoinID{abelian.NewCoinID(coin.TxHash, coin.OutIndex)},
		[][]byte{[]byte("serialized-txo")},
		false,
	)
	if err != nil {
		t.Fatalf("NewCoinRing: %v", err)
	}
	return ring
}

type fakeRings struct {
	ring *abelian.CoinRing
	err  error
}

func (f *fakeRings) RingFor(txID string, outIndex uint8, blockHeight int64) (*abelian.CoinRing, error) {
	return f.ring, f.err
}

type fakeCrafter struct {
	lastDesc *abelian.TxDescWithRing
	err      error
}

func (f *fakeCrafter) Craft(txDesc *abelian.TxDescWithRing, signer abelian.Account) (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	f.lastDesc = txDesc
	return "deadbeef", "crafted-txid", nil
}

// submitChain: fake chain pre-loaded with the coin's source tx and block.
func submitChain(coin *SpendableCoin, voutScript string) *fakeChain {
	return &fakeChain{
		height: coin.BlockHeight + 100,
		blocks: map[int64]*abelian.Block{
			coin.BlockHeight: mkBlock(coin.BlockHeight, "cb", coin.TxHash),
		},
		txs: map[string]*abelian.Tx{
			coin.TxHash: mkTx(coin.TxHash, voutScript),
		},
	}
}

func testCoin() *SpendableCoin {
	return &SpendableCoin{
		CoinID: "srctx:0", TxHash: "srctx", OutIndex: 0,
		ValueNeutrino: 5_000_000, BlockHeight: 10,
	}
}

const testFee = 1_000_000

func newTestSubmitter(t *testing.T, chain Client, sel CoinSelector, rings RingProvider, crafter TxCrafter) *Submitter {
	return NewSubmitter(chain, submitterAccount(t), sel, rings, crafter, testFee, scanLog)
}

func TestSubmit_HappyPath_MemoAndChange(t *testing.T) {
	coin := testCoin()
	voutData := []byte("source-vout-data")
	chain := submitChain(coin, hex.EncodeToString(voutData))
	sel := &fakeSelector{coin: coin}
	crafter := &fakeCrafter{}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, crafter)

	memo := []byte("QDAY2ANCHOR|v1|7|0x" + "00")
	res, err := sub.Submit(context.Background(), 42, memo, "", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Resumed || res.TxHash != "sent-tx" {
		t.Errorf("result = %+v", res)
	}
	if !sel.spent || sel.released {
		t.Errorf("coin lifecycle wrong: spent=%v released=%v", sel.spent, sel.released)
	}

	desc := crafter.lastDesc
	if desc == nil {
		t.Fatal("crafter never called")
	}
	if string(desc.TxMemo) != string(memo) {
		t.Errorf("TxMemo = %q, want %q", desc.TxMemo, memo)
	}
	if desc.TxFee != testFee {
		t.Errorf("TxFee = %d", desc.TxFee)
	}
	if len(desc.TxInDescs) != 1 || len(desc.TxOutDescs) != 1 {
		t.Fatalf("ins=%d outs=%d", len(desc.TxInDescs), len(desc.TxOutDescs))
	}
	in := desc.TxInDescs[0]
	if in.TxID != coin.TxHash || in.CoinValue != coin.ValueNeutrino || string(in.TxOutData) != string(voutData) {
		t.Errorf("input = %+v", in)
	}
	if got := desc.TxOutDescs[0].CoinValue; got != coin.ValueNeutrino-testFee {
		t.Errorf("change = %d, want %d", got, coin.ValueNeutrino-testFee)
	}
}

func TestSubmit_IdempotentResume(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	chain.txs["existing-hash"] = mkTx("existing-hash") // already on chain
	sel := &fakeSelector{coin: coin}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{}, &fakeCrafter{})

	res, err := sub.Submit(context.Background(), 42, []byte("m"), "existing-hash", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || res.TxHash != "existing-hash" {
		t.Errorf("result = %+v", res)
	}
	if sel.locked {
		t.Error("must not lock a coin when resuming an existing tx")
	}
}

func TestSubmit_ResendWhenTxVanished(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00") // "existing-hash" NOT in chain
	sel := &fakeSelector{coin: coin}
	crafter := &fakeCrafter{}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, crafter)

	res, err := sub.Submit(context.Background(), 42, []byte("same-memo"), "existing-hash", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Resumed || res.TxHash != "sent-tx" {
		t.Errorf("result = %+v", res)
	}
	if string(crafter.lastDesc.TxMemo) != "same-memo" {
		t.Error("resend must keep the identical memo payload")
	}
}

func TestSubmit_NoCoinPropagates(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	sub := newTestSubmitter(t, chain, &fakeSelector{}, &fakeRings{}, &fakeCrafter{})
	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", nil); !errors.Is(err, ErrNoSpendableCoin) {
		t.Errorf("err = %v, want ErrNoSpendableCoin", err)
	}
}

func TestSubmit_CoinTooSmallForFeePlusChange(t *testing.T) {
	coin := testCoin()
	coin.ValueNeutrino = testFee // == fee → cannot make non-zero change
	chain := submitChain(coin, "00")
	sub := newTestSubmitter(t, chain, &fakeSelector{coin: coin}, &fakeRings{}, &fakeCrafter{})
	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", nil); !errors.Is(err, ErrNoSpendableCoin) {
		t.Errorf("err = %v, want ErrNoSpendableCoin", err)
	}
}

func TestSubmit_ReleasesCoinOnPreBroadcastFailure(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	sel := &fakeSelector{coin: coin}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, &fakeCrafter{err: errors.New("sign blew up")})

	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", nil); err == nil {
		t.Fatal("expected craft error")
	}
	if !sel.released || sel.spent {
		t.Errorf("coin lifecycle wrong: released=%v spent=%v", sel.released, sel.spent)
	}
}

func TestSubmit_RingNotReadyReleasesCoin(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	sel := &fakeSelector{coin: coin}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{err: ErrRingNotReady}, &fakeCrafter{})

	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", nil); !errors.Is(err, ErrRingNotReady) {
		t.Fatalf("err = %v, want ErrRingNotReady", err)
	}
	if !sel.released {
		t.Error("coin must be released when ring is not ready")
	}
}

func TestSDKRingProvider_NotReadyGate(t *testing.T) {
	// Pick a coin whose ring group is provably incomplete: the group of the
	// chain tip itself, when the tip is not the group's last block.
	tip := int64(100)
	heights := abelian.GetRingBlockHeights(tip)
	last := heights[len(heights)-1]
	if last == tip {
		tip-- // shift so the group extends beyond the tip
	}
	coin := &SpendableCoin{CoinID: "c:0", TxHash: "c", OutIndex: 0, ValueNeutrino: 1, BlockHeight: tip}
	chain := &fakeChain{height: tip, blocks: map[int64]*abelian.Block{}, txs: map[string]*abelian.Tx{}}
	heights = abelian.GetRingBlockHeights(coin.BlockHeight)
	if heights[len(heights)-1] <= chain.height {
		t.Skipf("cannot construct incomplete group at tip %d (group %v)", tip, heights)
	}

	p := NewSDKRingProvider(chain)
	if _, err := p.RingFor(coin.TxHash, coin.OutIndex, coin.BlockHeight); !errors.Is(err, ErrRingNotReady) {
		t.Errorf("err = %v, want ErrRingNotReady", err)
	}
}

func TestSubmit_PersistTxIDBeforeBroadcast(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	sel := &fakeSelector{coin: coin}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, &fakeCrafter{})

	var persisted string
	persist := func(ctx context.Context, txHash string) error {
		persisted = txHash
		return nil
	}
	res, err := sub.Submit(context.Background(), 42, []byte("m"), "", persist)
	if err != nil {
		t.Fatal(err)
	}
	if persisted != "crafted-txid" {
		t.Errorf("persisted txid = %q, want crafted-txid", persisted)
	}
	if res.TxHash != "sent-tx" {
		t.Errorf("res = %+v", res)
	}
}

func TestSubmit_PersistFailureAbortsBeforeBroadcast(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	sel := &fakeSelector{coin: coin}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, &fakeCrafter{})

	persist := func(ctx context.Context, txHash string) error {
		return errors.New("db down")
	}
	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", persist); err == nil {
		t.Fatal("expected persist error to abort submit")
	}
	if !sel.released {
		t.Error("coin must be released when persist fails (no broadcast happened)")
	}
}

func TestSubmit_ReconcilesStaleLocksOnResume(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	chain.txs["existing-hash"] = mkTx("existing-hash")
	stale := &SpendableCoin{CoinID: "stale:0", TxHash: "stale", OutIndex: 0, ValueNeutrino: 1, BlockHeight: 1}
	sel := &fakeSelector{coin: coin, staleLocks: []*SpendableCoin{stale}}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{}, &fakeCrafter{})

	res, err := sub.Submit(context.Background(), 42, []byte("m"), "existing-hash", nil)
	if err != nil || !res.Resumed {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	// Tx exists on chain → the stale locked coin funded it → finalize SPENT.
	if len(sel.reconciled) != 1 || sel.reconciled[0] != "spend:stale:0" {
		t.Errorf("reconciled = %v, want [spend:stale:0]", sel.reconciled)
	}
}

func TestSubmit_ReleasesStaleLocksOnFreshAttempt(t *testing.T) {
	coin := testCoin()
	chain := submitChain(coin, "00")
	stale := &SpendableCoin{CoinID: "stale:0", TxHash: "stale", OutIndex: 0, ValueNeutrino: 1, BlockHeight: 1}
	sel := &fakeSelector{coin: coin, staleLocks: []*SpendableCoin{stale}}
	sub := newTestSubmitter(t, chain, sel, &fakeRings{ring: minimalRing(t, coin)}, &fakeCrafter{})

	if _, err := sub.Submit(context.Background(), 42, []byte("m"), "", nil); err != nil {
		t.Fatal(err)
	}
	// No prior tx → stale lock was a pre-broadcast crash → release it first.
	if sel.reconciled[0] != "release:stale:0" {
		t.Errorf("reconciled = %v, want release:stale:0 first", sel.reconciled)
	}
}
