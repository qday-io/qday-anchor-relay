// Package admin exposes a small internal-network-only HTTP API for dev/ops
// tooling that needs the Abelian PQ crypto stack (which only exists in Go):
// generating test accounts and revealing the relay wallet's receiving address.
// It must NEVER be published outside the compose network — the portal proxies
// it behind its own session auth.
package admin

import (
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// AddressAccount is the slice of abelian.Account the server needs.
type AddressAccount interface {
	GenerateAbelAddress() ([]byte, error)
}

type Server struct {
	network abelian.NetworkID
	privacy abelian.AccountPrivacyLevel
	wallet  AddressAccount
	// walletSource describes where the relay signing wallet comes from
	// ("mnemonic" or "keyfile") — shown in the portal settings section.
	walletSource string
	// fixedAddr, when non-nil, is the wallet's deterministic (AIP-11 sequence
	// 0) receiving address — stable across restarts, safe to hand out.
	fixedAddr []byte
	log       *slog.Logger
}

func NewServer(network abelian.NetworkID, privacy abelian.AccountPrivacyLevel, wallet AddressAccount, walletSource string, fixedAddr []byte, log *slog.Logger) *Server {
	return &Server{network: network, privacy: privacy, wallet: wallet, walletSource: walletSource, fixedAddr: fixedAddr, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/generate-account", s.handleGenerateAccount)
	mux.HandleFunc("GET /admin/wallet-address", s.handleWalletAddress)
	return mux
}

type generatedAccount struct {
	Mnemonic   string `json:"mnemonic"`
	AddressHex string `json:"address_hex"`
	Network    string `json:"network"`
	Privacy    int    `json:"privacy_level"`
}

// handleGenerateAccount mints a brand-new AIP-11 test account. The mnemonic is
// returned once and never stored anywhere server-side.
func (s *Server) handleGenerateAccount(w http.ResponseWriter, r *http.Request) {
	entropy, err := abelian.GenerateEntropySeed()
	if err != nil {
		s.fail(w, "generate entropy", err)
		return
	}
	words, err := abelian.EntropySeedToMnemonics(entropy)
	if err != nil {
		s.fail(w, "entropy to mnemonic", err)
		return
	}
	// Verified seed remap (docs/sdk-verified-api.md): returns (spend, sn,
	// DETECTOR, VIEW); constructor wants (spend, sn, value=view, detector).
	spend, sn, detector, view, err := abelian.MnemonicsToCryptoSeed(words)
	if err != nil {
		s.fail(w, "mnemonic to seeds", err)
		return
	}
	account := abelian.NewRootSeedAccount(s.network, s.privacy, spend, sn, view, detector)
	addr, err := account.GenerateAbelAddress()
	if err != nil {
		s.fail(w, "derive address", err)
		return
	}
	writeJSON(w, generatedAccount{
		Mnemonic:   strings.Join(words, " "),
		AddressHex: hex.EncodeToString(addr),
		Network:    s.network.String(),
		Privacy:    int(s.privacy),
	})
}

type walletAddress struct {
	AddressHex string `json:"address_hex"`
	Network    string `json:"network"`
	Privacy    int    `json:"privacy_level"`
	Source     string `json:"wallet_source"` // mnemonic | keyfile
	// Stable: true means AddressHex is deterministic (same on every request
	// and across restarts) — safe to share as THE funding address.
	Stable bool   `json:"stable"`
	Note   string `json:"note"`
}

// handleWalletAddress returns the relay wallet's receiving address. Mnemonic
// wallets get the deterministic sequence-0 address (stable); key-file wallets
// fall back to a fresh address per request (all detected by ReceiveCoin).
func (s *Server) handleWalletAddress(w http.ResponseWriter, r *http.Request) {
	if s.fixedAddr != nil {
		writeJSON(w, walletAddress{
			AddressHex: hex.EncodeToString(s.fixedAddr),
			Network:    s.network.String(),
			Privacy:    int(s.privacy),
			Source:     s.walletSource,
			Stable:     true,
			Note:       "fixed funding address (deterministic, same across restarts) — share this one",
		})
		return
	}
	addr, err := s.wallet.GenerateAbelAddress()
	if err != nil {
		s.fail(w, "derive relay wallet address", err)
		return
	}
	writeJSON(w, walletAddress{
		AddressHex: hex.EncodeToString(addr),
		Network:    s.network.String(),
		Privacy:    int(s.privacy),
		Source:     s.walletSource,
		Stable:     false,
		Note:       "fund the relay by sending ABEL to this address (fresh per request; all belong to the relay wallet)",
	})
}

func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	s.log.Error("admin: "+what, "err", err)
	http.Error(w, `{"error":"`+what+`"}`, http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
