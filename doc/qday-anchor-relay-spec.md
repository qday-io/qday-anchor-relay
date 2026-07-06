# QDAY2 ↔ Abelian Commit Anchoring Relay — Claude Code 實作規格 v1.0

> Sprint 5 · AbelianPlusNode · Target: 10 Jul 2026 · Owner: Morris / Vike
> 本文件為交付給 Claude Code 的完整實作規格。所有 SDK API 均經原始碼驗證，來源標註於 §2。

---

## 0. 給 Claude Code 的最高指導原則

1. **禁止發明 API。** Abelian SDK 的所有用法必須以 §2 列出的 repo 原始碼為準。實作前先 clone 並閱讀 `abelian-sdk-go-v2` 的 `examples/` 目錄（特別是 `examples/transaction/main.go`、`examples/transactionTracker/main.go`、`examples/database/`、`examples/coin/main.go`）。
2. 遇到本文件標註 `【TO-CONFIRM】` 的項目：以文件指定的預設值 + config 實作，並在 README 的「待確認清單」中列出，**不要自行猜測後寫死**。
3. 所有元件都必須可以在 `docker compose up` 一鍵啟動。
4. 冪等性是本系統的核心正確性要求（見 §6），任何寫路徑都要先想「crash 在這一行之後重啟會怎樣」。

---

## 1. 系統目標

以 **commit（zkRollup state root）為單位**：

1. **Poller**：從 QDAY DA Node（EVM 相容）上的 commit 合約讀取新的 batch commit（state root + batch number）。
2. **Submitter**：透過 Abelian Go SDK v2 建立帶 memo 的 Abelian 交易，memo 內容為 anchor payload，送上 Abelian 鏈。
3. **Confirmer**：等待 N 個 Abelian 區塊確認（N 可設定）。
4. **Recorder**：呼叫 DA 側 EVM 上我們部署的 `QDayAnchorRegistry` 合約，記錄 `(batchNumber, stateRoot, abelTxHash, abelHeight)`，完成雙向錨定。
5. **Portal**：帳密登入的後台，可查看所有 anchor job 的完整內容與狀態。

驗證路徑（第三方可獨立驗證）：讀 `QDayAnchorRegistry` 的 `Anchored` event → 取得 `abelTxHash` → 到 Abelian 鏈上查該交易（RPC `Tx.Memo` 欄位，hex 編碼）→ 解碼 memo 比對 state root。

---

## 2. 已驗證的技術事實（實作依據，附來源）

| # | 事實 | 來源（原始碼位置） |
|---|------|-------------------|
| F1 | Abelian 交易有 memo：建交易時 `TxDesc.TxMemo []byte` 可放任意 bytes | `pqabelian/abelian-sdk-go-v2` → `abelian/transaction.go` L65、L176 |
| F2 | RPC 查交易回傳 `Tx.Memo string`（hex 編碼） | 同上 → `abelian/rpc.go` L92；README §2 有真實主網交易含 memo 的範例 |
| F3 | Memo 大小上限：舊參數 1024 bytes、新參數（abecryptox）128KB | `pqabelian/abec` → `abecrypto/abecryptoparam/abecryptoparam.go` L62、`abecryptox/abecryptoxparam/abecryptoparamconsts.go` L17 |
| F4 | Memo 前綴協議是官方慣例：abec 的 AUT 協議即檢查 `TxMemo[:prefixLen]` | `pqabelian/abec` → `ctaut/api/workflow.go` L212-215 |
| F5 | SDK v2 支援 mnemonic（AIP-11）與 key material 兩種帳戶 | `abelian-sdk-go-v2` → `abelian/aip11.go`（`MnemonicsToCryptoSeed` 等）、`abelian/account.go`（`RootSeedAccount` / `CryptoKeysAccount`） |
| F6 | 建/簽/送交易流程：`NewTxDescWithRing` → `GenerateUnsignedRawTxWithRing` → `GenerateSignedRawTx(unsignedRawTx, signerAccounts)` → `client.SendRawTx(hex)` | `abelian-sdk-go-v2` → `examples/transaction/main.go` L24、L197-210 |
| F7 | RPC client 方法：`GetChainInfo`、`GetBlockHash`、`GetBlock(ByHeight)`、`GetRawTx`、`SendRawTx`、`GetRawMempool`，另有泛用 `Client.Do(method, params, result)` | `abelian-sdk-go-v2` → `abelian/methods.go`、`abelian/client.go` L108 |
| F8 | v1 SDK 的 `GetEstimatedTxFee()` 是**寫死的 0.1 ABEL**，不是真估算 | `pqabelian/abelian-sdk-go` → `abec-rpc.go` L257：`return AbelToNeutrino(0.1)` |
| F9 | v2 附官方範例：`examples/{account,address,coin,rpcclient,transaction,transactionTracker,database}`，database 範例示範 coin/ring/tx 的持久化 | `abelian-sdk-go-v2/examples/` |
| F10 | 網路：SDK 支援 MainNet / TestNet / RegressionNet / SimNet；abec RPC 預設需 rpcuser/rpcpass（+TLS） | `abelian-sdk-go-v2` → `abelian/chain.go`、`abelian/client.go`（`WithAuth`/`WithTLS`）|
| F11 | Coinbase maturity = 200 blocks（relay 錢包資金若來自挖礦需注意） | `abelian-sdk-go-v2` → `abelian/chain.go` `GetCoinbaseMaturity()` |

