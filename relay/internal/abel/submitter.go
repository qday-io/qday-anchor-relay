package abel

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// SpendableCoin is what the submitter needs from the coin store.
type SpendableCoin struct {
	CoinID        string
	TxHash        string
	OutIndex      uint8
	ValueNeutrino int64
	BlockHeight   int64
}

// CoinSelector locks a coin for a job / releases it on pre-broadcast failure.
type CoinSelector interface {
	SelectAndLockCoin(ctx context.Context, jobID int64, minValue int64) (*SpendableCoin, error)
	ReleaseCoin(ctx context.Context, coinID string) error
	MarkCoinSpent(ctx context.Context, coinID string, jobID int64) error
	// CoinsLockedByJob lists coins left LOCKED for the job by a previous
	// (crashed) attempt — Submit reconciles them before doing anything else.
	CoinsLockedByJob(ctx context.Context, jobID int64) ([]*SpendableCoin, error)
}

// PersistTxID stores the locally computed txid on the job BEFORE broadcast so
// a crash in the send window cannot cause a double-send (rule 6.1-2).
type PersistTxID func(ctx context.Context, txHash string) error

// ErrNoSpendableCoin mirrors the store sentinel without importing it.
var ErrNoSpendableCoin = errors.New("abel: no spendable coin available")

// ErrRingNotReady means the input coin's ring block group is not complete yet;
// retry on a later tick.
var ErrRingNotReady = errors.New("abel: coin ring block group incomplete, retry later")

// RingProvider rebuilds the TxoRing containing a coin. Interface so the
// submitter and scanner are unit-testable; production uses NewSDKRingProvider.
type RingProvider interface {
	RingFor(txID string, outIndex uint8, blockHeight int64) (*abelian.CoinRing, error)
}

// TxCrafter isolates the SDK's crypto-heavy generate/sign pipeline so the
// submitter is unit-testable. The production implementation calls
// GenerateUnsignedRawTxWithRing + GenerateSignedRawTx (verified F6 flow).
type TxCrafter interface {
	Craft(txDesc *abelian.TxDescWithRing, signer abelian.Account) (signedRawTxHex string, txID string, err error)
}

type sdkCrafter struct{}

func (sdkCrafter) Craft(txDesc *abelian.TxDescWithRing, signer abelian.Account) (string, string, error) {
	unsigned, err := abelian.GenerateUnsignedRawTxWithRing(txDesc)
	if err != nil {
		return "", "", fmt.Errorf("abel: generate unsigned tx: %w", err)
	}
	signed, err := abelian.GenerateSignedRawTx(unsigned, []abelian.Account{signer})
	if err != nil {
		return "", "", fmt.Errorf("abel: sign tx: %w", err)
	}
	return hex.EncodeToString(signed.Data), signed.TxID, nil
}

// NewSDKCrafter returns the production TxCrafter.
func NewSDKCrafter() TxCrafter { return sdkCrafter{} }

// SubmitResult reports a broadcast (or re-discovered) anchor transaction.
type SubmitResult struct {
	TxHash  string
	Resumed bool // true when an already-broadcast tx was found (no resend)
}

type Submitter struct {
	client  Client
	account abelian.Account
	coins   CoinSelector
	rings   RingProvider
	crafter TxCrafter
	fee     int64
	log     *slog.Logger
}

func NewSubmitter(client Client, account abelian.Account, coins CoinSelector, rings RingProvider, crafter TxCrafter, feeNeutrino int64, log *slog.Logger) *Submitter {
	return &Submitter{client: client, account: account, coins: coins, rings: rings, crafter: crafter, fee: feeNeutrino, log: log}
}

