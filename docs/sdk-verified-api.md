# abelian-sdk-go-v2 — Verified API Reference (source-extracted)

> Extracted 2026-07-06 from `github.com/pqabelian/abelian-sdk-go-v2` (go 1.23.2) source.
> This file is the ground truth for relay implementation. Do NOT invent APIs not listed here.
> SDK clone used for extraction: `examples/` + `abelian/` package.

## Client (`abelian/client.go`, `methods.go`, `rpc.go`)

```go
func NewClientConfig(endpoint string, options ...ClientOption) *ClientConfig
func WithTimeout(timeout uint64) ClientOption
func WithAuth(username string, password string) ClientOption
func WithTLS(caFile string) ClientOption
func NewClient(config *ClientConfig) (*Client, error)
func (client *Client) Do(method string, params []interface{}, result any) error
```

- Endpoint = full URL (`http://host:port`); SDK POSTs to it directly. Auth = HTTP Basic.
- TLS quirk: `Endpoint` doubles as TLS `ServerName` when TLS enabled.

RPC methods:

```go
func (client *Client) GetChainInfo() (*ChainInfo, error)              // "getinfo"; height = ChainInfo.NumBlocks (json "blocks")
func (client *Client) GetBlockHash(height int64) (string, error)      // "getblockhash"
func (client *Client) GetBlock(blockID string) (*Block, error)        // "getblockabe" verbosity 1
func (client *Client) GetBlockByHeight(height int64) (*Block, error)
func (client *Client) GetBlockBytesByHeight(height int64) ([]byte, error)
func (client *Client) GetRawTx(txID string) (*Tx, error)              // "getrawtransaction" [txID, true]
func (client *Client) SendRawTx(rawTx string) (string, error)         // hex string in, txid out ("sendrawtransactionabe")
func (client *Client) GetRawMempool() ([]string, error)
```

Key structs (`rpc.go`):

```go
type ChainInfo struct { NumBlocks int64 `json:"blocks"`; ... }
type Block struct {
    Height int64 `json:"height"`; Confirmations int64 `json:"confirmations"`
    BlockHash string `json:"hash"`; TxHashes []string `json:"tx"`; RawTxs []*Tx `json:"rawTx"`; ...
}
type Tx struct {
    TxID string `json:"txid"`; BlockHash string `json:"blockhash"`
    Confirmations int64 `bson:"confirmations"`   // ⚠️ BUG: bson tag, NOT json — will NOT unmarshal!
    Version uint32 `json:"version"`; Memo string `json:"memo"`
    Vin []*TxVin `json:"vin"`; Vout []*TxVout `json:"vout"`; ...
}
type TxVout struct { N int64 `json:"n"`; Script string `json:"script"` }  // Script = hex-encoded txOutData
type TxVin struct { TXORing TXORing `json:"prevutxoring"`; SerialNumber string `json:"serialnumber"` }
type TXORing struct { Version int64; BlockHashes []string `json:"blockhashs"`; OutPoints []OutPoint }
type OutPoint struct { TxHash string `json:"txid"`; Index uint8 `json:"index"` }
```

**⚠️ Confirmer implication:** `Tx.Confirmations` never populates from JSON (bson tag bug).
Compute confirmations as `GetBlock(tx.BlockHash).Confirmations`, or `chainHeight - block.Height + 1`.

## Chain params (`abelian/chain.go`)

```go
type NetworkID uint8
const ( MainNet NetworkID = 0; RegressionNet = 1; TestNet = 2; SimNet = 3 )
var NetName2NetID = map[string]NetworkID{"mainnet":…, "regressionnet":…, "testnet":…, "simnet":…}
func GetCoinbaseMaturity() int64                       // hardcoded 200
func EstimateTxFee(txinDescs interface{}, txOutDescs []*TxOutDesc) int64  // hardcoded 1_000_000 neutrino
func NeutrinoToAbel(n int64) float64                   // /1e7
func AbelToNeutrino(a float64) int64                   // *1e7  (1 ABEL = 1e7 neutrino)
func GetTxoRingSizeByBlockHeight(height int64) uint8
func GetBlockNumPerRingGroupByBlockHeight(height int64) uint8
func GetRingBlockHeights(height int64) []int64         // ring group containing height
func GetRingBlockGroupByHeight(client *Client, height int64) ([][]byte, error)  // serialized blocks of group
func BuildCoinRings(serializedBlocksForRingGroup [][]byte) ([]*CoinRing, error)
```

