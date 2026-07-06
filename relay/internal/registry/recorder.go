// Package registry writes completed anchors to the QDayAnchorRegistry
// contract on the DA chain (spec §7.4). The contract binding
// (qday_anchor_registry.go) is generated — regenerate with `make abigen`.
package registry

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// AnchorRecord is what gets written on-chain — the relay's final result.
type AnchorRecord struct {
	BatchNumber uint64
	StateRoot   [32]byte
	AbelTxHash  [32]byte
	AbelHeight  uint64
}

// ContractBackend is bind.ContractBackend; aliased for readability.
type ContractBackend = bind.ContractBackend

type Recorder struct {
	contract       *QDayAnchorRegistry
	backend        ContractBackend
	key            *ecdsa.PrivateKey
	chainID        *big.Int
	receiptTimeout time.Duration
	receiptPoll    time.Duration
}

// WaitMinedBackend adds receipt lookup, satisfied by *ethclient.Client and the
// simulated backend used in tests.
type WaitMinedBackend interface {
	ContractBackend
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

func NewRecorder(backend WaitMinedBackend, contractAddr common.Address, privateKeyHex string, chainID int64) (*Recorder, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("registry: parse relayer private key: %w", err)
	}
	contract, err := NewQDayAnchorRegistry(contractAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("registry: bind contract: %w", err)
	}
	return &Recorder{
		contract:       contract,
		backend:        backend,
		key:            key,
		chainID:        big.NewInt(chainID),
		receiptTimeout: 2 * time.Minute,
		receiptPoll:    2 * time.Second,
	}, nil
}

// IsAnchored asks the contract whether stateRoot is already recorded
// (idempotency rule 6.1-3).
func (r *Recorder) IsAnchored(ctx context.Context, stateRoot [32]byte) (bool, error) {
	return r.contract.IsAnchored(&bind.CallOpts{Context: ctx}, stateRoot)
}

// Record submits recordAnchor and waits for the receipt. Returns the record tx
// hash. Callers must check IsAnchored first; a revert here is re-checked
// against the contract to distinguish "already recorded" from real failure.
func (r *Recorder) Record(ctx context.Context, rec AnchorRecord) (string, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(r.key, r.chainID)
	if err != nil {
		return "", fmt.Errorf("registry: transactor: %w", err)
	}
	opts.Context = ctx
	// Pending nonce + suggested gas price (spec §7.4) are go-ethereum's
	// defaults when Nonce/GasPrice are nil — bind fills them from the backend.

	tx, err := r.contract.RecordAnchor(opts, rec.BatchNumber, rec.StateRoot, rec.AbelTxHash, rec.AbelHeight)
	if err != nil {
		return "", fmt.Errorf("registry: recordAnchor send: %w", err)
	}

	receipt, err := r.waitMined(ctx, tx.Hash())
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("registry: wait receipt %s: %w", tx.Hash(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash().Hex(), fmt.Errorf("registry: recordAnchor reverted (tx %s)", tx.Hash())
	}
	return tx.Hash().Hex(), nil
}

func (r *Recorder) waitMined(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	backend, ok := r.backend.(WaitMinedBackend)
	if !ok {
		return nil, fmt.Errorf("registry: backend cannot fetch receipts")
	}
	ctx, cancel := context.WithTimeout(ctx, r.receiptTimeout)
	defer cancel()
	ticker := time.NewTicker(r.receiptPoll)
	defer ticker.Stop()
	for {
		receipt, err := backend.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
