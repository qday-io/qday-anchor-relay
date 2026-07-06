package worker

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
	"github.com/qday/qday-anchor-relay/relay/internal/registry"
	"github.com/qday/qday-anchor-relay/relay/internal/store"
)

// abelHash64 is a valid 64-hex abel tx hash used across tests.
var abelHash64 = strings.Repeat("ab", 32)

type fakeJobStore struct {
	jobs            map[store.Status][]*store.Job
	transitions     []string // "from>to"
	failures        []int64
	patches         map[int64]store.JobPatch
	persistedHashes []string
	resurrected     []int64
}

func newFakeJobStore(jobs ...*store.Job) *fakeJobStore {
	f := &fakeJobStore{jobs: map[store.Status][]*store.Job{}, patches: map[int64]store.JobPatch{}}
	for _, j := range jobs {
		f.jobs[j.Status] = append(f.jobs[j.Status], j)
	}
	return f
}

func (f *fakeJobStore) RetryCandidates(ctx context.Context, status store.Status, base, limit int) ([]*store.Job, error) {
	return f.jobs[status], nil
}

func (f *fakeJobStore) TransitionJob(ctx context.Context, jobID int64, from, to store.Status, patch store.JobPatch) error {
	if !store.CanTransition(from, to) {
		return store.ErrIllegalTransition
	}
	f.transitions = append(f.transitions, transitionKey(jobID, from, to))
	f.patches[jobID] = patch
	return nil
}

func (f *fakeJobStore) FailOrRetry(ctx context.Context, job *store.Job, retryMax int, cause error) error {
	f.failures = append(f.failures, job.ID)
	return nil
}

func (f *fakeJobStore) SetJobAbelTxHash(ctx context.Context, jobID int64, txHash string) error {
	f.persistedHashes = append(f.persistedHashes, txHash)
	return nil
}

func (f *fakeJobStore) ResurrectCoinsSpentByJob(ctx context.Context, jobID int64) error {
	f.resurrected = append(f.resurrected, jobID)
	return nil
}

func transitionKey(id int64, from, to store.Status) string {
	return string(from) + ">" + string(to)
}

type fakeSubmit struct {
	res  abel.SubmitResult
	err  error
	got  []byte // memo
	hash string // existing hash passed
}

func (f *fakeSubmit) Submit(ctx context.Context, jobID int64, memo []byte, existing string, persist abel.PersistTxID) (abel.SubmitResult, error) {
	f.got, f.hash = memo, existing
	if f.err == nil && persist != nil {
		if err := persist(ctx, f.res.TxHash); err != nil {
			return abel.SubmitResult{}, err
		}
	}
	return f.res, f.err
}

type fakeConfirm struct {
	res abel.ConfirmResult
	err error
}

func (f *fakeConfirm) Check(ctx context.Context, h string, at time.Time) (abel.ConfirmResult, error) {
	return f.res, f.err
}

type fakeRecord struct {
	anchored          bool
	anchoredAfterFail bool
	recordHash        string
	recordErr         error
	recorded          []registry.AnchorRecord
}

func (f *fakeRecord) IsAnchored(ctx context.Context, root [32]byte) (bool, error) {
	return f.anchored, nil
}

func (f *fakeRecord) Record(ctx context.Context, rec registry.AnchorRecord) (string, error) {
	if f.recordErr != nil {
		f.anchored = f.anchoredAfterFail // simulate concurrent recording
		return "", f.recordErr
	}
	f.recorded = append(f.recorded, rec)
	return f.recordHash, nil
}

func job(id int64, status store.Status) *store.Job {
	var root [32]byte
	root[0] = byte(id)
	return &store.Job{ID: id, BatchNumber: uint64(id), StateRoot: root, Status: status, UpdatedAt: time.Now()}
}

func newPipeline(s JobStore, sub AnchorSubmitter, c AnchorConfirmer, r AnchorRecorder) *Pipeline {
	return NewPipeline(s, sub, c, r, 5, 30, testLog)
}

