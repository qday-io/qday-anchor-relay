package registry

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

// deployerKey is a throwaway dev key for the simulated backend only.
const deployerKeyHex = "4c0883a69102937d6231471b5dbb6204fe51296170827936ea5cce4b76994b0f"

const simChainID = 1337 // ethclient/simulated fixed chain id

func newSimRecorder(t *testing.T) (*Recorder, *simulated.Backend) {
	t.Helper()
	key, err := crypto.HexToECDSA(deployerKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	sim := simulated.NewBackend(types.GenesisAlloc{
		addr: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)},
	})
	t.Cleanup(func() { sim.Close() })
	client := sim.Client()

	opts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(simChainID))
	if err != nil {
		t.Fatal(err)
	}
	contractAddr, _, contract, err := DeployQDayAnchorRegistry(opts, client)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	sim.Commit()

	// Owner authorizes itself as relayer (mirrors Deploy.s.sol).
	if _, err := contract.SetRelayer(opts, addr, true); err != nil {
		t.Fatalf("setRelayer: %v", err)
	}
	sim.Commit()

	rec, err := NewRecorder(client, contractAddr, deployerKeyHex, simChainID)
	if err != nil {
		t.Fatal(err)
	}
	rec.receiptPoll = 10 * time.Millisecond
	// The simulated backend only mines on Commit(); mine in the background so
	// waitMined can observe the receipt.
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				sim.Commit()
			}
		}
	}()
	return rec, sim
}

func testRecord(batch uint64) AnchorRecord {
	var root, abelTx [32]byte
	root[0] = byte(batch)
	abelTx[0] = 0xab
	return AnchorRecord{BatchNumber: batch, StateRoot: root, AbelTxHash: abelTx, AbelHeight: 999}
}

func TestRecorder_RecordAndIsAnchored(t *testing.T) {
	rec, _ := newSimRecorder(t)
	ctx := context.Background()
	r := testRecord(7)

	anchored, err := rec.IsAnchored(ctx, r.StateRoot)
	if err != nil || anchored {
		t.Fatalf("IsAnchored before = %v, %v", anchored, err)
	}

	txHash, err := rec.Record(ctx, r)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !strings.HasPrefix(txHash, "0x") {
		t.Errorf("txHash = %q", txHash)
	}

	anchored, err = rec.IsAnchored(ctx, r.StateRoot)
	if err != nil || !anchored {
		t.Fatalf("IsAnchored after = %v, %v", anchored, err)
	}
}

func TestRecorder_DuplicateReverts(t *testing.T) {
	rec, _ := newSimRecorder(t)
	ctx := context.Background()
	r := testRecord(8)

	if _, err := rec.Record(ctx, r); err != nil {
		t.Fatal(err)
	}
	// Same root again → contract revert must surface as an error (caller then
	// re-checks IsAnchored and advances, rule 6.1-3).
	if _, err := rec.Record(ctx, r); err == nil {
		t.Fatal("expected duplicate recordAnchor to fail")
	}
	anchored, err := rec.IsAnchored(ctx, r.StateRoot)
	if err != nil || !anchored {
		t.Fatalf("IsAnchored = %v, %v", anchored, err)
	}
}
