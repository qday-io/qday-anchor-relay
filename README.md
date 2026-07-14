# qday-anchor-relay

QDAY2 ↔ Abelian commit anchoring relay (Sprint 5 · AbelianPlusNode).

Anchors QDAY DA-chain batch commits — one **commit = one zkRollup state root** — onto the
Abelian chain (as a transaction memo), then, once confirmed, records the result back into the
`QDayAnchorRegistry` contract on the DA side, completing a two-way anchor. Full specification:
`doc/qday-anchor-relay-spec.md`.

```
┌─────────────┐   eth_getLogs    ┌─────────────────────────── relay ───────────────────────────┐
│ QDAY DA Node │ ───────────────▶ │ Poller ─▶ anchor_jobs(PENDING)                               │
│  (EVM)       │                  │             │ Submitter: memo tx (QDAY2ANCHOR|v1|batch|root) │
└─────▲───────┘                  │             ▼                                                │
      │ recordAnchor             │        ABEL_SUBMITTED ─▶ Confirmer (N confirmations)         │
      │                          │             ▼                                                │
┌─────┴──────────────┐           │        ABEL_CONFIRMED ─▶ Recorder ─▶ CONTRACT_RECORDED ─▶ DONE│
│ QDayAnchorRegistry │ ◀──────── │                                                              │
└────────────────────┘           └───────────────┬──────────────────────────────────────────────┘
                                                 │ SendRawTx(memo)        ┌──────────────┐
                                                 └───────────────────────▶│ Abelian node │
                                  Postgres ◀─── portal (read-only, :3000) └──────────────┘
```

Third-party verification path: read the `Anchored` event from `QDayAnchorRegistry` → take
`abelTxHash` → fetch the transaction via Abelian RPC `getrawtransaction` and read its `memo`
field (hex) → decode and compare the state root.

## Quick start

```bash
cp .env.example .env
# Fill in the required values (see the to-confirm list below). Generate the admin password hash:
node -e "console.log(require('bcryptjs').hashSync(process.argv[1], 12))" 'your-password'
# SESSION_SECRET: openssl rand -hex 32

docker compose up -d --build
open http://localhost:3000        # portal (username/password login)
```

Only the portal publishes a port; the relay and postgres stay on the internal compose network.
Migrations are embedded in the relay binary and applied automatically at startup (idempotent) —
no separate migrate service.

## Make targets

| target | purpose |
|---|---|
| `make up` / `make down` / `make logs` | start/stop compose, tail logs |
| `make migrate DATABASE_URL=…` | apply migrations to an arbitrary DB manually (the relay already self-migrates at startup) |
| `make test` | relay Go tests + contracts Foundry tests |
| `make test-relay` | requires the liboqs fork (see below); DB integration tests need `TEST_DATABASE_URL` |
| `make test-contracts` | `forge test` |
| `make abigen` | regenerate the Go binding for `QDayAnchorRegistry` |

## Local builds: the cryptosuite liboqs fork (important)

**`CGO_ENABLED=0` is not possible.** `abelian-sdk-go-v2` depends on `cryptosuite/liboqs-go`
(a cgo binding) which calls `OQS_KEM_keypair_with_recovery` — an API that exists **only in the
cryptosuite fork**; upstream liboqs (the brew package) does not have it. The Docker build
handles this automatically (`relay/Dockerfile` stage 1). For local development:

```bash
git clone --depth 1 https://github.com/cryptosuite/liboqs.git
# The fork predates modern compilers; -Werror trips on new warnings:
# comment out add_compile_options(-Werror) in .CMake/compiler_opts.cmake
cmake -S liboqs -B liboqs/build -DBUILD_SHARED_LIBS=ON -DOQS_BUILD_ONLY_LIB=ON \
      -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=$HOME/.local/liboqs
cmake --build liboqs/build -j8 && cmake --install liboqs/build
# Write a liboqs.pc into $HOME/.local/liboqs/lib/pkgconfig (see relay/Dockerfile stage 1)
make test-relay LIBOQS_PREFIX=$HOME/.local/liboqs
```

DB integration tests (optional):