// Submit broadcasts the anchor memo transaction for a job (spec §7.2).
// existingTxHash implements idempotency rule 6.1-2: when the job already has a
// tx hash, the chain is consulted before any resend. persistTxID is called
// with the locally computed txid before broadcast.
func (s *Submitter) Submit(ctx context.Context, jobID int64, memo []byte, existingTxHash string, persistTxID PersistTxID) (SubmitResult, error) {
	if existingTxHash != "" {
		if tx, err := s.client.RawTx(existingTxHash); err == nil && tx != nil {
			// Tx exists — a previous attempt broadcast it. Any coins still
			// LOCKED for this job funded that tx: finalize them as SPENT.
			if err := s.reconcileLockedCoins(ctx, jobID, true); err != nil {
				s.log.Error("abel: reconcile locked coins", "job", jobID, "err", err)
			}
			return SubmitResult{TxHash: existingTxHash, Resumed: true}, nil
		}
		// Not on chain and not in mempool → fall through and resend with the
		// same memo (payload identical by construction).
	}
	// A crashed attempt may have left coins LOCKED without a broadcast —
	// return them to the pool before locking a new one.
	if err := s.reconcileLockedCoins(ctx, jobID, false); err != nil {
		return SubmitResult{}, err
	}

	// Coin must cover fee plus a non-zero change output (a zero-output tx is
	// not a valid transfer shape in the SDK examples).
	coin, err := s.coins.SelectAndLockCoin(ctx, jobID, s.fee+1)
	if err != nil {
		return SubmitResult{}, err
	}
	// Any failure before broadcast returns the coin to the pool.
	broadcast := false
	defer func() {
		if !broadcast {
			if rerr := s.coins.ReleaseCoin(context.WithoutCancel(ctx), coin.CoinID); rerr != nil {
				s.log.Error("abel: release coin failed", "coin", coin.CoinID, "err", rerr)
			}
		}
	}()

	txInDesc, err := s.buildInput(coin)
	if err != nil {
		return SubmitResult{}, err
	}

	changeAddrBytes, err := s.account.GenerateAbelAddress()
	if err != nil {
		return SubmitResult{}, fmt.Errorf("abel: generate change address: %w", err)
	}
	changeAddr, err := abelian.NewAbelAddress(changeAddrBytes)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("abel: change address: %w", err)
	}

	txInDescs := []*abelian.TxInDescWithRing{txInDesc}
	txOutDescs := []*abelian.TxOutDesc{{AbelAddress: changeAddr, CoinValue: coin.ValueNeutrino - s.fee}}
	// The relay always builds exactly one input and one change output, so the
	// SDK's Sort* calls (spec §7.2 step 2) are identity operations; they are
	// applied only when a future change introduces multiple descriptors.
	// (GenerateUnsignedRawTxWithRing still validates every descriptor.)
	if len(txInDescs) > 1 {
		if err := abelian.SortTxInDescWithRing(txInDescs); err != nil {
			return SubmitResult{}, fmt.Errorf("abel: sort inputs: %w", err)
		}
	}
	if len(txOutDescs) > 1 {
		if err := abelian.SortTxOutDesc(txOutDescs); err != nil {
			return SubmitResult{}, fmt.Errorf("abel: sort outputs: %w", err)
		}
	}

	txDesc := abelian.NewTxDescWithRing(txInDescs, txOutDescs, s.fee)
	// Verified F1: constructor has no memo parameter; assign the field.
	txDesc.TxMemo = memo

	rawHex, txID, err := s.crafter.Craft(txDesc, s.account)
	if err != nil {
		return SubmitResult{}, err
	}

	// Persist the txid BEFORE broadcast: if we crash inside the send window,
	// the next attempt finds the hash and resumes instead of double-sending.
	if persistTxID != nil {
		if err := persistTxID(ctx, txID); err != nil {
			return SubmitResult{}, fmt.Errorf("abel: persist txid before broadcast: %w", err)
		}
	}

	sentID, err := s.client.SendSignedTx(rawHex)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("abel: send raw tx: %w", err)
	}
	broadcast = true
	if sentID != txID {
		s.log.Warn("abel: node returned different txid", "local", txID, "node", sentID)
	}
	// WithoutCancel: the tx is on the network; a shutdown signal must not
	// leave the coin LOCKED-but-spent.
	if err := s.coins.MarkCoinSpent(context.WithoutCancel(ctx), coin.CoinID, jobID); err != nil {
		// Tx is out; the coin row lags. The next Submit for this job resumes
		// via the persisted hash and reconcileLockedCoins finalizes the coin.
		s.log.Error("abel: mark coin spent failed", "coin", coin.CoinID, "err", err)
	}
	return SubmitResult{TxHash: sentID}, nil
}

