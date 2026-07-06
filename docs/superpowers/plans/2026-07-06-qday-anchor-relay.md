# QDAY2 ↔ Abelian Anchor Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Anchor QDAY2 zkRollup state roots onto the Abelian chain via memo transactions, record the anchoring back into an EVM registry contract, with an admin portal.

**Architecture:** Single Go relay process (poller → submitter → confirmer → recorder workers over a Postgres-backed state machine), a Solidity `QDayAnchorRegistry` (Foundry), a Next.js 15 read-only portal. All in one monorepo, `docker compose up` bootable.

**Tech Stack:** Go 1.23 + `github.com/pqabelian/abelian-sdk-go-v2` + `github.com/ethereum/go-ethereum` + `pgx`; Foundry (solc 0.8.24); Next.js 15 + `iron-session` + `bcryptjs` + `pg`.

## Global Constraints

- Spec: `doc/qday-anchor-relay-spec.md` v1.0. All `【TO-CONFIRM】` items: default + config, listed in README; never hardcode guesses.
- **No invented SDK APIs.** Every abelian-sdk call must appear in `docs/sdk-verified-api.md` (source-extracted). Three verified SDK gotchas MUST be respected:
  1. `Tx.Confirmations` has a `bson` tag (not `json`) — never read it; compute confirmations via `GetBlock(tx.BlockHash).Confirmations` or `chainHeight - blockHeight + 1`.
  2. AIP-11 remap: `MnemonicsToCryptoSeed` returns `(spend, sn, detector, view)`; `NewRootSeedAccount` takes `(spend, sn, value=view, detector)`.
  3. `NewTxDescWithRing` has no memo param — assign `txDesc.TxMemo` after construction.
- Memo protocol (fixed): `QDAY2ANCHOR|v1|<batchNumber decimal>|0x<stateRoot 64 hex lowercase>`.
- State machine: `PENDING → ABEL_SUBMITTED → ABEL_CONFIRMED → CONTRACT_RECORDED → DONE`; failure → same status + `retry_count++` (≤ RETRY_MAX) → `FAILED`. Idempotency rules 6.1-1..5 of spec.
- DB unique: `batch_number`, `state_root`. Poller writes jobs + cursor in ONE tx (`ON CONFLICT DO NOTHING`).
- Contract source is given verbatim in spec §8 — copy it exactly.
- Only portal exposes a port in compose. Portal DB user = `portal_ro` (SELECT only).
- Commit after each passing task. TDD where a unit seam exists.

---

### Task 1: Scaffold (done inline)

docker-compose.yml, .env.example, Makefile, migrations 0001/0002, deploy/initdb/01-portal-ro.sh, .gitignore. Commit `chore: scaffold monorepo`.

### Task 2: contracts/ — QDayAnchorRegistry

**Files:**
- Create: `contracts/foundry.toml`, `contracts/src/QDayAnchorRegistry.sol` (verbatim spec §8), `contracts/test/QDayAnchorRegistry.t.sol`, `contracts/script/Deploy.s.sol`

**Interfaces produced:** `recordAnchor(uint64,bytes32,bytes32,uint64)`, `anchorOf(bytes32) returns (uint64,bytes32,uint64,uint64)`, `isAnchored(bytes32) returns (bool)`, `rootOfBatch(uint64)`, `setRelayer(address,bool)`, event `Anchored(uint64 indexed,bytes32 indexed,bytes32,uint64)`.

**Steps:**
- [ ] Write `foundry.toml` (solc ^0.8.24, src/test/script dirs).
- [ ] Copy contract verbatim from spec §8.
- [ ] Tests (write first, watch fail via missing file if strict TDD; contract is spec-given so test-after acceptable):
  - `test_RecordAnchor_StoresAndEmits` — expectEmit full event, then `anchorOf` returns all four fields, `isAnchored` true, `rootOfBatch` set.
  - `test_RecordAnchor_RevertNotRelayer` — unauthorized caller reverts `NotRelayer`.
  - `test_SetRelayer_OnlyOwner` — non-owner reverts `NotOwner`; owner sets and emits `RelayerSet`.
  - `test_RecordAnchor_RevertDuplicateRoot` — same stateRoot twice → `AlreadyAnchored`.
  - `test_RecordAnchor_RevertDuplicateBatch` — same batch, different root → `BatchAlreadyAnchored`.
- [ ] `Deploy.s.sol`: `vm.envAddress("RELAYER_ADDRESS")` (fallback broadcast sender), deploy, `setRelayer(relayer, true)`.
- [ ] Run `forge test -vv` → all pass. Commit `feat(contracts): QDayAnchorRegistry + tests + deploy script`.

### Task 3: relay/ — module, config, memo codec, store + state machine