Ring group math: `first = height - height%blockNumPerGroup`; group = `[first, first+n)`.
Coin example gates maturity handling on last block of each group: `height%n == n-1`.

## Accounts (`abelian/account.go`, `aip11.go`)

```go
type AccountPrivacyLevel int
const ( AccountPrivacyLevelFullPrivacyOld = 0; AccountPrivacyLevelFullPrivacy = 1; AccountPrivacyLevelPseudonym = 2 )

type ViewAccount interface {
    ReceiveCoin(txVersion uint32, txOutData []byte) (success bool, v uint64, err error)
    GenerateSerialNumberWithRing(coinID *CoinID, serializedRing []byte) ([]byte, error)
    GenerateSerialNumberWithBlocks(coinID *CoinID, serializedBlocksForRingGroup [][]byte) ([]byte, error)
    GenerateSerialNumbersWithBlocks(coinIDs []*CoinID, serializedBlocksForRingGroup [][]byte) ([][]byte, error)
    ViewKeyMaterial() ([]byte, []byte, []byte)   // snKeySeed, valueKeySeed, detectorKey
    AccountType() AccountType
}
type Account interface {
    ViewAccount
    GenerateAbelAddress() ([]byte, error)
    SpendKeyMaterial() []byte
    ViewAccount() ViewAccount
}

func NewAccount(networkID NetworkID, accountPrivacyLevel AccountPrivacyLevel) (Account, error)
func NewRootSeedAccount(networkID NetworkID, lvl AccountPrivacyLevel,
    coinSpendKeySeed, coinSerialNumberKeySeed, coinValueKeySeed, coinDetectorKey []byte) *RootSeedAccount
func NewRootSeedViewAccount(netID NetworkID, lvl AccountPrivacyLevel,
    coinSerialNumberKeySeed, coinValueKeySeed, coinDetectorKey []byte) *RootSeedViewAccount
```

AIP-11 mnemonic (`aip11.go`):

```go
func MnemonicsToCryptoSeed(mnemonics []string) (SpendKeyRootSeed, SerialNoKeyRootSeed, DetectorRootKey, ViewKeyRootSeed []byte, err error)
```

**⚠️ Param-order remap (critical):** `MnemonicsToCryptoSeed` returns `(spend, serialNo, DETECTOR, VIEW)`
but `NewRootSeedAccount` takes `(spend, serialNo, VALUE(=view), DETECTOR)`. Correct wiring:

```go
spend, sn, detector, view, err := abelian.MnemonicsToCryptoSeed(words)
acct := abelian.NewRootSeedAccount(netID, lvl, spend, sn, view, detector)
```

**⚠️ Address instability:** `RootSeedAccount.GenerateAbelAddress()` uses fresh public randomness —
every call returns a DIFFERENT address. Generate change addresses per-tx freely; ownership detection
is via `ReceiveCoin` (detector key), not address comparison.

Key-material persistence convention (from `examples/database/account.go`): store hex of the 4 seeds:
`spend_key_seed, sn_key_seed, value_key_seed, detector_key` + `network_id`, `account_privacy_level`.
Our `ABEL_WALLET_KEY_FILE` JSON mirrors those field names.

## Coins (`abelian/coin.go`)

```go
type CoinID struct { TxID string; Index uint8 }
func NewCoinID(txHash string, index uint8) *CoinID
func (id CoinID) String() string                       // "txid:index"
type Coin struct {
    TxVersion uint32; TxID string; Index uint8; BlockHash string; BlockHeight int64
    Value int64; SerialNumber string; TxVoutData []byte; RingID string; RingIndex uint8
}
func NewCoin(txVersion uint32, txID string, index uint8, blockHash string,
    blockHeight int64, value int64, serialNumber string, txVoutData []byte) *Coin
func (coin *Coin) SetRingInfo(ringID string, ringIndex uint8)
```

