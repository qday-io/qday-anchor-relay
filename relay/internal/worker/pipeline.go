package worker

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
	"github.com/qday/qday-anchor-relay/relay/internal/anchor"
	"github.com/qday/qday-anchor-relay/relay/internal/registry"
	"github.com/qday/qday-anchor-relay/relay/internal/store"
)

// JobStore is the slice of store.Store the pipeline needs (fake-able).
type JobStore interface {
	RetryCandidates(ctx context.Context, status store.Status, baseSeconds int, limit int) ([]*store.Job, error)
	TransitionJob(ctx context.Context, jobID int64, from, to store.Status, patch store.JobPatch) error
	FailOrRetry(ctx context.Context, job *store.Job, retryMax int, cause error) error
	// SetJobAbelTxHash persists the pre-broadcast txid (rule 6.1-2).
	SetJobAbelTxHash(ctx context.Context, jobID int64, txHash string) error
	// ResurrectCoinsSpentByJob returns a lost tx's coins to the pool.
	ResurrectCoinsSpentByJob(ctx context.Context, jobID int64) error
}

// AnchorSubmitter abstracts abel.Submitter.
type AnchorSubmitter interface {
	Submit(ctx context.Context, jobID int64, memo []byte, existingTxHash string, persistTxID abel.PersistTxID) (abel.SubmitResult, error)
}

// AnchorConfirmer abstracts abel.Confirmer.
type AnchorConfirmer interface {
	Check(ctx context.Context, abelTxHash string, submittedAt time.Time) (abel.ConfirmResult, error)
}

// AnchorRecorder abstracts registry.Recorder.
type AnchorRecorder interface {
	IsAnchored(ctx context.Context, stateRoot [32]byte) (bool, error)
	Record(ctx context.Context, rec registry.AnchorRecord) (string, error)
}

// Pipeline advances anchor jobs through the state machine (spec §6.1).
type Pipeline struct {
	store       JobStore
	submitter   AnchorSubmitter
	confirmer   AnchorConfirmer
	recorder    AnchorRecorder
	retryMax    int
	backoffBase int // seconds
	batchSize   int
	log         *slog.Logger
}

func NewPipeline(s JobStore, sub AnchorSubmitter, conf AnchorConfirmer, rec AnchorRecorder,
	retryMax int, backoffBaseSeconds int, log *slog.Logger) *Pipeline {
	return &Pipeline{
		store: s, submitter: sub, confirmer: conf, recorder: rec,
		retryMax: retryMax, backoffBase: backoffBaseSeconds, batchSize: 20, log: log,
	}
}

// SubmitTick moves PENDING jobs to ABEL_SUBMITTED.
func (p *Pipeline) SubmitTick(ctx context.Context) error {
	jobs, err := p.store.RetryCandidates(ctx, store.StatusPending, p.backoffBase, p.batchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		memo := anchor.EncodeMemo(job.BatchNumber, job.StateRoot)
		persist := func(pctx context.Context, txHash string) error {
			return p.store.SetJobAbelTxHash(pctx, job.ID, txHash)
		}
		res, err := p.submitter.Submit(ctx, job.ID, memo, job.AbelTxHash, persist)
		if err != nil {
			// Environmental waits (empty wallet / immature ring group) are
			// normal timing conditions — do not burn the retry budget.
			if errors.Is(err, abel.ErrNoSpendableCoin) || errors.Is(err, abel.ErrRingNotReady) {
				p.log.Info("pipeline: submit waiting", "job", job.ID, "reason", err)
				continue
			}
			p.failOrRetry(ctx, job, err)
			continue
		}
		patch := store.JobPatch{
			AbelTxHash: &res.TxHash,
			Detail:     map[string]any{"memo": string(memo), "resumed": res.Resumed},
		}
		if err := p.store.TransitionJob(ctx, job.ID, store.StatusPending, store.StatusAbelSubmitted, patch); err != nil {
			p.log.Error("pipeline: submit transition failed", "job", job.ID, "err", err)
		}
	}
	return nil
}