**Files:**
- Create: `relay/go.mod` (go 1.23.2), `relay/internal/config/config.go` + `config_test.go`
- Create: `relay/internal/anchor/memo.go` + `memo_test.go` (golden vectors shared w/ portal)
- Create: `relay/internal/store/store.go`, `store/jobs.go`, `store/coins.go`, `store/cursor.go`, `store/statemachine.go` + `statemachine_test.go`
- Create: `relay/internal/store/db_test.go` (pgx integration tests, skipped unless `TEST_DATABASE_URL` set)

**Interfaces produced:**
- `config.Load() (*Config, error)` — env-driven, validates required fields.
- `anchor.EncodeMemo(batchNumber uint64, stateRoot [32]byte) []byte`
- `anchor.DecodeMemo(b []byte) (batchNumber uint64, stateRoot [32]byte, err error)`
- `store.Store` with `CreateJobsAndAdvanceCursor(ctx, commits []da.Commit, newCursor uint64) error`, `JobsByStatus`, `TransitionJob(ctx, jobID, from, to Status, patch JobPatch) error` (writes `job_events` in same tx), `FailOrRetry(ctx, job, err)`, coin CRUD (`SelectAndLockCoin`, `MarkCoinSpent`, `UpsertCoin`), `Cursor/SetCursor`.
- `statemachine.CanTransition(from, to Status) bool` — full allowed-matrix, unit-tested exhaustively.

**Memo golden vectors** (shared, also used by portal tests):

```
batch=1024 root=3fa8…(64 hex) → "QDAY2ANCHOR|v1|1024|0x<root>"
batch=0    root=000…0        → "QDAY2ANCHOR|v1|0|0x000…0"
decode rejects: wrong prefix, wrong version, 3 or 5 segments, root not 0x+64hex, uppercase hex ok? → NO (spec says lowercase; decode lowercases input root before compare? → decode accepts lowercase only, encode always emits lowercase)
```

**Steps:** TDD each unit (memo codec, statemachine matrix, retry/backoff calc). Backoff: `RETRY_BACKOFF_BASE_SECONDS * 2^retry_count`, capped. Store methods integration-tested behind `TEST_DATABASE_URL` guard. Commit per unit.

### Task 4: relay/ — registry recorder (abigen) + DA poller

**Files:**
- Create: `relay/internal/registry/qday_anchor_registry.go` (generated — `make abigen`), `registry/recorder.go` + `recorder_test.go`
- Create: `relay/internal/da/source.go` (Commit + CommitSource iface), `da/eventlog.go` (EventLogCommitSource), `da/poller.go` + `poller_test.go`

**Interfaces produced:**
- `da.Commit{BatchNumber uint64; StateRoot [32]byte; DABlock uint64; DATxHash string}`
- `da.CommitSource{FetchCommits(ctx, from, to uint64) ([]Commit, error); LatestBlock(ctx) (uint64, error)}`
- `registry.Recorder{Record(ctx, job) (txHash string, err error); IsAnchored(ctx, stateRoot [32]byte) (bool, error)}`

**Steps:**
- [ ] `make abigen` — forge build + abigen binding (go-ethereum abigen via `go run`).
- [ ] `EventLogCommitSource`: `ethclient` + `FilterLogs` on assumed event `BatchCommitted(uint64 indexed batchNumber, bytes32 stateRoot)`; topic from `abi.JSON` parse of embedded assumed ABI — NOT hand-written hash. File-level comment: TO-CONFIRM C1.
- [ ] Poller loop: `safeHead = LatestBlock() - DA_CONFIRM_DEPTH`; batch ranges (≤ 2000 blocks/query); write jobs + cursor in one DB tx. Unit test with fake CommitSource + fake store: cursor advances only with jobs; duplicate commits no-op.
- [ ] Recorder: idempotency — before send, `anchorOf(stateRoot)`; if recorded → advance without tx. Send via abigen binding, `SuggestGasPrice`, pending nonce, wait receipt w/ timeout; status=1 → advance; revert → re-check `anchorOf`. Unit test against simulated backend (`simulated.NewBackend`) with real generated binding.
- [ ] Commit per component.

### Task 5: relay/ — abel: wallet, client wrapper, coin scanner, submitter, confirmer

**Files:**
- Create: `relay/internal/abel/wallet.go` + `wallet_test.go` (loaders; key-file JSON: `{network_id, account_privacy_level, spend_key_seed, sn_key_seed, value_key_seed, detector_key}` hex — mirrors SDK example DB fields)
- Create: `relay/internal/abel/client.go` (thin interface over SDK client for mocking: `ChainHeight`, `BlockByHeight`, `RawTx`, `SendSignedTx`, `RingBlockGroup` …)
- Create: `relay/internal/abel/scanner.go` + `scanner_test.go` (coin discovery via `ReceiveCoin`, spent detection via serial numbers, maturity)
- Create: `relay/internal/abel/submitter.go` + `submitter_test.go`
- Create: `relay/internal/abel/confirmer.go` + `confirmer_test.go`

