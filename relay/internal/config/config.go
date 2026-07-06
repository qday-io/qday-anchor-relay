// Package config parses and validates relay configuration from the environment
// (spec §4). All 【TO-CONFIRM】 values are plain config — nothing is hardcoded.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Getenv abstracts os.Getenv so tests can inject envs without mutating the process.
type Getenv func(key string) string

type Postgres struct {
	Host, Port, DB, User, Password string
}

// DSN renders a pgx/lib-pq compatible connection string.
func (p Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(p.User), url.QueryEscape(p.Password), p.Host, p.Port, p.DB)
}

type DA struct {
	RPCURL         string
	ChainID        int64
	CommitContract string
	ConfirmDepth   uint64
	PollInterval   time.Duration
	StartBlock     uint64
}

type Registry struct {
	Address           string
	RelayerPrivateKey string
}

type Abel struct {
	Network        string // mainnet | testnet | regressionnet | simnet
	RPCEndpoint    string
	RPCUser        string
	RPCPass        string
	TLSCertPath    string
	ConfirmDepth   int64
	WalletMnemonic string
	WalletKeyFile  string
	AccountPrivacy string // pseudonymous | full-privacy
	TxFeeNeutrino  int64
}

type Config struct {
	Postgres         Postgres
	DA               DA
	Registry         Registry
	Abel             Abel
	RetryMax         int
	RetryBackoffBase time.Duration
	MemoPrefix       string
}

var validNetworks = map[string]bool{
	"mainnet": true, "testnet": true, "regressionnet": true, "simnet": true,
}

var validPrivacy = map[string]bool{
	"pseudonymous": true, "full-privacy": true,
}

// Load reads configuration via getenv, applies spec defaults, and validates.
func Load(getenv Getenv) (*Config, error) {
	var errs []error

	require := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			errs = append(errs, fmt.Errorf("missing required env %s", key))
		}
		return v
	}
	optional := func(key, def string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			return def
		}
		return v
	}
	parseInt := func(key, def string, bits int) int64 {
		raw := optional(key, def)
		n, err := strconv.ParseInt(raw, 10, bits)
		if err != nil {
			errs = append(errs, fmt.Errorf("env %s: not an integer: %q", key, raw))
		}
		return n
	}
	parseUint := func(key, def string) uint64 {
		raw := optional(key, def)
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("env %s: not a non-negative integer: %q", key, raw))
		}
		return n
	}

	cfg := &Config{
		Postgres: Postgres{
			Host:     optional("POSTGRES_HOST", "postgres"),
			Port:     optional("POSTGRES_PORT", "5432"),
			DB:       optional("POSTGRES_DB", "anchor"),
			User:     optional("POSTGRES_USER", "anchor"),
			Password: require("POSTGRES_PASSWORD"),
		},
		DA: DA{
			RPCURL:         require("DA_RPC_URL"),
			ChainID:        parseInt("DA_CHAIN_ID", "0", 64),
			CommitContract: require("DA_COMMIT_CONTRACT"),
			ConfirmDepth:   parseUint("DA_CONFIRM_DEPTH", "12"),
			PollInterval:   time.Duration(parseInt("DA_POLL_INTERVAL_SECONDS", "10", 64)) * time.Second,
			StartBlock:     parseUint("DA_START_BLOCK", "0"),
		},
		Registry: Registry{
			Address:           require("ANCHOR_REGISTRY_ADDRESS"),
			RelayerPrivateKey: require("RELAYER_PRIVATE_KEY"),
		},
		Abel: Abel{
			Network:        optional("ABEL_NETWORK", "testnet"),
			RPCEndpoint:    require("ABEL_RPC_ENDPOINT"),
			RPCUser:        getenv("ABEL_RPC_USER"),
			RPCPass:        getenv("ABEL_RPC_PASS"),
			TLSCertPath:    getenv("ABEL_RPC_TLS_CERT_PATH"),
			ConfirmDepth:   parseInt("ABEL_CONFIRM_DEPTH", "6", 64),
			WalletMnemonic: strings.TrimSpace(getenv("ABEL_WALLET_MNEMONIC")),
			WalletKeyFile:  strings.TrimSpace(getenv("ABEL_WALLET_KEY_FILE")),
			AccountPrivacy: optional("ABEL_ACCOUNT_PRIVACY", "pseudonymous"),
			TxFeeNeutrino:  parseInt("ABEL_TX_FEE_NEUTRINO", "1000000", 64),
		},
		RetryMax:         int(parseInt("RETRY_MAX", "5", 32)),
		RetryBackoffBase: time.Duration(parseInt("RETRY_BACKOFF_BASE_SECONDS", "30", 64)) * time.Second,
		MemoPrefix:       optional("MEMO_PREFIX", "QDAY2ANCHOR"),
	}

	if cfg.DA.ChainID == 0 {
		errs = append(errs, errors.New("missing required env DA_CHAIN_ID"))
	}
	if !validNetworks[cfg.Abel.Network] {
		errs = append(errs, fmt.Errorf("env ABEL_NETWORK: invalid network %q", cfg.Abel.Network))
	}
	if !validPrivacy[cfg.Abel.AccountPrivacy] {
		errs = append(errs, fmt.Errorf("env ABEL_ACCOUNT_PRIVACY: must be pseudonymous or full-privacy, got %q", cfg.Abel.AccountPrivacy))
	}
	if cfg.Abel.WalletMnemonic == "" && cfg.Abel.WalletKeyFile == "" {
		errs = append(errs, errors.New("wallet: set ABEL_WALLET_MNEMONIC or ABEL_WALLET_KEY_FILE"))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}
