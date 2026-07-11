package abel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestANSClient_Register(t *testing.T) {
	var gotBody map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/ans/v1/register" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "register_result",
			"payload": map[string]any{
				"already_registered": false,
				"register_at":        "2026-07-11T00:00:00Z",
				"short_address":      "abe0cafe",
			},
		})
	}))
	defer ts.Close()

	c := NewANSClient(ts.URL + "/ans/v1")
	short, err := c.Register(context.Background(), "02010000ff")
	if err != nil {
		t.Fatal(err)
	}
	if short != "abe0cafe" {
		t.Errorf("short = %q", short)
	}
	if gotBody["abel_address"] != "02010000ff" {
		t.Errorf("body = %v", gotBody)
	}
}

func TestANSClient_RegisterErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "payload": "Wrong parameters."})
	}))
	defer ts.Close()

	c := NewANSClient(ts.URL)
	if _, err := c.Register(context.Background(), "xx"); err == nil {
		t.Fatal("expected error for ANS error response")
	}
}