Coin scan loop (from `examples/coin/main.go`): per block → per `TxHashes[i]` → `GetRawTx` →
per `Vout[j]`: `txOutData, _ := hex.DecodeString(tx.Vout[j].Script)` →
`ok, value, err := viewAccount.ReceiveCoin(uint32(tx.Version), txOutData)`.
`i == 0` in block = coinbase tx (maturity 200). Spent detection: match `tx.Vin[k].SerialNumber`
against stored coin serial numbers (+ outpoint match in `Vin[k].TXORing.OutPoints`).

## Transactions (`abelian/transaction.go`)

```go
type TxOutDesc struct { AbelAddress *AbelAddress; CoinValue int64 }
type TxInDescWithRing struct {
    BlockHeight int64; BlockID string; TxVersion uint32; TxID string; TxOutIndex uint8
    TxOutData []byte; CoinValue int64; CoinSerialNumber []byte; TxoRing *CoinRing
}
type TxDescWithRing struct {
    TxInDescs []*TxInDescWithRing; TxOutDescs []*TxOutDesc; TxFee int64; TxMemo []byte
}
func NewTxDescWithRing(txInDescs []*TxInDescWithRing, txOutDescs []*TxOutDesc, txFee int64) *TxDescWithRing
// ⚠️ constructor has NO memo param — assign txDesc.TxMemo = []byte(...) after construction.

func SortTxInDescWithRing(txIndescs []*TxInDescWithRing) error
func SortTxOutDesc(txOutdescs []*TxOutDesc) error

type UnsignedRawTx struct { Data []byte }
type SignedRawTx struct { Data []byte; TxID string }
func GenerateUnsignedRawTxWithRing(txDesc *TxDescWithRing) (*UnsignedRawTx, error)
func GenerateSignedRawTx(unsignedRawTx *UnsignedRawTx, signerAccounts []Account) (*SignedRawTx, error)

type CoinRing struct {
    Version uint32; RingBlockHeight int32; CoinIDRing *CoinIDRing
    SerializedTxOuts [][]byte; IsCoinbase bool
}
func NewCoinRing(ringVersion uint32, ringBlockHeight int64, blockIDs []string,
    coins []*CoinID, serializedTxOuts [][]byte, fromCoinbase bool) (*CoinRing, error)
func (ring *CoinRing) RingId() (string, error)
func (ring *CoinRing) Serialize(w io.Writer) error
type CoinIDRing struct { Version uint32; BlockIDs []string; CoinIDs []*CoinID }

func NewAbelAddress(data []byte) (*AbelAddress, error)   // address.go
```

Send flow (verbatim from `examples/transaction/main.go`):

```go
err = abelian.SortTxInDescWithRing(txInDescs)
err = abelian.SortTxOutDesc(txOutDescs)
txDesc := abelian.NewTxDescWithRing(txInDescs, txOutDescs, fee)
txDesc.TxMemo = []byte(memoString)                        // memo assigned post-construction
unsignedRawTx, err := abelian.GenerateUnsignedRawTxWithRing(txDesc)
signedRawTx, err := abelian.GenerateSignedRawTx(unsignedRawTx, []abelian.Account{acct})
txHash, err := client.SendRawTx(hex.EncodeToString(signedRawTx.Data))
// txHash == signedRawTx.TxID (example asserts equality)
```

Ring for an input coin (from `examples/coin/main.go` `BuildCoinRingsWithBlockHeight`):

```go
serializedBlocks, err := abelian.GetRingBlockGroupByHeight(client, coin.BlockHeight)
rings, err := abelian.BuildCoinRings(serializedBlocks)
// find ring whose CoinIDRing.CoinIDs contains coin's (TxID, Index); note ringIndex
// serial number:
var w bytes.Buffer
err = ring.Serialize(&w)
sn, err := viewAccount.GenerateSerialNumberWithRing(coin.ID(), w.Bytes())
```

Rings only become buildable once the coin's whole ring block group is complete
(`height%blockNumPerGroup == blockNumPerGroup-1` for the group's last height).

## Known SDK example bugs (do not copy)

- `examples/rpcclient/main.go` passes `int32` to `GetBlockHash(int64)` — doesn't compile.
- `examples/database/coin.go` `LoadImmatureCoinbaseCoins` scan/column mismatch.
- `Tx.Confirmations` bson-tag bug (above).
