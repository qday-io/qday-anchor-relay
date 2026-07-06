package abel

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// WalletConfig is the wallet-relevant slice of config (avoids importing
// internal/config here).
type WalletConfig struct {
	Network        string // mainnet | testnet | regressionnet | simnet
	Mnemonic       string // AIP-11 mnemonic, space separated (option A)
	KeyFile        string // key material JSON path (option B)
	AccountPrivacy string // pseudonymous | full-privacy
}

// KeyFileFormat is the JSON layout of ABEL_WALLET_KEY_FILE. Field names mirror
// the SDK's own persistence example (examples/database/account.go): the four
// root seeds, hex encoded.
type KeyFileFormat struct {
	NetworkID           *uint8 `json:"network_id,omitempty"`            // optional; config wins when unset
	AccountPrivacyLevel *int   `json:"account_privacy_level,omitempty"` // optional; config wins when unset
	SpendKeySeed        string `json:"spend_key_seed"`
	SnKeySeed           string `json:"sn_key_seed"`
	ValueKeySeed        string `json:"value_key_seed"`
	DetectorKey         string `json:"detector_key"`
}

func networkID(name string) (abelian.NetworkID, error) {
	id, ok := abelian.NetName2NetID[name]
	if !ok {
		return 0, fmt.Errorf("abel: unknown network %q", name)
	}
	return id, nil
}

func privacyLevel(name string) (abelian.AccountPrivacyLevel, error) {
	switch name {
	case "pseudonymous":
		return abelian.AccountPrivacyLevelPseudonym, nil
	case "full-privacy":
		return abelian.AccountPrivacyLevelFullPrivacy, nil
	default:
		return 0, fmt.Errorf("abel: unknown privacy level %q", name)
	}
}

// LoadWallet builds the relay signing account from either an AIP-11 mnemonic
// or a key material JSON file (spec §7.2 — both supported, config picks one).
func LoadWallet(cfg WalletConfig) (abelian.Account, error) {
	netID, err := networkID(cfg.Network)
	if err != nil {
		return nil, err
	}
	lvl, err := privacyLevel(cfg.AccountPrivacy)
	if err != nil {
		return nil, err
	}

	switch {
	case cfg.Mnemonic != "":
		return loadFromMnemonic(netID, lvl, cfg.Mnemonic)
	case cfg.KeyFile != "":
		return loadFromKeyFile(netID, lvl, cfg.KeyFile)
	default:
		return nil, fmt.Errorf("abel: no wallet configured (mnemonic or key file required)")
	}
}

func loadFromMnemonic(netID abelian.NetworkID, lvl abelian.AccountPrivacyLevel, mnemonic string) (abelian.Account, error) {
	words := strings.Fields(mnemonic)
	// Verified return order (docs/sdk-verified-api.md): spend, serialNo,
	// DETECTOR, VIEW — while NewRootSeedAccount wants (spend, sn, value=view,
	// detector). The remap below is deliberate; do not "fix" it.
	spend, sn, detector, view, err := abelian.MnemonicsToCryptoSeed(words)
	if err != nil {
		return nil, fmt.Errorf("abel: mnemonic to crypto seed: %w", err)
	}
	return abelian.NewRootSeedAccount(netID, lvl, spend, sn, view, detector), nil
}

func loadFromKeyFile(netID abelian.NetworkID, lvl abelian.AccountPrivacyLevel, path string) (abelian.Account, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("abel: read key file: %w", err)
	}
	var kf KeyFileFormat
	if err := json.Unmarshal(raw, &kf); err != nil {
		return nil, fmt.Errorf("abel: parse key file: %w", err)
	}
	decode := func(field, val string) []byte {
		if err != nil {
			return nil
		}
		var b []byte
		b, err = hex.DecodeString(val)
		if err != nil {
			err = fmt.Errorf("abel: key file field %s is not hex: %w", field, err)
		}
		return b
	}
	spend := decode("spend_key_seed", kf.SpendKeySeed)
	sn := decode("sn_key_seed", kf.SnKeySeed)
	value := decode("value_key_seed", kf.ValueKeySeed)
	detector := decode("detector_key", kf.DetectorKey)
	if err != nil {
		return nil, err
	}
	if kf.NetworkID != nil {
		netID = abelian.NetworkID(*kf.NetworkID)
	}
	if kf.AccountPrivacyLevel != nil {
		lvl = abelian.AccountPrivacyLevel(*kf.AccountPrivacyLevel)
	}
	return abelian.NewRootSeedAccount(netID, lvl, spend, sn, value, detector), nil
}
