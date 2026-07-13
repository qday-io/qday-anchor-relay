# qday-anchor-relay

QDAY2 ↔ Abelian commit anchoring relay（Sprint 5 · AbelianPlusNode）。

以 **commit（zkRollup state root）為單位**，把 QDAY DA 鏈上的 batch commit 錨定到 Abelian 鏈（交易 memo），
確認後回寫 DA 側 `QDayAnchorRegistry` 合約，完成雙向錨定。規格見 `doc/qday-anchor-relay-spec.md`。

```
┌─────────────┐   eth_getLogs    ┌─────────────────────────── relay ───────────────────────────┐
│ QDAY DA Node │ ───────────────▶ │ Poller ─▶ anchor_jobs(PENDING)                               │
│  (EVM)       │                  │             │ Submitter：memo tx（QDAY2ANCHOR|v1|batch|root） │
└─────▲───────┘                  │             ▼                                                │
      │ recordAnchor             │        ABEL_SUBMITTED ─▶ Confirmer（N confirmations）         │
      │                          │             ▼                                                │
┌─────┴──────────────┐           │        ABEL_CONFIRMED ─▶ Recorder ─▶ CONTRACT_RECORDED ─▶ DONE│
│ QDayAnchorRegistry │ ◀──────── │                                                              │
└────────────────────┘           └───────────────┬──────────────────────────────────────────────┘
                                                 │ SendRawTx(memo)        ┌──────────────┐
                                                 └───────────────────────▶│ Abelian node │
                                  Postgres ◀─── portal（唯讀後台, :3000）  └──────────────┘
```

第三方驗證路徑：讀 `QDayAnchorRegistry` 的 `Anchored` event → 取 `abelTxHash` → Abelian RPC
`getrawtransaction` 的 `memo` 欄位（hex）→ 解碼比對 state root。

## 快速啟動

```bash
cp .env.example .env
# 填入必要值（見下方待確認清單）；產生 admin 密碼 hash：
node -e "console.log(require('bcryptjs').hashSync(process.argv[1], 12))" 'your-password'
# SESSION_SECRET：openssl rand -hex 32

docker compose up -d --build
open http://localhost:3000        # portal（帳密登入）
```

只有 portal 對外開 port；relay 與 postgres 走 compose 內部網路。migration 由
`migrate/migrate` one-shot container 於 postgres healthy 後自動執行。

## Make targets

| target | 作用 |
|---|---|
| `make up` / `make down` / `make logs` | docker compose 起停/看 log |
| `make migrate` | 手動重跑 migrations |
| `make test` | relay Go 測試 + contracts Foundry 測試 |
| `make test-relay` | 需要 liboqs fork（見下）；DB 整合測試需 `TEST_DATABASE_URL` |
| `make test-contracts` | `forge test` |
| `make abigen` | 重新產生 `QDayAnchorRegistry` 的 Go binding |

## 本機建置：cryptosuite liboqs fork（重要）

**`CGO_ENABLED=0` 不可行。** `abelian-sdk-go-v2` 依賴 `cryptosuite/liboqs-go`（cgo binding），
且其呼叫 `OQS_KEM_keypair_with_recovery` —— 這是 **cryptosuite fork 專屬 API**，
上游 liboqs（brew 版）沒有。Docker build 已內建處理（`relay/Dockerfile` stage 1）。本機開發：

```bash
git clone --depth 1 https://github.com/cryptosuite/liboqs.git
# fork 較舊，新版 clang 會踩 -Werror：註解掉 .CMake/compiler_opts.cmake 的 add_compile_options(-Werror)
cmake -S liboqs -B liboqs/build -DBUILD_SHARED_LIBS=ON -DOQS_BUILD_ONLY_LIB=ON \
      -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=$HOME/.local/liboqs
cmake --build liboqs/build -j8 && cmake --install liboqs/build
# 寫一個 liboqs.pc 到 $HOME/.local/liboqs/lib/pkgconfig（參考 relay/Dockerfile stage 1）
make test-relay LIBOQS_PREFIX=$HOME/.local/liboqs
```

DB 整合測試（可選）：