**SDK 版本決策：採用 v2（`github.com/pqabelian/abelian-sdk-go-v2`，Go 1.23.2）。**
理由：v2 功能完整（F5、F6、F7、F9 全部成立），v1 唯一的「優勢」費用估算實為寫死常數（F8）。v1 的 demo repo（`abelian-sdk-go-demo`）僅作流程概念參考，**不要**混用 v1 API。

---

## 3. Monorepo 結構

Repo 名稱：`qday-anchor-relay`（新 repo）。命名原則：這是 QDay team 的服務，repo 與合約皆以 `qday` / `QDay` 開頭；Abelian 在本系統中的角色是資料儲存與最終驗證層，不進入命名。

```
qday-anchor-relay/
├── docker-compose.yml
├── .env.example
├── README.md                  # 含架構圖、啟動方式、待確認清單
├── Makefile                   # make up / down / migrate / test / abigen
├── relay/                     # Go 1.23 anchoring relay
│   ├── cmd/relay/main.go
│   ├── internal/
│   │   ├── config/            # env 解析與驗證
│   │   ├── da/                # Poller：DA EVM 合約監聽（CommitSource interface）
│   │   ├── abel/              # Submitter + Confirmer：SDK v2 封裝、coin 管理
│   │   ├── registry/          # Recorder：QDayAnchorRegistry 寫入（go-ethereum）
│   │   ├── store/             # Postgres repository + 狀態機轉移
│   │   └── worker/            # 主迴圈：掃描 job、依狀態分派
│   ├── migrations/            # golang-migrate SQL 檔
│   ├── Dockerfile
│   └── go.mod
├── contracts/                 # Foundry 專案
│   ├── src/QDayAnchorRegistry.sol
│   ├── script/Deploy.s.sol
│   ├── test/QDayAnchorRegistry.t.sol
│   └── foundry.toml
├── portal/                    # Next.js 15 (App Router) 後台
│   ├── app/
│   │   ├── login/page.tsx
│   │   ├── (protected)/
│   │   │   ├── dashboard/page.tsx
│   │   │   ├── jobs/page.tsx
│   │   │   ├── jobs/[id]/page.tsx
│   │   │   └── coins/page.tsx
│   │   └── api/auth/...
│   ├── middleware.ts          # session 保護所有 (protected) 路由
│   ├── lib/db.ts              # Postgres 唯讀連線
│   ├── Dockerfile
│   └── package.json
└── docs/
    └── solution-design.html   # 既有的一頁式 solution design
```

---

## 4. 設定（.env.example 全表）

