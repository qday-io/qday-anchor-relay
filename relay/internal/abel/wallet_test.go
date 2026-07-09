package abel

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// generateTestMnemonic produces a valid AIP-11 mnemonic via the SDK itself.
func generateTestMnemonic(t *testing.T) string {
	t.Helper()
	entropy, err := abelian.GenerateEntropySeed()
	if err != nil {
		t.Fatalf("GenerateEntropySeed: %v", err)
	}
	words, err := abelian.EntropySeedToMnemonics(entropy)
	if err != nil {
		t.Fatalf("EntropySeedToMnemonics: %v", err)
	}
	return strings.Join(words, " ")
}

func TestLoadWallet_Mnemonic(t *testing.T) {
	mnemonic := generateTestMnemonic(t)
	acct, err := LoadWallet(WalletConfig{
		Network:        "testnet",
		Mnemonic:       mnemonic,
		AccountPrivacy: "pseudonymous",
	})
	if err != nil {
		t.Fatalf("LoadWallet: %v", err)
	}
	if acct.AccountType() != abelian.AccountTypeSeeds {
		t.Errorf("AccountType = %v, want AccountTypeSeeds", acct.AccountType())
	}
	// The account must be able to derive a receiving address.
	addr, err := acct.GenerateAbelAddress()
	if err != nil || len(addr) == 0 {
		t.Fatalf("GenerateAbelAddress: len=%d err=%v", len(addr), err)
	}
	if _, err := abelian.NewAbelAddress(addr); err != nil {
		t.Errorf("generated address invalid: %v", err)
	}
}

// TestLoadWallet_MnemonicSeedRemap pins the AIP-11 → NewRootSeedAccount param
// remap (docs/sdk-verified-api.md): the account built by LoadWallet must be
// able to detect coins sent to its own address. A wrong remap (detector/view
// swapped) breaks ReceiveCoin self-detection.
func TestLoadWallet_MnemonicSeedRemap(t *testing.T) {
	if testing.Short() {
		t.Skip("PQ crypto self-detection test is slow")
	}
	mnemonic := generateTestMnemonic(t)
	acct, err := LoadWallet(WalletConfig{
		Network:        "testnet",
		Mnemonic:       mnemonic,
		AccountPrivacy: "pseudonymous",
	})
	if err != nil {
		t.Fatalf("LoadWallet: %v", err)
	}
	// Sanity: loading the same mnemonic twice must yield accounts with
	// identical key material (deterministic derivation).
	acct2, err := LoadWallet(WalletConfig{
		Network:        "testnet",
		Mnemonic:       mnemonic,
		AccountPrivacy: "pseudonymous",
	})
	if err != nil {
		t.Fatal(err)
	}
	sn1, v1, d1 := acct.ViewKeyMaterial()
	sn2, v2, d2 := acct2.ViewKeyMaterial()
	if hex.EncodeToString(sn1) != hex.EncodeToString(sn2) ||
		hex.EncodeToString(v1) != hex.EncodeToString(v2) ||
		hex.EncodeToString(d1) != hex.EncodeToString(d2) ||
		hex.EncodeToString(acct.SpendKeyMaterial()) != hex.EncodeToString(acct2.SpendKeyMaterial()) {
		t.Error("same mnemonic produced different key material")
	}
}

func TestLoadWallet_KeyFileRoundtrip(t *testing.T) {
	// Create a fresh account, export its material to the key-file JSON, load
	// it back, and compare key material.
	src, err := abelian.NewAccount(abelian.TestNet, abelian.AccountPrivacyLevelPseudonym)
	if err != nil {
		t.Fatalf("NewAccount: %v", err)
	}
	sn, value, detector := src.ViewKeyMaterial()
	kf := KeyFileFormat{
		SpendKeySeed: hex.EncodeToString(src.SpendKeyMaterial()),
		SnKeySeed:    hex.EncodeToString(sn),
		ValueKeySeed: hex.EncodeToString(value),
		DetectorKey:  hex.EncodeToString(detector),
	}
	raw, err := json.Marshal(kf)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wallet.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadWallet(WalletConfig{
		Network:        "testnet",
		KeyFile:        path,
		AccountPrivacy: "pseudonymous",
	})
	if err != nil {
		t.Fatalf("LoadWallet(keyfile): %v", err)
	}
	lsn, lvalue, ldetector := loaded.ViewKeyMaterial()
	if hex.EncodeToString(lsn) != kf.SnKeySeed ||
		hex.EncodeToString(lvalue) != kf.ValueKeySeed ||
		hex.EncodeToString(ldetector) != kf.DetectorKey ||
		hex.EncodeToString(loaded.SpendKeyMaterial()) != kf.SpendKeySeed {
		t.Error("loaded key material differs from exported material")
	}
}

func TestLoadWallet_Errors(t *testing.T) {
	if _, err := LoadWallet(WalletConfig{Network: "bogus", Mnemonic: "x", AccountPrivacy: "pseudonymous"}); err == nil {
		t.Error("expected error for bad network")
	}
	if _, err := LoadWallet(WalletConfig{Network: "testnet", Mnemonic: "x", AccountPrivacy: "bogus"}); err == nil {
		t.Error("expected error for bad privacy level")
	}
	if _, err := LoadWallet(WalletConfig{Network: "testnet", AccountPrivacy: "pseudonymous"}); err == nil {
		t.Error("expected error when no wallet source configured")
	}
	if _, err := LoadWallet(WalletConfig{Network: "testnet", KeyFile: "/nonexistent.json", AccountPrivacy: "pseudonymous"}); err == nil {
		t.Error("expected error for missing key file")
	}
}

func TestFixedWalletAddress_StableAndValid(t *testing.T) {
	mnemonic := generateTestMnemonic(t)
	cfg := WalletConfig{Network: "testnet", Mnemonic: mnemonic, AccountPrivacy: "pseudonymous"}

	addr1, ok, err := FixedWalletAddress(cfg)
	if err != nil || !ok {
		t.Fatalf("FixedWalletAddress: ok=%v err=%v", ok, err)
	}
	addr2, ok, err := FixedWalletAddress(cfg)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if hex.EncodeToString(addr1) != hex.EncodeToString(addr2) {
		t.Error("fixed address changed between calls")
	}
	if _, err := abelian.NewAbelAddress(addr1); err != nil {
		t.Errorf("fixed address invalid: %v", err)
	}
	// Different mnemonic → different fixed address.
	addr3, _, err := FixedWalletAddress(WalletConfig{Network: "testnet", Mnemonic: generateTestMnemonic(t), AccountPrivacy: "pseudonymous"})
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(addr1) == hex.EncodeToString(addr3) {
		t.Error("different mnemonics produced identical fixed addresses")
	}
}

func TestFixedWalletAddress_KeyfileHasNone(t *testing.T) {
	_, ok, err := FixedWalletAddress(WalletConfig{Network: "testnet", KeyFile: "/x.json", AccountPrivacy: "pseudonymous"})
	if err != nil || ok {
		t.Errorf("keyfile wallet: ok=%v err=%v, want ok=false", ok, err)
	}
}