```bash
docker run -d --rm --name qday-test-pg -e POSTGRES_DB=anchor -e POSTGRES_USER=anchor \
  -e POSTGRES_PASSWORD=pw -p 5434:5432 postgres:16-alpine
docker run --rm -v $PWD/relay/migrations:/migrations:ro migrate/migrate:v4.18.1 \
  -path=/migrations -database="postgres://anchor:pw@host.docker.internal:5434/anchor?sslmode=disable" up
TEST_DATABASE_URL="postgres://anchor:pw@localhost:5434/anchor?sslmode=disable" \
  make test-relay LIBOQS_PREFIX=...
```

## 合約部署（contracts/）

```bash
cd contracts
RELAYER_ADDRESS=0x<relayer EOA> forge script script/Deploy.s.sol \
  --rpc-url $DA_RPC_URL --private-key $DEPLOYER_PRIVATE_KEY --broadcast
# 部署後把地址填入 .env 的 ANCHOR_REGISTRY_ADDRESS
```

合約以 `evm_version = paris` 編譯（無 PUSH0——DA 鏈 fork 等級未確認前的保守選擇）。

## Memo 協議（不可任意變更）

```
QDAY2ANCHOR|v1|<batchNumber 十進位>|0x<stateRoot 64 位小寫 hex>
```

Relay（Go `relay/internal/anchor`）與 Portal（TS `portal/lib/memo.ts`）各一份實作，
共用 golden test 向量。

## 冪等性設計（spec §6.1，逐條對應）

1. **重啟續跑**：worker tick 以 DB 狀態為準，掃描所有非 `DONE`/`FAILED` job。
2. **不重送**：submit 前若 job 已有 `abel_tx_hash`，先 `getrawtransaction` 查鏈，存在即直接推進。
3. **寫合約前查 `anchorOf`**：已記錄即推進；revert 後亦會再查一次（區分「已被記錄」與真失敗）。
4. **雙層防重**：DB `batch_number` / `state_root` UNIQUE + 合約 revert。
5. **重送同 payload**：memo 由 `(batch, root)` 決定，重送內容必然相同。

## 實作備註（與 spec 的差異／已驗證的 SDK 事實）

全部 SDK 事實來源：`docs/sdk-verified-api.md`（自 SDK 原始碼萃取，含檔案行號）。

- **Aconcagua（TxVersion 3）**：testnet2 已跑 abec Aconcagua 分叉，官方 SDK pin 的舊 abec
  只認 version 1–2（`ReceiveCoin` 會炸 `Unsupported TxVersion`）。本 repo 升級
  `abec@main` 並 vendor SDK 於 `relay/third_party/abelian-sdk-go-v2`（go.mod replace），
  補丁僅兩處：`CreateTransferTxByRootSeed/ByCryptoKeys` 多了開頭 `txVersion` 參數
  （傳 `wire.TxVersion`）。上游 SDK 更新後可移除 vendor。
- **固定注資地址**：mnemonic 錢包用 AIP-11 序號 0 的 public randomness 衍生確定性地址
  （重啟不變）；key-file 錢包無此路徑，退回每次輪換。
- **短地址 + ANS**：錢包轉帳用短地址（`abe0…`，68 bytes），但錢包靠 ANS 解析短→完整地址，
  **未註冊的短地址收不到轉帳**。relay 啟動時自動註冊注資地址（`ABEL_ANS_URL`）；
  portal 產生的測試帳號也自動註冊。
- **內部 admin API**（`RELAY_ADMIN_ADDR`，預設 :8081）：僅 compose 內網，不對外開 port；
  portal 過 session 驗證後代理。提供 wallet-address（含短地址）與 generate-account。
- **真 commit 事件**：`PolygonRollupManager` 的 `VerifyBatchesTrustedAggregator`
  （topic0 由 ABI 計算並以單元測試 pin 住）。`DA_ROLLUP_ID` 可過濾 indexed rollupID
  （DB 的 `batch_number` UNIQUE 假設單一 rollup —— 多 rollup 共用 manager 時必填）。

- **`Tx.Confirmations` 不可用**：SDK 結構體 tag 誤植為 `bson`，JSON 永遠解不出來。
  Confirmer 改用 `getblock` 的 `Block.Confirmations`（tag 正確），或 `tip - height + 1` 回退計算。