```bash
# ── Postgres ──────────────────────────────
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_DB=anchor
POSTGRES_USER=anchor
POSTGRES_PASSWORD=change-me
PORTAL_DB_USER=portal_ro            # portal 專用唯讀帳號（migration 建立）
PORTAL_DB_PASSWORD=change-me

# ── QDAY DA Node（EVM 側）──────────────────
DA_RPC_URL=                          # 【TO-CONFIRM】DA node 的 JSON-RPC endpoint
DA_CHAIN_ID=                         # 【TO-CONFIRM】由團隊夥伴提供
DA_COMMIT_CONTRACT=                  # 【TO-CONFIRM】commit 合約地址（Vike）
DA_CONFIRM_DEPTH=12                  # DA 側 reorg 安全深度
DA_POLL_INTERVAL_SECONDS=10
DA_START_BLOCK=0                     # 首次啟動的掃描起點

# ── QDayAnchorRegistry（DA 側 EVM）─────────────
ANCHOR_REGISTRY_ADDRESS=             # 部署後填入
RELAYER_PRIVATE_KEY=                 # DA 側 EOA（gas 注資由團隊夥伴負責）

# ── Abelian ───────────────────────────────
ABEL_NETWORK=testnet                 # mainnet | testnet | regressionnet | simnet
ABEL_RPC_ENDPOINT=                   # 【TO-CONFIRM】團隊已有節點，對接 endpoint
ABEL_RPC_USER=
ABEL_RPC_PASS=
ABEL_RPC_TLS_CERT_PATH=              # 選填；abec 預設 TLS
ABEL_CONFIRM_DEPTH=6                 # Abelian 確認深度
# 錢包：兩種擇一（都要實作支援，見 §7.2）
ABEL_WALLET_MNEMONIC=                # 【TO-CONFIRM】AIP-11 mnemonic 字串（空格分隔）
ABEL_WALLET_KEY_FILE=                # 【TO-CONFIRM】或 key material JSON 檔路徑
ABEL_ACCOUNT_PRIVACY=pseudonymous    # pseudonymous | full-privacy【TO-CONFIRM】
ABEL_TX_FEE_NEUTRINO=1000000         # 手續費（Neutrino），預設對齊官方 0.1 ABEL 常數，可調

# ── Relay 行為 ─────────────────────────────
RETRY_MAX=5
RETRY_BACKOFF_BASE_SECONDS=30        # exponential backoff 基數
MEMO_PREFIX=QDAY2ANCHOR              # 見 §5 memo 格式

# ── Portal ────────────────────────────────
PORTAL_PORT=3000
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=                 # bcrypt hash，README 說明產生方式
SESSION_SECRET=                      # ≥32 bytes random
ABEL_EXPLORER_BASE=https://explorer.pqabelian.io   # mainnet；testnet explorer【TO-CONFIRM】
DA_EXPLORER_BASE=                    # 【TO-CONFIRM】DA 側如有 explorer
```

---

## 5. Memo 格式（協議層規格，不可隨意變更）

採 ASCII 可讀格式（在 explorer 上 hex→string 直接可讀，參考 F2 的主網範例；官方 AUT 協議同樣用前綴模式，F4）：

```
QDAY2ANCHOR|v1|<batchNumber十進位>|0x<stateRoot 64位hex小寫>
```

範例：`QDAY2ANCHOR|v1|1024|0x3fa8...c21e`（約 90 bytes，遠低於 1KB 上限，F3）。

解析規則：以 `|` 分割固定 4 段；`v1` 為格式版本號，未來擴充時遞增。Relay 端與 Portal 端各實作一份 encode/decode + 單元測試（golden test 共用測試向量）。

---

## 6. 狀態機與資料庫（Postgres）

### 6.1 狀態機

```
PENDING → ABEL_SUBMITTED → ABEL_CONFIRMED → CONTRACT_RECORDED → DONE
                └──────────── 任一步失敗 → 原狀態 + retry_count++（≤ RETRY_MAX）→ FAILED
```

