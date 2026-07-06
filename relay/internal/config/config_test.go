package config

import (
	"testing"
	"time"
)

func validEnv() map[string]string {
	return map[string]string{
		"POSTGRES_HOST":              "localhost",
		"POSTGRES_PORT":              "5432",
		"POSTGRES_DB":                "anchor",
		"POSTGRES_USER":              "anchor",
		"POSTGRES_PASSWORD":          "pw",
		"DA_RPC_URL":                 "http://da:8545",
		"DA_CHAIN_ID":                "1337",
		"DA_COMMIT_CONTRACT":         "0x1111111111111111111111111111111111111111",
		"DA_CONFIRM_DEPTH":           "12",
		"DA_POLL_INTERVAL_SECONDS":   "10",
		"DA_START_BLOCK":             "100",
		"ANCHOR_REGISTRY_ADDRESS":    "0x2222222222222222222222222222222222222222",
		"RELAYER_PRIVATE_KEY":        "4c0883a69102937d6231471b5dbb6204fe51296170827936ea5cce4b76994b0f",
		"ABEL_NETWORK":               "testnet",
		"ABEL_RPC_ENDPOINT":          "http://abec:8667",
		"ABEL_RPC_USER":              "u",
		"ABEL_RPC_PASS":              "p",
		"ABEL_CONFIRM_DEPTH":         "6",
		"ABEL_WALLET_MNEMONIC":       "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		"ABEL_ACCOUNT_PRIVACY":       "pseudonymous",
		"ABEL_TX_FEE_NEUTRINO":       "1000000",
		"RETRY_MAX":                  "5",
		"RETRY_BACKOFF_BASE_SECONDS": "30",
	}
}

func loadFrom(t *testing.T, env map[string]string) (*Config, error) {
	t.Helper()
	return Load(func(key string) string { return env[key] })
}

func TestLoad_Valid(t *testing.T) {
	cfg, err := loadFrom(t, validEnv())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DA.ConfirmDepth != 12 || cfg.DA.PollInterval != 10*time.Second || cfg.DA.StartBlock != 100 {
		t.Errorf("DA config mismatch: %+v", cfg.DA)
	}
	if cfg.Abel.Network != "testnet" || cfg.Abel.ConfirmDepth != 6 || cfg.Abel.TxFeeNeutrino != 1000000 {
		t.Errorf("Abel config mismatch: %+v", cfg.Abel)
	}
	if cfg.RetryMax != 5 || cfg.RetryBackoffBase != 30*time.Second {
		t.Errorf("retry config mismatch: %+v", cfg)
	}
	if cfg.Postgres.DSN() != "postgres://anchor:pw@localhost:5432/anchor?sslmode=disable" {
		t.Errorf("DSN = %q", cfg.Postgres.DSN())
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	for _, key := range []string{"DA_RPC_URL", "ABEL_RPC_ENDPOINT", "POSTGRES_PASSWORD", "DA_COMMIT_CONTRACT", "ANCHOR_REGISTRY_ADDRESS", "RELAYER_PRIVATE_KEY"} {
		env := validEnv()
		delete(env, key)
		if _, err := loadFrom(t, env); err == nil {
			t.Errorf("Load() without %s expected error", key)
		}
	}
}

func TestLoad_WalletEitherMnemonicOrKeyFile(t *testing.T) {
	env := validEnv()
	delete(env, "ABEL_WALLET_MNEMONIC")
	if _, err := loadFrom(t, env); err == nil {
		t.Error("expected error when neither mnemonic nor key file set")
	}
	env["ABEL_WALLET_KEY_FILE"] = "/keys/wallet.json"
	cfg, err := loadFrom(t, env)
	if err != nil {
		t.Fatalf("Load() with key file error = %v", err)
	}
	if cfg.Abel.WalletKeyFile != "/keys/wallet.json" {
		t.Errorf("WalletKeyFile = %q", cfg.Abel.WalletKeyFile)
	}
}

func TestLoad_InvalidNetwork(t *testing.T) {
	env := validEnv()
	env["ABEL_NETWORK"] = "bogusnet"
	if _, err := loadFrom(t, env); err == nil {
		t.Error("expected error for invalid ABEL_NETWORK")
	}
}

func TestLoad_InvalidPrivacy(t *testing.T) {
	env := validEnv()
	env["ABEL_ACCOUNT_PRIVACY"] = "invisible"
	if _, err := loadFrom(t, env); err == nil {
		t.Error("expected error for invalid ABEL_ACCOUNT_PRIVACY")
	}
}

func TestLoad_Defaults(t *testing.T) {
	env := validEnv()
	delete(env, "DA_CONFIRM_DEPTH")
	delete(env, "DA_POLL_INTERVAL_SECONDS")
	delete(env, "ABEL_CONFIRM_DEPTH")
	delete(env, "RETRY_MAX")
	delete(env, "RETRY_BACKOFF_BASE_SECONDS")
	delete(env, "ABEL_TX_FEE_NEUTRINO")
	cfg, err := loadFrom(t, env)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DA.ConfirmDepth != 12 || cfg.DA.PollInterval != 10*time.Second ||
		cfg.Abel.ConfirmDepth != 6 || cfg.RetryMax != 5 ||
		cfg.RetryBackoffBase != 30*time.Second || cfg.Abel.TxFeeNeutrino != 1000000 {
		t.Errorf("defaults mismatch: %+v", cfg)
	}
}
