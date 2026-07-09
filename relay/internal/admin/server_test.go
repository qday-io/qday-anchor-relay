package admin

import (
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }

var testLog = slog.New(slog.NewTextHandler(nullWriter{}, nil))

type fakeWallet struct{ addr []byte }

func (f fakeWallet) GenerateAbelAddress() ([]byte, error) { return f.addr, nil }

func TestGenerateAccount(t *testing.T) {
	srv := NewServer(abelian.TestNet, abelian.AccountPrivacyLevelPseudonym, fakeWallet{}, testLog)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/admin/generate-account", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var acc generatedAccount
	if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
		t.Fatal(err)
	}
	if len(strings.Fields(acc.Mnemonic)) < 12 {
		t.Errorf("mnemonic too short: %q", acc.Mnemonic)
	}
	if acc.Network != "testnet" || acc.AddressHex == "" {
		t.Errorf("acc = %+v", acc)
	}
	// The returned address must decode and parse as a valid AbelAddress.
	addrBytes, err := hex.DecodeString(acc.AddressHex)
	if err != nil {
		t.Fatalf("address not hex: %v", err)
	}
	if _, err := abelian.NewAbelAddress(addrBytes); err != nil {
		t.Errorf("generated address invalid: %v", err)
	}
}

func TestWalletAddress(t *testing.T) {
	srv := NewServer(abelian.TestNet, abelian.AccountPrivacyLevelPseudonym, fakeWallet{addr: []byte{0xab, 0xcd}}, testLog)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/admin/wallet-address")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var wa walletAddress
	if err := json.NewDecoder(resp.Body).Decode(&wa); err != nil {
		t.Fatal(err)
	}
	if wa.AddressHex != "abcd" {
		t.Errorf("address = %q", wa.AddressHex)
	}
}

func TestMethodRouting(t *testing.T) {
	srv := NewServer(abelian.TestNet, abelian.AccountPrivacyLevelPseudonym, fakeWallet{}, testLog)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// GET on generate-account must not work (POST-only).
	resp, err := ts.Client().Get(ts.URL + "/admin/generate-account")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("GET /admin/generate-account should not be allowed")
	}
}