冪等規則（必須逐條實作）：
1. 重啟時掃描所有非 `DONE`/`FAILED` 的 job 續跑。
2. 送 Abelian 前：若 DB 已有 `abel_tx_hash`，先 `GetRawTx` 查鏈確認該交易是否存在／已確認，存在則直接推進狀態，**不重送**。
3. 寫合約前：先呼叫 `QDayAnchorRegistry.anchorOf(stateRoot)` 查是否已記錄，已記錄則直接推進狀態。
4. DB 層 `batch_number`、`state_root` 皆 UNIQUE；合約層重複 `recordAnchor` 直接 revert（雙層防重）。
5. Abelian 交易失敗重送時 memo 內容不變（同一 payload 即使意外上鏈兩次，語意上也無害，但依規則 2 應避免）。

### 6.2 Schema（migrations/0001_init.up.sql）

```sql
CREATE TABLE anchor_jobs (
  id              BIGSERIAL PRIMARY KEY,
  batch_number    BIGINT      NOT NULL UNIQUE,
  state_root      BYTEA       NOT NULL UNIQUE,       -- 32 bytes
  da_block_number BIGINT      NOT NULL,
  da_tx_hash      TEXT,                              -- commit 事件所在交易
  abel_tx_hash    TEXT,
  abel_height     BIGINT,
  record_tx_hash  TEXT,                              -- QDayAnchorRegistry 寫入交易
  status          TEXT        NOT NULL DEFAULT 'PENDING',
  retry_count     INT         NOT NULL DEFAULT 0,
  last_error      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_status ON anchor_jobs(status);

-- Poller 游標：記錄 DA 側掃描進度（crash-safe）
CREATE TABLE poller_cursor (
  id              INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  last_scanned_block BIGINT NOT NULL
);

-- Relay 錢包的 coin 追蹤（Abelian 為 UTXO+ring 模型；schema 對齊
-- abelian-sdk-go-v2/examples/database/ 的欄位設計，改用 Postgres）
CREATE TABLE relay_coins (
  coin_id         TEXT PRIMARY KEY,                  -- txHash:index
  tx_hash         TEXT   NOT NULL,
  out_index       SMALLINT NOT NULL,
  value_neutrino  BIGINT NOT NULL,
  block_height    BIGINT NOT NULL,
  ring_id         TEXT,
  ring_index      SMALLINT,
  status          TEXT   NOT NULL DEFAULT 'UNSPENT', -- UNSPENT | LOCKED | SPENT
  spent_by_job    BIGINT REFERENCES anchor_jobs(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_coins_status ON relay_coins(status);

-- 狀態轉移稽核（portal 的 timeline 資料來源）
CREATE TABLE job_events (
  id          BIGSERIAL PRIMARY KEY,
  job_id      BIGINT NOT NULL REFERENCES anchor_jobs(id),
  from_status TEXT,
  to_status   TEXT NOT NULL,
  detail      JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_events_job ON job_events(job_id);
```

Migration 同時建立 `portal_ro` 唯讀角色（僅 SELECT 權限）。

---

## 7. Relay 實作細節（Go）

### 7.1 Poller（`internal/da`）

**已知：commit 從 DA Node 上的合約拿（EVM）。未知：合約 ABI 與事件簽名（Vike 提供）。**

實作方式——定義 adapter interface，讓 ABI 確認後只改一個檔案：

```go
type Commit struct {
    BatchNumber uint64
    StateRoot   [32]byte
    DABlock     uint64
    DATxHash    string
}

type CommitSource interface {
    // 掃描 [from, to] 區塊範圍內的 commit 事件
    FetchCommits(ctx context.Context, from, to uint64) ([]Commit, error)
    LatestBlock(ctx context.Context) (uint64, error)
}
```

預設實作 `EventLogCommitSource`：用 `go-ethereum` 的 `ethclient` + `eth_getLogs` 監聽 `DA_COMMIT_CONTRACT`，事件簽名暫定：

```solidity
event BatchCommitted(uint64 indexed batchNumber, bytes32 stateRoot);
```

`【TO-CONFIRM】`此事件簽名為**假設**，等 Vike 提供實際 ABI 後調整 `EventLogCommitSource` 一個檔案即可。topic hash 由 ABI 計算，不要手寫。