func TestSubmitTick_HappyPath(t *testing.T) {
	j := job(1, store.StatusPending)
	fs := newFakeJobStore(j)
	sub := &fakeSubmit{res: abel.SubmitResult{TxHash: abelHash64}}
	p := newPipeline(fs, sub, &fakeConfirm{}, &fakeRecord{})

	if err := p.SubmitTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 1 || fs.transitions[0] != "PENDING>ABEL_SUBMITTED" {
		t.Errorf("transitions = %v", fs.transitions)
	}
	// Memo must be the canonical encoding for the job.
	wantPrefix := "QDAY2ANCHOR|v1|1|0x"
	if !strings.HasPrefix(string(sub.got), wantPrefix) {
		t.Errorf("memo = %q", sub.got)
	}
	if *fs.patches[1].AbelTxHash != abelHash64 {
		t.Errorf("patch = %+v", fs.patches[1])
	}
}

func TestSubmitTick_PassesExistingHashForIdempotency(t *testing.T) {
	j := job(2, store.StatusPending)
	j.AbelTxHash = "prior-hash"
	fs := newFakeJobStore(j)
	sub := &fakeSubmit{res: abel.SubmitResult{TxHash: "prior-hash", Resumed: true}}
	p := newPipeline(fs, sub, &fakeConfirm{}, &fakeRecord{})

	if err := p.SubmitTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sub.hash != "prior-hash" {
		t.Errorf("existing hash passed = %q", sub.hash)
	}
}

func TestSubmitTick_FailureCountsRetry(t *testing.T) {
	fs := newFakeJobStore(job(3, store.StatusPending))
	p := newPipeline(fs, &fakeSubmit{err: errors.New("no coins")}, &fakeConfirm{}, &fakeRecord{})

	if err := p.SubmitTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.failures) != 1 || len(fs.transitions) != 0 {
		t.Errorf("failures=%v transitions=%v", fs.failures, fs.transitions)
	}
}

func TestConfirmTick_Confirmed(t *testing.T) {
	j := job(4, store.StatusAbelSubmitted)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{res: abel.ConfirmResult{Status: abel.Confirmed, AbelHeight: 777, Confirmations: 6}}, &fakeRecord{})

	if err := p.ConfirmTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 1 || fs.transitions[0] != "ABEL_SUBMITTED>ABEL_CONFIRMED" {
		t.Errorf("transitions = %v", fs.transitions)
	}
	if *fs.patches[4].AbelHeight != 777 {
		t.Errorf("patch = %+v", fs.patches[4])
	}
}

func TestConfirmTick_LostRollsBackToPending(t *testing.T) {
	j := job(5, store.StatusAbelSubmitted)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{res: abel.ConfirmResult{Status: abel.ConfirmLost}}, &fakeRecord{})

	if err := p.ConfirmTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 1 || fs.transitions[0] != "ABEL_SUBMITTED>PENDING" {
		t.Errorf("transitions = %v", fs.transitions)
	}
}

func TestConfirmTick_PendingNoTransition(t *testing.T) {
	j := job(6, store.StatusAbelSubmitted)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{res: abel.ConfirmResult{Status: abel.ConfirmPending}}, &fakeRecord{})

	if err := p.ConfirmTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 0 {
		t.Errorf("transitions = %v", fs.transitions)
	}
}

func TestRecordTick_RecordsAndFinishes(t *testing.T) {
	j := job(7, store.StatusAbelConfirmed)
	j.AbelTxHash = abelHash64
	j.AbelHeight = 999
	fs := newFakeJobStore(j)
	rec := &fakeRecord{recordHash: "0xrecordtx"}
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, rec)

	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"ABEL_CONFIRMED>CONTRACT_RECORDED", "CONTRACT_RECORDED>DONE"}
	if len(fs.transitions) != 2 || fs.transitions[0] != want[0] || fs.transitions[1] != want[1] {
		t.Errorf("transitions = %v", fs.transitions)
	}
	if len(rec.recorded) != 1 {
		t.Fatalf("recorded = %+v", rec.recorded)
	}
	r := rec.recorded[0]
	wantHash, _ := hex.DecodeString(abelHash64)
	if r.BatchNumber != 7 || r.AbelHeight != 999 || !strings.EqualFold(hex.EncodeToString(r.AbelTxHash[:]), hex.EncodeToString(wantHash)) {
		t.Errorf("record = %+v", r)
	}
}

