package anchor

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Golden vectors — the portal (portal/lib/memo.test.ts) must use the same set.
var goldenVectors = []struct {
	name        string
	batchNumber uint64
	stateRoot   string // 64 hex chars, lowercase
	memo        string
}{
	{
		name:        "spec example shape",
		batchNumber: 1024,
		stateRoot:   "3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e",
		memo:        "QDAY2ANCHOR|v1|1024|0x3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e",
	},
	{
		name:        "zero batch zero root",
		batchNumber: 0,
		stateRoot:   "0000000000000000000000000000000000000000000000000000000000000000",
		memo:        "QDAY2ANCHOR|v1|0|0x0000000000000000000000000000000000000000000000000000000000000000",
	},
	{
		name:        "max uint64 batch",
		batchNumber: 18446744073709551615,
		stateRoot:   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		memo:        "QDAY2ANCHOR|v1|18446744073709551615|0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	},
}

func mustRoot(t *testing.T, hexRoot string) [32]byte {
	t.Helper()
	b, err := hex.DecodeString(hexRoot)
	if err != nil || len(b) != 32 {
		t.Fatalf("bad test vector root %q: %v", hexRoot, err)
	}
	var root [32]byte
	copy(root[:], b)
	return root
}

func TestEncodeMemo_Golden(t *testing.T) {
	for _, v := range goldenVectors {
		t.Run(v.name, func(t *testing.T) {
			got := EncodeMemo(v.batchNumber, mustRoot(t, v.stateRoot))
			if !bytes.Equal(got, []byte(v.memo)) {
				t.Errorf("EncodeMemo() = %q, want %q", got, v.memo)
			}
		})
	}
}

func TestDecodeMemo_Golden(t *testing.T) {
	for _, v := range goldenVectors {
		t.Run(v.name, func(t *testing.T) {
			batch, root, err := DecodeMemo([]byte(v.memo))
			if err != nil {
				t.Fatalf("DecodeMemo() error = %v", err)
			}
			if batch != v.batchNumber {
				t.Errorf("batch = %d, want %d", batch, v.batchNumber)
			}
			if root != mustRoot(t, v.stateRoot) {
				t.Errorf("root = %x, want %s", root, v.stateRoot)
			}
		})
	}
}

func TestDecodeMemo_Roundtrip(t *testing.T) {
	root := mustRoot(t, "3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e")
	memo := EncodeMemo(42, root)
	batch, gotRoot, err := DecodeMemo(memo)
	if err != nil || batch != 42 || gotRoot != root {
		t.Fatalf("roundtrip failed: batch=%d root=%x err=%v", batch, gotRoot, err)
	}
}

func TestDecodeMemo_Rejects(t *testing.T) {
	validRoot := "0x3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e"
	cases := []struct {
		name string
		memo string
	}{
		{"wrong prefix", "OTHERANCHOR|v1|1|" + validRoot},
		{"wrong version", "QDAY2ANCHOR|v2|1|" + validRoot},
		{"three segments", "QDAY2ANCHOR|v1|1"},
		{"five segments", "QDAY2ANCHOR|v1|1|" + validRoot + "|extra"},
		{"missing 0x", "QDAY2ANCHOR|v1|1|" + strings.TrimPrefix(validRoot, "0x")},
		{"short root", "QDAY2ANCHOR|v1|1|0x3fa8"},
		{"uppercase root", "QDAY2ANCHOR|v1|1|0x" + strings.ToUpper(validRoot[2:])},
		{"non-hex root", "QDAY2ANCHOR|v1|1|0x" + strings.Repeat("zz", 32)},
		{"non-numeric batch", "QDAY2ANCHOR|v1|abc|" + validRoot},
		{"negative batch", "QDAY2ANCHOR|v1|-1|" + validRoot},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := DecodeMemo([]byte(c.memo)); err == nil {
				t.Errorf("DecodeMemo(%q) expected error, got nil", c.memo)
			}
		})
	}
}