Poller 迴圈：`safeHead = LatestBlock() - DA_CONFIRM_DEPTH`；掃描 `(cursor, safeHead]`，每批寫入 `anchor_jobs`（`ON CONFLICT DO NOTHING`，冪等）+ 更新 `poller_cursor`，**同一個 DB transaction**。

### 7.2 Abelian 錢包與 Submitter（`internal/abel`）

錢包載入（兩種都支援，config 擇一）：
- **Mnemonic**：`aip11.MnemonicsToCryptoSeed` → `NewRootSeedAccount`（參考 `examples/account/main.go` 的實際建構方式）。
- **Key material**：從 JSON 檔載入 → `NewCryptoKeysAccount`。JSON 欄位以 SDK `examples/account` 實際輸出為準。

Coin 掃描：啟動時與定期（每個 Abelian 新塊）掃描區塊中屬於本帳戶的 output——用 `ViewAccount.ReceiveCoin(txVersion, txOutData)` 判定歸屬（見 `abelian/account.go` L207/L356），寫入 `relay_coins`。實作邏輯完整參照 `examples/coin/main.go` 與 `examples/database/coin.go`，只是持久層換成 Postgres。

Submitter 流程（參照 `examples/transaction/main.go`，函式名皆已驗證 F6）：
1. 從 `relay_coins` 挑選 `UNSPENT` 且 value ≥ fee 的 coin，標記 `LOCKED`（DB transaction）。
2. 取得 ring 資料、組 `TxInDescWithRing` / `TxOutDesc`（找零回自己地址）。
3. `txDesc := abelian.NewTxDescWithRing(txInDescs, txOutDescs, feeNeutrino)`，**再設定 `txDesc.TxMemo = []byte(memoString)`**（`NewTxDescWithRing` 建構子不含 memo 參數，直接對欄位賦值——見 F1、`transaction.go` L176）。
4. `GenerateUnsignedRawTxWithRing` → `GenerateSignedRawTx(unsigned, []Account{acct})` → `client.SendRawTx(hex.EncodeToString(signed.Data))`。
5. 記錄 `abel_tx_hash`，狀態 → `ABEL_SUBMITTED`，coin → `SPENT`（找零 coin 待確認後由掃描器收回）。

### 7.3 Confirmer

`GetChainInfo()` 取當前高度；`GetRawTx(abelTxHash)` 取交易確認資訊（回傳結構含 `Confirmations` 欄位，見 v2 README 範例輸出）。`confirmations ≥ ABEL_CONFIRM_DEPTH` → 狀態 `ABEL_CONFIRMED`。交易在 mempool 消失且未上鏈超過 timeout → 回退重送（規則 6.1-2）。

### 7.4 Recorder（`internal/registry`）

用 `go-ethereum` 對 `QDayAnchorRegistry` 發交易。合約 binding 用 `abigen` 產生（Makefile target `make abigen`），不要手刻 ABI encoding。gas price 用 `SuggestGasPrice`，nonce 用 pending nonce。收到 receipt（status=1）→ `CONTRACT_RECORDED` → `DONE`；revert 則檢查是否為「已記錄」（先 `anchorOf` 查詢，規則 6.1-3）。

### 7.5 Worker 主迴圈

單一 process、goroutine per worker（poller / submitter / confirmer / recorder / coin-scanner），各自獨立 ticker + context 優雅關閉。結構化日誌用 `log/slog`（JSON handler）。任何 worker panic 要 recover + 記 log，不能拖垮整個 process。

---

## 8. QDayAnchorRegistry 合約（contracts/src/QDayAnchorRegistry.sol）

