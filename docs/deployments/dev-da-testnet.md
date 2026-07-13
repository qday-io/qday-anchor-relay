# Dev 部署紀錄 — DA testnet（2026-07-08）

> 開發環境部署。所使用的 relayer 私鑰為團隊公用的**開發專用**測試私鑰（Anvil 公開帳號），
> 依團隊聲明絕不用於正式 testnet/mainnet。

| 項目 | 值 |
|---|---|
| DA RPC | `http://159.138.121.45:1545` |
| Chain ID | `31337` |
| QDayAnchorRegistry | `0x59b670e9fA9D0A427751Af201D676719a970857b` |
| MockBatchCommitter（暫代 commit 合約） | `0xa85233C63b9Ee964Add6F2cffe00Fd84eb32338f` |
| Relayer EOA（已 setRelayer） | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` |
| Portal | `http://localhost:3100`（本機 compose；帳號 admin） |

## 已驗證

- `recordAnchor` → `Anchored` event → `anchorOf`/`isAnchored` 回讀，鏈上實測通過
  （tx `0x964065f3ab1a79220b06fc508a153dea9419b30a2df4c248ac0b9c235eacbdbe`，測試資料 batch=1）。
- Poller 端對端：MockBatchCommitter 發 3 筆 `BatchCommitted` → relay 掃到 → `anchor_jobs`
  3 筆 PENDING（state root 比對正確）→ portal dashboard 顯示。
- Submitter 等待語義：無 Abelian 資金時記 `submit waiting`，不消耗 retry 額度。

## 更新（2026-07-13）：端對端已通

上面「尚缺」三項全部解決：

| 項目 | 值 |
|---|---|
| Abelian RPC | `https://testnet2-rpc-00.pqabelian.io`（Aconcagua，TxVersion 3） |
| 真 commit 來源 | `PolygonRollupManager` `0x959922bE…07B1` / `VerifyBatchesTrustedAggregator` |
| 注資 | 30,000 ABEL（tx `3a0c6b6d…371c`，固定地址 + ANS 已註冊） |
| **第一筆完整錨定** | batch 221：Abel tx `8fa60aad…514b` @737568 → record tx `0x536565…babc` → **DONE**（全程 7 分鐘） |

鏈上驗證：`anchorOf(0x898007fd…) = (221, 8fa60aad…, 737568, …)`；Abel memo =
`QDAY2ANCHOR|v1|221|0x898007fd…`。

## 注意

- 此鏈出塊似乎跟交易走（不定期），dev 用 `DA_CONFIRM_DEPTH=2` 加速；正式環境用 12。
- mock 合約任何人可呼叫 `commit()`，僅供 poller 測試。