- **AIP-11 參數順序**：`MnemonicsToCryptoSeed` 回傳 `(spend, sn, detector, view)`，
  但 `NewRootSeedAccount` 收 `(spend, sn, value=view, detector)` —— wallet loader 已做 remap，勿「修正」。
- **Memo 賦值**：`NewTxDescWithRing` 建構子無 memo 參數，需對 `TxDesc.TxMemo` 欄位賦值（spec F1）。
- **portal_ro 角色**：密碼不能寫死在 migration SQL，改由 `deploy/initdb/01-portal-ro.sh`
  （讀環境變數）建角色；migration 0002 只負責 GRANT（既有 cluster 的 fallback 角色為
  NOLOGIN，需 ops 手動設密碼）。
- **abel_cursor（migration 0003）／serial_number（migration 0004）**：spec schema 之外的
  兩個擴充——Abelian 掃描游標（crash-safe 續掃），與 coin serial number 欄位（鏈上花費
  偵測，對照 `examples/coin` 的 TrackCoins）。
- **Coinbase output 不掃**：relay 錢包預期由轉帳注資；跳過 coinbase 同時避開 200 塊成熟期（F11）。
  若必須用挖礦資金，需另外處理成熟期。
- **Ring 花費時重建、serial 落地**：花費用的 ring 從鏈上重建（`GetRingBlockGroupByHeight`
  → `BuildCoinRings`）；serial number 於 ring group 湊齊時由 scanner 產生並回填 DB，
  供 Vin serial 比對偵測花費。ring block group 未湊齊時回報 wait（不消耗 retry 額度）。
- **交易遺失復原**：confirmer 區分「節點回覆查無此交易」與傳輸層錯誤（後者只重試不重送）；
  判定遺失後 job 回退 PENDING 且該 job 花掉的 coin 復活回 UNSPENT。
- **廣播前先落 txid**：簽名後、送出前即把本地算出的 txid 寫入 job，廣播窗口內 crash
  不會造成重複送出；重啟時殘留的 LOCKED coin 由 submitter 對帳（已上鏈→SPENT，未上鏈→釋放）。
- **單輸入單找零**：submitter 永遠 1 in / 1 out（找零回自己），SDK 的 Sort* 僅在多 descriptor 時呼叫。
- **找零地址每次不同**：`RootSeedAccount.GenerateAbelAddress()` 每次產生不同地址（SDK 設計，
  fresh public randomness）；歸屬判定靠 `ReceiveCoin`（detector key），與地址無關，故無影響。

## 待確認清單（spec §13）

| # | 項目 | 誰確認 | 影響範圍 | 目前預設 |
|---|------|--------|----------|----------|
| C1 | ~~DA commit 合約地址 + 事件 ABI~~ ✅ 已確認 | Vike | `PolygonRollupManager` / `VerifyBatchesTrustedAggregator` | 已實作並實測（batch 221 全程 DONE） |
| C2 | DA RPC endpoint、chain ID、relayer gas 注資 | 團隊夥伴 | `.env` | 空值（啟動時驗證必填） |
| C3 | Abelian endpoint（testnet/mainnet）與 RPC 憑證 | Morris | `.env` | 空值 |
| C4 | 錢包形式：mnemonic 或 key material；privacy level | Morris | `.env`（兩種 loader 都已實作） | `pseudonymous` |
| C5 | Testnet explorer URL | Morris | portal 外連 | mainnet explorer |

另待確認：DA 側 EVM fork 等級（目前合約以 paris 編譯）、`ABEL_TX_FEE_NEUTRINO`（目前對齊官方
0.1 ABEL 常數；SDK `EstimateTxFee` 實為寫死 1_000_000，見 F8）。

## 目錄結構

```
relay/       Go 1.23 relay（cmd/relay + internal/{config,da,abel,registry,store,worker,anchor}）
contracts/   Foundry：QDayAnchorRegistry + 測試 + Deploy script
portal/      Next.js 15 後台（唯讀，iron-session 登入）
deploy/      postgres initdb script（portal_ro 角色）
docs/        sdk-verified-api.md（SDK 事實來源）、實作計畫
doc/         原始規格
```