**此合約儲存的就是 relay「處理後的最終結果」**——只有當 pipeline 全部完成（commit 已取得 → 已錨定到 Abelian → 已達確認深度）之後，Recorder 才會把這筆完整的錨定紀錄寫入合約。每筆紀錄包含：`batchNumber`（QDAY2 batch 編號）、`stateRoot`（該 batch 的 zkRollup state root）、`abelTxHash`（承載此 state root 的 Abelian 交易 hash）、`abelHeight`（該交易所在的 Abelian 區塊高度）、`recordedAt`（上鏈時間戳）。中間狀態與失敗紀錄不上鏈，只留在 Postgres（`anchor_jobs` + `job_events`）供 portal 查閱——鏈上只存「已驗證完成」的結果。

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract QDayAnchorRegistry {
    struct Anchor {
        uint64  batchNumber;
        bytes32 abelTxHash;
        uint64  abelHeight;
        uint64  recordedAt;   // block.timestamp
    }

    address public owner;
    mapping(address => bool) public relayers;
    mapping(bytes32 => Anchor) private anchors;       // stateRoot => Anchor
    mapping(uint64  => bytes32) public rootOfBatch;   // batchNumber => stateRoot

    event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot,
                   bytes32 abelTxHash, uint64 abelHeight);
    event RelayerSet(address relayer, bool allowed);

    error NotOwner();
    error NotRelayer();
    error AlreadyAnchored(bytes32 stateRoot);
    error BatchAlreadyAnchored(uint64 batchNumber);

    modifier onlyOwner()   { if (msg.sender != owner) revert NotOwner(); _; }
    modifier onlyRelayer() { if (!relayers[msg.sender]) revert NotRelayer(); _; }

    constructor() { owner = msg.sender; }

    function setRelayer(address r, bool allowed) external onlyOwner {
        relayers[r] = allowed;
        emit RelayerSet(r, allowed);
    }

    function recordAnchor(uint64 batchNumber, bytes32 stateRoot,
                          bytes32 abelTxHash, uint64 abelHeight)
        external onlyRelayer
    {
        if (anchors[stateRoot].recordedAt != 0) revert AlreadyAnchored(stateRoot);
        if (rootOfBatch[batchNumber] != bytes32(0)) revert BatchAlreadyAnchored(batchNumber);
        anchors[stateRoot] = Anchor(batchNumber, abelTxHash, abelHeight, uint64(block.timestamp));
        rootOfBatch[batchNumber] = stateRoot;
        emit Anchored(batchNumber, stateRoot, abelTxHash, abelHeight);
    }

    function anchorOf(bytes32 stateRoot) external view
        returns (uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt)
    {
        Anchor memory a = anchors[stateRoot];
        return (a.batchNumber, a.abelTxHash, a.abelHeight, a.recordedAt);
    }

    function isAnchored(bytes32 stateRoot) external view returns (bool) {
        return anchors[stateRoot].recordedAt != 0;
    }
}
```

備註：`abelTxHash` 是 Abelian 交易 hash（32 bytes hex → bytes32）。Foundry 測試至少覆蓋：權限、重複 revert、event 內容、`anchorOf` 回讀。部署腳本 `Deploy.s.sol` 部署後自動 `setRelayer(relayerEOA, true)`。

---

## 9. Portal（Next.js 15，App Router）

### 9.1 認證（必要需求：帳密登入，非公開）

- 單一 admin 帳號：`ADMIN_USERNAME` + `ADMIN_PASSWORD_HASH`（bcrypt，cost ≥ 12）。README 提供產生指令：`node -e "console.log(require('bcryptjs').hashSync(process.argv[1], 12))" 'your-password'`。
- Session：`iron-session`（httpOnly、secure、sameSite=lax cookie），`SESSION_SECRET` 加密。
- `middleware.ts` 保護 `(protected)` 群組全部路由，未登入 302 → `/login`。
- 登入端點 rate limit：同 IP 5 次失敗鎖 15 分鐘（記憶體即可，單機部署）。
- 無註冊功能、無密碼重設（環境變數即真相）。
- DB 連線用 `portal_ro` 唯讀帳號——portal 對資料庫**零寫入權限**。

### 9.2 頁面

| 頁面 | 內容 |
|------|------|
| `/dashboard` | 各狀態 job 計數卡片、最近 10 筆 job、relay 健康摘要（poller cursor 落後量、UNSPENT coin 數與總額、FAILED 數） |
| `/jobs` | 全部 job 表格：batch#、state root（截斷+複製鈕）、狀態 badge、abel tx、record tx、更新時間。支援狀態 filter、batch# 搜尋、分頁（50/頁） |
| `/jobs/[id]` | 單一 job 完整資訊 + `job_events` timeline + memo 內容（decode 後顯示）+ explorer 外連（`ABEL_EXPLORER_BASE`、`DA_EXPLORER_BASE`） |
| `/coins` | relay_coins 表格：coin id、value（顯示 ABEL 與 Neutrino）、狀態、高度 |

狀態 badge 配色沿用 solution design：DONE 綠、FAILED 琥珀、進行中灰、Abelian 相關紫（`#6250E0`）。設計基調沿用 `docs/solution-design.html` 的 token（IBM Plex Sans/Mono、紙白底、ink 墨色）。