// reconcileLockedCoins cleans up coins left LOCKED by a crashed attempt:
// spent=true finalizes them as SPENT (their tx is on chain), spent=false
// releases them back to the pool.
func (s *Submitter) reconcileLockedCoins(ctx context.Context, jobID int64, spent bool) error {
	locked, err := s.coins.CoinsLockedByJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("abel: locked coins for job %d: %w", jobID, err)
	}
	for _, c := range locked {
		if spent {
			err = s.coins.MarkCoinSpent(ctx, c.CoinID, jobID)
		} else {
			err = s.coins.ReleaseCoin(ctx, c.CoinID)
		}
		if err != nil {
			return fmt.Errorf("abel: reconcile coin %s: %w", c.CoinID, err)
		}
		s.log.Info("abel: reconciled stale locked coin", "coin", c.CoinID, "job", jobID, "spent", spent)
	}
	return nil
}

// buildInput reconstructs everything the SDK needs to spend a coin from chain
// data (the spec schema stores no vout/ring blobs — stateless rebuild).
func (s *Submitter) buildInput(coin *SpendableCoin) (*abelian.TxInDescWithRing, error) {
	tx, err := s.client.RawTx(coin.TxHash)
	if err != nil {
		return nil, fmt.Errorf("abel: source tx %s: %w", coin.TxHash, err)
	}
	if int(coin.OutIndex) >= len(tx.Vout) {
		return nil, fmt.Errorf("abel: coin %s: out index %d beyond %d vouts", coin.CoinID, coin.OutIndex, len(tx.Vout))
	}
	txOutData, err := hex.DecodeString(tx.Vout[coin.OutIndex].Script)
	if err != nil {
		return nil, fmt.Errorf("abel: coin %s vout script: %w", coin.CoinID, err)
	}
	block, err := s.client.BlockByHeight(coin.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("abel: coin block %d: %w", coin.BlockHeight, err)
	}

	ring, err := s.rings.RingFor(coin.TxHash, coin.OutIndex, coin.BlockHeight)
	if err != nil {
		return nil, err
	}

	return &abelian.TxInDescWithRing{
		BlockHeight: coin.BlockHeight,
		BlockID:     block.BlockHash,
		TxVersion:   tx.Version,
		TxID:        coin.TxHash,
		TxOutIndex:  coin.OutIndex,
		TxOutData:   txOutData,
		CoinValue:   coin.ValueNeutrino,
		TxoRing:     ring,
	}, nil
}

// sdkRingProvider rebuilds the coin's TxoRing from its ring block group
// (verified flow: GetRingBlockGroupByHeight → BuildCoinRings → match CoinID).
type sdkRingProvider struct {
	client Client
}

func NewSDKRingProvider(client Client) RingProvider { return &sdkRingProvider{client: client} }

func (p *sdkRingProvider) RingFor(txID string, outIndex uint8, blockHeight int64) (*abelian.CoinRing, error) {
	// The ring group only exists once all its blocks are on chain.
	heights := abelian.GetRingBlockHeights(blockHeight)
	tip, err := p.client.ChainHeight()
	if err != nil {
		return nil, fmt.Errorf("abel: chain height: %w", err)
	}
	for _, h := range heights {
		if h > tip {
			return nil, fmt.Errorf("%w (needs block %d, tip %d)", ErrRingNotReady, h, tip)
		}
	}

	serializedBlocks, err := p.client.RingBlockGroup(blockHeight)
	if err != nil {
		return nil, fmt.Errorf("abel: ring block group for height %d: %w", blockHeight, err)
	}
	rings, err := abelian.BuildCoinRings(serializedBlocks)
	if err != nil {
		return nil, fmt.Errorf("abel: build coin rings: %w", err)
	}
	want := abelian.NewCoinID(txID, outIndex)
	for _, ring := range rings {
		for _, id := range ring.CoinIDRing.CoinIDs {
			if id.TxID == want.TxID && id.Index == want.Index {
				return ring, nil
			}
		}
	}
	return nil, fmt.Errorf("abel: coin %s not found in its ring group", want.String())
}
