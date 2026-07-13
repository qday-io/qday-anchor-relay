package abel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ANSClient registers addresses with the Abelian Name Service so wallets can
// resolve the short form when sending (wallets look short addresses up in ANS
// — an unregistered short address cannot receive wallet transfers).
type ANSClient struct {
	baseURL string // e.g. https://testnet-ans.pqabelian.io/ans/v1
	http    *http.Client
}

func NewANSClient(baseURL string) *ANSClient {
	return &ANSClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type ansRegisterResponse struct {
	Type    string `json:"type"`
	Payload struct {
		AlreadyRegistered bool   `json:"already_registered"`
		RegisterAt        string `json:"register_at"`
		ShortAddress      string `json:"short_address"`
	} `json:"payload"`
}

// Register submits the full address (hex) and returns the ANS-issued short
// address hex. Idempotent server-side (already_registered=true on repeats).
func (c *ANSClient) Register(ctx context.Context, fullAddressHex string) (shortAddressHex string, err error) {
	body, err := json.Marshal(map[string]string{"abel_address": fullAddressHex})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/register", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("abel: ans register: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("abel: ans register: HTTP %d", resp.StatusCode)
	}

	var parsed ansRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("abel: ans register: decode response: %w", err)
	}
	if parsed.Type != "register_result" || parsed.Payload.ShortAddress == "" {
		return "", fmt.Errorf("abel: ans register failed (type=%q)", parsed.Type)
	}
	return parsed.Payload.ShortAddress, nil
}
