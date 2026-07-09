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
	log          *slog.Logger
}

func NewServer(network abelian.NetworkID, privacy abelian.AccountPrivacyLevel, wallet AddressAccount, walletSource string, log *slog.Logger) *Server {
	return &Server{network: network, privacy: privacy, wallet: wallet, walletSource: walletSource, log: log}
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
	Note       string `json:"note"`
}

// handleWalletAddress derives a fresh receiving address for the RELAY's own
// wallet (root-seed accounts yield a different address per call by design;
// all of them are detected by ReceiveCoin).
func (s *Server) handleWalletAddress(w http.ResponseWriter, r *http.Request) {
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