// ConfirmTick moves ABEL_SUBMITTED jobs to ABEL_CONFIRMED (or back to PENDING
// when the tx was lost — rule 6.1-2).
func (p *Pipeline) ConfirmTick(ctx context.Context) error {
	jobs, err := p.store.RetryCandidates(ctx, store.StatusAbelSubmitted, 0, p.batchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if job.AbelTxHash == "" {
			p.failOrRetry(ctx, job, fmt.Errorf("job in ABEL_SUBMITTED without abel_tx_hash"))
			continue
		}
		res, err := p.confirmer.Check(ctx, job.AbelTxHash, job.UpdatedAt)
		if err != nil {
			p.failOrRetry(ctx, job, err)
			continue
		}
		switch res.Status {
		case abel.Confirmed:
			patch := store.JobPatch{
				AbelHeight: &res.AbelHeight,
				Detail:     map[string]any{"confirmations": res.Confirmations},
			}
			if err := p.store.TransitionJob(ctx, job.ID, store.StatusAbelSubmitted, store.StatusAbelConfirmed, patch); err != nil {
				p.log.Error("pipeline: confirm transition failed", "job", job.ID, "err", err)
			}
		case abel.ConfirmLost:
			patch := store.JobPatch{Detail: map[string]any{"reason": "abel tx lost, resubmitting", "lost_tx": job.AbelTxHash}}
			if err := p.store.TransitionJob(ctx, job.ID, store.StatusAbelSubmitted, store.StatusPending, patch); err != nil {
				p.log.Error("pipeline: rollback transition failed", "job", job.ID, "err", err)
				continue
			}
			// The lost tx never consumed its coins on chain — return them.
			if err := p.store.ResurrectCoinsSpentByJob(ctx, job.ID); err != nil {
				p.log.Error("pipeline: coin resurrection failed", "job", job.ID, "err", err)
			}
		}
	}
	return nil
}

// RecordTick moves ABEL_CONFIRMED jobs through CONTRACT_RECORDED to DONE.
// It also sweeps jobs stranded in CONTRACT_RECORDED by a crash between the two
// transitions (rule 6.1-1: every non-terminal status must resume).
func (p *Pipeline) RecordTick(ctx context.Context) error {
	stranded, err := p.store.RetryCandidates(ctx, store.StatusContractRecorded, 0, p.batchSize)
	if err != nil {
		return err
	}
	for _, job := range stranded {
		if err := p.store.TransitionJob(ctx, job.ID, store.StatusContractRecorded, store.StatusDone, store.JobPatch{}); err != nil {
			p.log.Error("pipeline: stranded-done transition failed", "job", job.ID, "err", err)
		}
	}

	jobs, err := p.store.RetryCandidates(ctx, store.StatusAbelConfirmed, p.backoffBase, p.batchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.recordJob(ctx, job)
	}
	return nil
}

func (p *Pipeline) recordJob(ctx context.Context, job *store.Job) {
	// Idempotency rule 6.1-3: consult the contract before writing.
	anchored, err := p.recorder.IsAnchored(ctx, job.StateRoot)
	if err != nil {
		p.failOrRetry(ctx, job, err)
		return
	}

	var recordTxHash string
	detail := map[string]any{}
	if anchored {
		detail["reason"] = "already anchored on contract, advancing without tx"
	} else {
		abelTxHash, err := abelTxHashTo32(job.AbelTxHash)
		if err != nil {
			p.failOrRetry(ctx, job, err)
			return
		}
		recordTxHash, err = p.recorder.Record(ctx, registry.AnchorRecord{
			BatchNumber: job.BatchNumber,
			StateRoot:   job.StateRoot,
			AbelTxHash:  abelTxHash,
			AbelHeight:  uint64(job.AbelHeight),
		})
		if err != nil {
			// A revert may mean "someone recorded it meanwhile" — re-check.
			if again, cerr := p.recorder.IsAnchored(ctx, job.StateRoot); cerr == nil && again {
				detail["reason"] = "recordAnchor reverted but root is anchored, advancing"
				detail["reverted_tx"] = recordTxHash
				recordTxHash = "" // never store a reverted tx as the anchor record
			} else {
				p.failOrRetry(ctx, job, err)
				return
			}
		}
		if recordTxHash != "" {
			detail["record_tx"] = recordTxHash
		}
	}

	patch := store.JobPatch{Detail: detail}
	if recordTxHash != "" {
		patch.RecordTxHash = &recordTxHash
	}
	if err := p.store.TransitionJob(ctx, job.ID, store.StatusAbelConfirmed, store.StatusContractRecorded, patch); err != nil {
		p.log.Error("pipeline: record transition failed", "job", job.ID, "err", err)
		return
	}
	if err := p.store.TransitionJob(ctx, job.ID, store.StatusContractRecorded, store.StatusDone, store.JobPatch{}); err != nil {
		p.log.Error("pipeline: done transition failed", "job", job.ID, "err", err)
	}
}

func (p *Pipeline) failOrRetry(ctx context.Context, job *store.Job, cause error) {
	p.log.Warn("pipeline: job step failed", "job", job.ID, "status", job.Status, "err", cause)
	if err := p.store.FailOrRetry(ctx, job, p.retryMax, cause); err != nil {
		p.log.Error("pipeline: FailOrRetry failed", "job", job.ID, "err", err)
	}
}

// abelTxHashTo32 converts a 64-hex Abelian tx hash into bytes32 for the
// contract (spec §8 備註).
func abelTxHashTo32(h string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 32 {
		return out, fmt.Errorf("worker: abel tx hash %q is not 32-byte hex (err=%v)", h, err)
	}
	copy(out[:], b)
	return out, nil
}
