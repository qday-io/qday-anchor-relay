// Package admin exposes a small internal-network-only HTTP API for dev/ops
// tooling that needs the Abelian PQ crypto stack (which only exists in Go):
// generating test accounts and revealing the relay wallet's receiving address.
// It must NEVER be published outside the compose network — the portal proxies
// it behind its own session auth.
package admin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
)

// AddressAccount is the slice of abelian.Account the server needs.
type AddressAccount interface {
	GenerateAbelAddress() ([]byte, error)
}

// ANSRegistrar registers full addresses with the Abelian Name Service so the
// short form resolves in wallets. Nil disables registration.
type ANSRegistrar interface {
	Register(ctx context.Context, fullAddressHex string) (shortAddressHex string, err error)
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
	ans       ANSRegistrar // nil = registration disabled
	log       *slog.Logger
}

func NewServer(network abelian.NetworkID, privacy abelian.AccountPrivacyLevel, wallet AddressAccount, walletSource string, fixedAddr []byte, ans ANSRegistrar, log *slog.Logger) *Server {
	return &Server{network: network, privacy: privacy, wallet: wallet, walletSource: walletSource, fixedAddr: fixedAddr, ans: ans, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/generate-account", s.handleGenerateAccount)
	mux.HandleFunc("GET /admin/wallet-address", s.handleWalletAddress)
	return mux
}

type generatedAccount struct {
	Mnemonic        string `json:"mnemonic"`
	AddressHex      string `json:"address_hex"`
	ShortAddressHex string `json:"short_address_hex,omitempty"`
	// ANSRegistered: the address was registered with the name service, so the
	// short form resolves in wallets.
	ANSRegistered bool   `json:"ans_registered"`
	Network       string `json:"network"`
	Privacy       int    `json:"privacy_level"`
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
	addrHex := hex.EncodeToString(addr)

	shortHex := ""
	if short, err := abel.ShortAddress(addr); err == nil {
		shortHex = hex.EncodeToString(short)
	}
	// Register with ANS so the short form resolves in wallets right away.
	ansOK := false
	if s.ans != nil {
		if _, err := s.ans.Register(r.Context(), addrHex); err != nil {
			s.log.Warn("admin: ANS registration for generated account failed", "err", err)
		} else {
			ansOK = true
		}
	}

	writeJSON(w, generatedAccount{
		Mnemonic:        strings.Join(words, " "),
		AddressHex:      addrHex,
		ShortAddressHex: shortHex,
		ANSRegistered:   ansOK,
		Network:         s.network.String(),
		Privacy:         int(s.privacy),
	})
}

type walletAddress struct {
	AddressHex string `json:"address_hex"`
	// ShortAddressHex is the 68-byte wallet-friendly short form (abe0…) that
	// wallet apps auto-resolve to the full address. Empty when derivation
	// fails.
	ShortAddressHex string `json:"short_address_hex,omitempty"`
	Network         string `json:"network"`
	Privacy         int    `json:"privacy_level"`
	Source          string `json:"wallet_source"` // mnemonic | keyfile
	// Stable: true means AddressHex is deterministic (same on every request
	// and across restarts) — safe to share as THE funding address.
	Stable bool   `json:"stable"`
	Note   string `json:"note"`
}

// handleWalletAddress returns the relay wallet's receiving address. Mnemonic
// wallets get the deterministic sequence-0 address (stable); key-file wallets
// fall back to a fresh address per request (all detected by ReceiveCoin).
func (s *Server) handleWalletAddress(w http.ResponseWriter, r *http.Request) {
	addr := s.fixedAddr
	stable := true
	note := "fixed funding address (deterministic, same across restarts) — share this one"
	if addr == nil {
		fresh, err := s.wallet.GenerateAbelAddress()
		if err != nil {
			s.fail(w, "derive relay wallet address", err)
			return
		}
		addr, stable = fresh, false
		note = "fund the relay by sending ABEL to this address (fresh per request; all belong to the relay wallet)"
	}

	shortHex := ""
	if short, err := abel.ShortAddress(addr); err != nil {
		s.log.Warn("admin: short address derivation failed", "err", err)
	} else {
		shortHex = hex.EncodeToString(short)
	}

	writeJSON(w, walletAddress{
		AddressHex:      hex.EncodeToString(addr),
		ShortAddressHex: shortHex,
		Network:         s.network.String(),
		Privacy:         int(s.privacy),
		Source:          s.walletSource,
		Stable:          stable,
		Note:            note,
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
