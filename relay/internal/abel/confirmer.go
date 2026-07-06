package abel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// isRPCNotFound reports whether the error is a JSON-RPC level error (the node
// answered — e.g. "no information available about transaction") rather than a
// transport failure. abec wraps RPC errors as abelian.RPCError.
func isRPCNotFound(err error) bool {
	var rpcErr abelian.RPCError
	if errors.As(err, &rpcErr) {
		return true
	}
	var rpcErrPtr *abelian.RPCError
	return errors.As(err, &rpcErrPtr)
}

// ConfirmStatus is the confirmer's verdict on a submitted anchor tx.
type ConfirmStatus int

const (
	// ConfirmPending: tx known (mempool or shallow block), keep waiting.
	ConfirmPending ConfirmStatus = iota
	// Confirmed: tx buried ≥ confirmDepth blocks.
	Confirmed
	// ConfirmLost: tx gone from chain AND mempool past the grace window —
	// roll the job back for resubmission (rule 6.1-2).
	ConfirmLost
)

type ConfirmResult struct {
	Status        ConfirmStatus
	AbelHeight    int64
	Confirmations int64
}

type Confirmer struct {
	client       Client
	confirmDepth int64
	// lostGrace: how long a tx may be missing from both chain and mempool
	// before we declare it lost and resubmit.
	lostGrace time.Duration
	log       *slog.Logger
}

func NewConfirmer(client Client, confirmDepth int64, lostGrace time.Duration, log *slog.Logger) *Confirmer {
	if lostGrace <= 0 {
		lostGrace = 30 * time.Minute
	}
	return &Confirmer{client: client, confirmDepth: confirmDepth, lostGrace: lostGrace, log: log}
}

// Check inspects one submitted tx (spec §7.3). submittedAt is the job's last
// status-change time, used for the lost-tx grace window.
//
// NOTE (verified SDK bug): Tx.Confirmations carries a `bson` tag instead of
// `json`, so it never unmarshals from the RPC response. Confirmations are
// derived from the enclosing block instead (docs/sdk-verified-api.md).
func (c *Confirmer) Check(ctx context.Context, abelTxHash string, submittedAt time.Time) (ConfirmResult, error) {
	tx, err := c.client.RawTx(abelTxHash)
	if err != nil && !isRPCNotFound(err) {
		// Transport/auth/node failure — NOT evidence the tx is gone. Treating
		// it as lost would trigger a duplicate resubmit; retry next tick.
		return ConfirmResult{}, fmt.Errorf("abel: get tx %s: %w", abelTxHash, err)
	}
	if err != nil || tx == nil {
		return c.handleUnknownTx(abelTxHash, submittedAt, err)
	}

	if tx.BlockHash == "" {
		// Still in mempool.
		return ConfirmResult{Status: ConfirmPending}, nil
	}

	confs, height, err := c.blockConfirmations(tx.BlockHash)
	if err != nil {
		return ConfirmResult{}, err
	}
	if confs >= c.confirmDepth {
		return ConfirmResult{Status: Confirmed, AbelHeight: height, Confirmations: confs}, nil
	}
	return ConfirmResult{Status: ConfirmPending, AbelHeight: height, Confirmations: confs}, nil
}

func (c *Confirmer) handleUnknownTx(abelTxHash string, submittedAt time.Time, cause error) (ConfirmResult, error) {
	ids, err := c.client.MempoolTxIDs()
	if err != nil {
		return ConfirmResult{}, fmt.Errorf("abel: mempool check: %w", err)
	}
	for _, id := range ids {
		if id == abelTxHash {
			return ConfirmResult{Status: ConfirmPending}, nil
		}
	}
	if time.Since(submittedAt) < c.lostGrace {
		c.log.Warn("abel: tx not visible yet", "tx", abelTxHash, "cause", cause)
		return ConfirmResult{Status: ConfirmPending}, nil
	}
	c.log.Error("abel: tx lost, scheduling resubmit", "tx", abelTxHash)
	return ConfirmResult{Status: ConfirmLost}, nil
}

// blockConfirmations returns (confirmations, blockHeight) for a block hash.
// Prefers the node-reported Block.Confirmations (json tag is correct on
// Block); falls back to tip - height + 1.
func (c *Confirmer) blockConfirmations(blockHash string) (int64, int64, error) {
	block, err := c.client.Block(blockHash)
	if err != nil {
		return 0, 0, fmt.Errorf("abel: block %s: %w", blockHash, err)
	}
	if block.Confirmations > 0 {
		return block.Confirmations, block.Height, nil
	}
	tip, err := c.client.ChainHeight()
	if err != nil {
		return 0, 0, fmt.Errorf("abel: chain height: %w", err)
	}
	return tip - block.Height + 1, block.Height, nil
}