```bash
docker run -d --rm --name qday-test-pg -e POSTGRES_DB=anchor -e POSTGRES_USER=anchor \
  -e POSTGRES_PASSWORD=pw -p 5434:5432 postgres:16-alpine
docker run --rm -v $PWD/relay/migrations:/migrations:ro migrate/migrate:v4.18.1 \
  -path=/migrations -database="postgres://anchor:pw@host.docker.internal:5434/anchor?sslmode=disable" up
TEST_DATABASE_URL="postgres://anchor:pw@localhost:5434/anchor?sslmode=disable" \
  make test-relay LIBOQS_PREFIX=...
```

## Contract deployment (contracts/)

```bash
cd contracts
RELAYER_ADDRESS=0x<relayer EOA> forge script script/Deploy.s.sol \
  --rpc-url $DA_RPC_URL --private-key $DEPLOYER_PRIVATE_KEY --broadcast
# Put the deployed address into ANCHOR_REGISTRY_ADDRESS in .env
```

Contracts are compiled with `evm_version = paris` (no PUSH0 — a conservative choice until the
DA chain's fork level is confirmed).

## Memo protocol (must not be changed casually)

```
QDAY2ANCHOR|v1|<batchNumber decimal>|0x<stateRoot 64 hex chars, lowercase>
```

The relay (Go, `relay/internal/anchor`) and the portal (TS, `portal/lib/memo.ts`) each carry an
implementation, sharing the same golden test vectors.

## Idempotency design (spec §6.1, point by point)

1. **Resume after restart**: worker ticks are driven purely by DB state, sweeping every job
   that is not `DONE`/`FAILED`.
2. **Never double-send**: before submitting, if the job already has an `abel_tx_hash`, check
   the chain via `getrawtransaction` first — if the tx exists, advance without resending.
3. **Check `anchorOf` before writing the contract**: if already recorded, advance; after a
   revert the contract is checked again (distinguishing "someone recorded it" from a real failure).
4. **Two-layer duplicate protection**: DB UNIQUE constraints on `batch_number` / `state_root`
   plus contract-level reverts.
5. **Resends carry the identical payload**: the memo is a pure function of `(batch, root)`,
   so a resend is byte-identical by construction.

## Implementation notes (deviations from the spec / verified SDK facts)

Source of truth for all SDK facts: `docs/sdk-verified-api.md` (extracted from SDK source,
with file/line references).

- **Aconcagua (TxVersion 3)**: testnet2 runs the abec Aconcagua fork; the official SDK pins an
  older abec that only knows versions 1–2 (`ReceiveCoin` fails with `Unsupported TxVersion`).
  This repo upgrades to `abec@main` and vendors the SDK at `relay/third_party/abelian-sdk-go-v2`
  (go.mod replace). The patch is two call sites: `CreateTransferTxByRootSeed/ByCryptoKeys`
  gained a leading `txVersion` parameter (we pass `wire.TxVersion`). The vendored copy can be
  dropped once upstream catches up.
- **Fixed funding address**: mnemonic wallets derive a deterministic address from AIP-11
  sequence-0 public randomness (stable across restarts); key-file wallets have no such path and
  fall back to per-request rotation.
- **Short addresses + ANS**: wallets transfer to the short form (`abe0…`, 68 bytes) but resolve
  short → full via the Abelian Name Service — **an unregistered short address cannot receive
  wallet transfers**. The relay auto-registers its funding address at startup (`ABEL_ANS_URL`);
  test accounts generated in the portal are auto-registered too.
- **Internal admin API** (`RELAY_ADMIN_ADDR`, default :8081): compose-internal only, never
  published; the portal proxies to it behind session auth. Provides wallet-address (including
  the short form) and generate-account.
- **Real commit event**: `VerifyBatchesTrustedAggregator` on the `PolygonRollupManager`
  (topic0 computed from the ABI and pinned by a unit test). `DA_ROLLUP_ID` filters the indexed
  rollupID — the DB's `batch_number` UNIQUE constraint assumes a single rollup, so the filter
  is mandatory when multiple rollups share one manager.
- **`Tx.Confirmations` is unusable**: the SDK struct tag says `bson` instead of `json`, so the
  field never unmarshals. The confirmer uses `Block.Confirmations` from `getblock` (correct tag)
  or falls back to `tip - height + 1`.
- **AIP-11 parameter order**: `MnemonicsToCryptoSeed` returns `(spend, sn, detector, view)` but
  `NewRootSeedAccount` takes `(spend, sn, value=view, detector)` — the wallet loader remaps
  deliberately; do not "fix" it.
- **Memo assignment**: the `NewTxDescWithRing` constructor has no memo parameter; the
  `TxDesc.TxMemo` field is assigned after construction (spec F1).
- **`portal_ro` role**: the password cannot live in migration SQL, so the role is created by
  `deploy/initdb/01-portal-ro.sh` (reads env vars); migration 0002 only handles GRANTs (the
  fallback role on pre-existing clusters is NOLOGIN — ops must set a password manually).
- **`abel_cursor` (migration 0003) / `serial_number` (migration 0004)**: two extensions beyond
  the spec schema — a crash-safe Abelian scan cursor, and the coin serial-number column used
  for on-chain spend detection (mirroring the SDK `examples/coin` TrackCoins flow).
- **Coinbase outputs are not scanned**: the relay wallet is expected to be funded by transfer;
  skipping coinbase also sidesteps the 200-block maturity rule (F11). Mining-funded wallets
  would need explicit maturity handling.
- **Rings rebuilt at spend time, serials persisted**: the ring needed to spend a coin is rebuilt
  from chain data (`GetRingBlockGroupByHeight` → `BuildCoinRings`); serial numbers are derived
  by the scanner once a coin's ring group completes and stored in the DB for Vin-serial spend
  detection. An incomplete ring group reports "wait" (does not consume the retry budget).
- **Lost-transaction recovery**: the confirmer distinguishes "node says no such transaction"
  from transport errors (the latter only retries, never resubmits); once a tx is declared lost,
  the job rolls back to PENDING and its coins are resurrected to UNSPENT — except coins whose
  spend was already observed on chain.
- **Txid persisted before broadcast**: the locally computed txid is written to the job after
  signing but before sending, so a crash inside the broadcast window cannot cause a
  double-send; stale LOCKED coins are reconciled by the submitter on restart (on chain → SPENT,
  not on chain → released).
- **Single input, single change output**: the submitter always builds 1-in/1-out (change back
  to itself); the SDK's Sort* helpers are only invoked with multiple descriptors.
- **Change address differs every time**: `RootSeedAccount.GenerateAbelAddress()` produces a
  different address per call (SDK design, fresh public randomness); ownership detection is via
  `ReceiveCoin` (detector key), not address comparison, so this is harmless.

## To-confirm list (spec §13)

| # | Item | Owner | Scope | Current default |
|---|------|-------|-------|-----------------|
| C1 | ~~DA commit contract address + event ABI~~ ✅ confirmed | Vike | `PolygonRollupManager` / `VerifyBatchesTrustedAggregator` | Implemented and verified live (batch 221 ran to DONE) |
| C2 | DA RPC endpoint, chain ID, relayer gas funding | Team | `.env` | empty (validated as required at startup) |
| C3 | Abelian endpoint (testnet/mainnet) and RPC credentials | Morris | `.env` | empty |
| C4 | Wallet form: mnemonic or key material; privacy level | Morris | `.env` (both loaders implemented) | `pseudonymous` |
| C5 | Testnet explorer URL | Morris | portal external links | mainnet explorer |

Also to confirm: the DA chain's EVM fork level (contracts currently compiled for paris), and
`ABEL_TX_FEE_NEUTRINO` (currently aligned with the official 0.1 ABEL constant; the SDK's
`EstimateTxFee` is hardcoded to 1_000_000 — see F8).

## Repository layout

```
relay/       Go 1.23 relay (cmd/relay + internal/{config,da,abel,registry,store,worker,anchor})
contracts/   Foundry: QDayAnchorRegistry + tests + deploy script
portal/      Next.js 15 admin portal (read-only, iron-session login)
deploy/      postgres initdb script (portal_ro role) + server deploy kit (GHCR compose + updater)
docs/        sdk-verified-api.md (SDK source of truth), implementation plan, deployment records
doc/         original specification
```