**Interfaces produced:**
- `abel.LoadWallet(cfg) (abelian.Account, error)` — mnemonic path (AIP-11 remap!) or key-file path.
- `abel.Submitter.Submit(ctx, job) (txHash string, err error)` — full §7.2 flow, memo set post-construction.
- `abel.Confirmer.Check(ctx, job) (confirmed bool, height int64, err error)` — confirmations from block, NOT `Tx.Confirmations` (bson bug). Mempool-gone + timeout → resubmit signal.

**Steps:** TDD against mocked client interface. Critical test cases:
- wallet: mnemonic remap wiring (view seed → value param), key-file JSON roundtrip.
- scanner: coinbase index-0 detection, maturity 200 gate, ring-group-completion gate for serial numbers, spent detection by serialNumber+outpoint.
- submitter: coin selection (smallest-first accumulate ≥ fee), change output = total − fee back to self (fresh `GenerateAbelAddress` fine — instability documented), sort ins/outs, memo bytes exact, idempotency: existing `abel_tx_hash` on job → `GetRawTx` first, skip resend if found.
- confirmer: confirmations math from chain height; mempool-disappeared handling → revert to PENDING-resubmit path (rule 6.1-2 compliant, memo unchanged).
- Commit per component.

### Task 6: relay/ — worker loop, main, Dockerfile

**Files:**
- Create: `relay/internal/worker/worker.go` + `worker_test.go` (generic ticker worker w/ panic recover), `relay/cmd/relay/main.go`, `relay/Dockerfile`

**Steps:**
- [ ] Worker: `Run(ctx)` per role; recover() logs panic, worker restarts after backoff; slog JSON handler.
- [ ] main: config → store → clients → wallet → start poller/scanner/submitter/confirmer/recorder goroutines; SIGTERM graceful.
- [ ] Startup idempotency sweep: scan non-DONE/FAILED jobs and dispatch by status (rules 6.1-1/2/3).
- [ ] Dockerfile: golang:1.23 builder, try `CGO_ENABLED=0`; record outcome in README (liboqs-go note).
- [ ] `go build ./... && go vet ./... && go test ./...` all green. Commit.

### Task 7: portal/ — Next.js 15

**Files:**
- Create: `portal/package.json` (next@15, react@19, iron-session, bcryptjs, pg), `portal/middleware.ts`, `portal/lib/{db,session,auth,memo,format}.ts`, `portal/app/login/page.tsx`, `portal/app/api/auth/login/route.ts`, `portal/app/api/auth/logout/route.ts`, `portal/app/(protected)/layout.tsx`, `dashboard/page.tsx`, `jobs/page.tsx`, `jobs/[id]/page.tsx`, `coins/page.tsx`, `portal/Dockerfile`, memo decode unit test (same golden vectors as Go, vitest).

**Steps:**
- [ ] Auth: POST login → bcrypt compare vs `ADMIN_PASSWORD_HASH`, iron-session cookie (httpOnly, sameSite=lax, secure in prod); in-memory rate limit 5 fails/IP → 15 min lock; middleware protects `(protected)`; logout destroys session.
- [ ] lib/db: `pg` Pool as `portal_ro`; server components only, `export const revalidate = 0` + refresh button.
- [ ] Pages per spec §9.2; status badges (DONE green, FAILED amber, in-progress gray, Abelian purple #6250E0); IBM Plex fonts; jobs filter/search/pagination 50/page; job detail timeline from `job_events`, memo decoded, explorer links from env.
- [ ] `npm run build` green + vitest memo tests pass. Commit.

### Task 8: README + integration

- [ ] README: architecture, quickstart (`cp .env.example .env` → fill → `docker compose up`), bcrypt hash generation command, `make` targets, CGO decision record, 待確認清單 C1–C5 table (from spec §13), verification path walkthrough (§1).
- [ ] `docker compose build` succeeds locally. Commit.

### Task 9: Subagent verification (user requirement)

- [ ] Dispatch test-runner subagent: `make test` (Go unit), `forge test`, portal build + vitest — report exact output.
- [ ] Dispatch code-review subagent(s): review full diff vs spec — correctness, idempotency rules 6.1-1..5, SDK API fidelity vs `docs/sdk-verified-api.md`, security (session, SQL params, key handling).
- [ ] Apply fixes; re-run tests; final commit.

## Self-Review Notes

- Spec coverage: §3 scaffold ✅ T1; §4 env ✅ T1; §5 memo ✅ T3+T7 (golden shared); §6 schema/state ✅ T1+T3; §7.1 ✅ T4; §7.2/7.3 ✅ T5; §7.4 ✅ T4; §7.5 ✅ T6; §8 ✅ T2; §9 ✅ T7; §10 ✅ T1+T6+T7; §11 tests ✅ T2–T5, integration optional (not Sprint-5-required, skipped); §12 order followed; §13 ✅ T8.
- Known divergence from spec (documented in README): `portal_ro` role created by initdb script (password can't live in migration SQL); migration 0002 still applies grants. Confirmer uses block-derived confirmations due to SDK `Tx.Confirmations` bson-tag bug.