func TestRecordTick_AlreadyAnchoredSkipsWrite(t *testing.T) {
	j := job(8, store.StatusAbelConfirmed)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	rec := &fakeRecord{anchored: true}
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, rec)

	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rec.recorded) != 0 {
		t.Error("must not send recordAnchor when already anchored")
	}
	if len(fs.transitions) != 2 {
		t.Errorf("transitions = %v", fs.transitions)
	}
}

func TestRecordTick_RevertThenAnchoredAdvances(t *testing.T) {
	j := job(9, store.StatusAbelConfirmed)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	rec := &fakeRecord{recordErr: errors.New("execution reverted"), anchoredAfterFail: true}
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, rec)

	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 2 || len(fs.failures) != 0 {
		t.Errorf("transitions=%v failures=%v", fs.transitions, fs.failures)
	}
}

func TestRecordTick_RealFailureRetries(t *testing.T) {
	j := job(10, store.StatusAbelConfirmed)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	rec := &fakeRecord{recordErr: errors.New("rpc down")}
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, rec)

	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.failures) != 1 || len(fs.transitions) != 0 {
		t.Errorf("transitions=%v failures=%v", fs.transitions, fs.failures)
	}
}

func TestRecordTick_BadAbelHashFails(t *testing.T) {
	j := job(11, store.StatusAbelConfirmed)
	j.AbelTxHash = "not-hex"
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, &fakeRecord{})

	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.failures) != 1 {
		t.Errorf("failures = %v", fs.failures)
	}
}

func TestSubmitTick_WaitErrorsDoNotBurnRetries(t *testing.T) {
	for _, waitErr := range []error{abel.ErrNoSpendableCoin, abel.ErrRingNotReady} {
		fs := newFakeJobStore(job(20, store.StatusPending))
		p := newPipeline(fs, &fakeSubmit{err: waitErr}, &fakeConfirm{}, &fakeRecord{})
		if err := p.SubmitTick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(fs.failures) != 0 || len(fs.transitions) != 0 {
			t.Errorf("%v: failures=%v transitions=%v (want none)", waitErr, fs.failures, fs.transitions)
		}
	}
}

func TestSubmitTick_PersistsHashBeforeBroadcast(t *testing.T) {
	fs := newFakeJobStore(job(21, store.StatusPending))
	p := newPipeline(fs, &fakeSubmit{res: abel.SubmitResult{TxHash: abelHash64}}, &fakeConfirm{}, &fakeRecord{})
	if err := p.SubmitTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.persistedHashes) != 1 || fs.persistedHashes[0] != abelHash64 {
		t.Errorf("persisted = %v", fs.persistedHashes)
	}
}

func TestConfirmTick_LostResurrectsCoins(t *testing.T) {
	j := job(22, store.StatusAbelSubmitted)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{res: abel.ConfirmResult{Status: abel.ConfirmLost}}, &fakeRecord{})
	if err := p.ConfirmTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.resurrected) != 1 || fs.resurrected[0] != 22 {
		t.Errorf("resurrected = %v", fs.resurrected)
	}
}

func TestRecordTick_SweepsStrandedContractRecorded(t *testing.T) {
	j := job(23, store.StatusContractRecorded)
	fs := newFakeJobStore(j)
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, &fakeRecord{})
	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fs.transitions) != 1 || fs.transitions[0] != "CONTRACT_RECORDED>DONE" {
		t.Errorf("transitions = %v", fs.transitions)
	}
}

func TestRecordTick_RevertedTxNotStoredAsRecord(t *testing.T) {
	j := job(24, store.StatusAbelConfirmed)
	j.AbelTxHash = abelHash64
	fs := newFakeJobStore(j)
	rec := &fakeRecord{recordErr: errors.New("execution reverted"), anchoredAfterFail: true}
	p := newPipeline(fs, &fakeSubmit{}, &fakeConfirm{}, rec)
	if err := p.RecordTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if patch := fs.patches[24]; patch.RecordTxHash != nil {
		t.Errorf("reverted tx stored as record_tx_hash: %v", *patch.RecordTxHash)
	}
}