資料讀取用 Server Components 直接查 Postgres（`pg`），每頁 `revalidate = 0` + 手動重新整理鈕即可，不需要 websocket。

---

## 10. docker-compose.yml

服務：
1. `postgres:16-alpine`：volume 持久化、healthcheck `pg_isready`。
2. `migrate`：`migrate/migrate` 官方 image，one-shot 跑 `relay/migrations`，`depends_on postgres (healthy)`。
3. `relay`：multi-stage build（`golang:1.23` build → `debian:bookworm-slim` run；**先嘗試 `CGO_ENABLED=0`，若 SDK 依賴的 `cryptosuite/liboqs-go` 造成連結錯誤則改 CGO=1 並在 builder 安裝 build-essential，README 記錄結論**）。`depends_on migrate (completed)`。restart: unless-stopped。
4. `portal`：`node:22-alpine` multi-stage，`depends_on postgres (healthy)`，對外 `PORTAL_PORT`。

只有 portal 對外開 port；relay 與 postgres 走內部網路。

---

## 11. 測試策略

1. **單元測試（relay）**：狀態機轉移全矩陣、memo encode/decode golden tests、coin 選擇邏輯、retry/backoff。
2. **合約測試（Foundry）**：§8 列出的四項。
3. **整合測試（可選，時間允許）**：`CommitSource` 與 Abelian client 各做 mock 實作，跑完整 pipeline happy path + crash 恢復（kill 在每個狀態、重啟驗證續跑）。abec 有 SimNet/RegressionNet（F10）可作真鏈整合測試，但**不列入 Sprint 5 必要範圍**。
4. CI 不強制（新 repo，先 `make test` 可跑即可）。

---

## 12. 建置順序（給 Claude Code 的執行計畫)

1. Monorepo 骨架 + docker-compose + migrations + `.env.example`。
2. `contracts/`：QDayAnchorRegistry + 測試 + 部署腳本（獨立可交付，最快能給團隊部署）。
3. `relay/`：config → store/狀態機（含單元測試）→ registry（abigen）→ da poller（interface + 假設事件實作）→ abel（錢包載入 → coin 掃描 → submitter → confirmer）。**abel 是最重的部分，實作前必讀 SDK examples。**
4. `portal/`：auth → jobs 列表 → job 詳情 → dashboard → coins。
5. README：架構圖（引用 docs/solution-design.html）、快速啟動、bcrypt 產生指令、**待確認清單**（§13）。

## 13. 待確認清單（實作以 config + 預設值處理，勿寫死猜測）

| # | 項目 | 誰確認 | 影響範圍 |
|---|------|--------|----------|
| C1 | DA commit 合約地址 + 事件 ABI | Vike | `internal/da` 一個檔案 |
| C2 | DA RPC endpoint、chain ID、relayer gas 注資 | 團隊夥伴 | `.env` |
| C3 | Abelian endpoint（testnet/mainnet）與 RPC 憑證 | Morris | `.env` |
| C4 | 錢包形式：mnemonic 或 key material；privacy level | Morris | `.env`（兩種 loader 都已實作） |
| C5 | Testnet explorer URL | Morris | portal 外連 |
