// Package anchor implements the QDAY2ANCHOR memo protocol (spec §5).
//
// Format (fixed, protocol-level — do not change without bumping the version segment):
//
//	QDAY2ANCHOR|v1|<batchNumber decimal>|0x<stateRoot 64 hex lowercase>
package anchor

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	MemoPrefix  = "QDAY2ANCHOR"
	MemoVersion = "v1"
)

// EncodeMemo renders the anchor payload for an Abelian transaction memo.
func EncodeMemo(batchNumber uint64, stateRoot [32]byte) []byte {
	return []byte(fmt.Sprintf("%s|%s|%d|0x%s",
		MemoPrefix, MemoVersion, batchNumber, hex.EncodeToString(stateRoot[:])))
}

// DecodeMemo parses a memo produced by EncodeMemo. It accepts only the exact
// v1 format: 4 pipe-separated segments, decimal batch number, and a lowercase
// 0x-prefixed 64-hex-digit state root.
func DecodeMemo(memo []byte) (batchNumber uint64, stateRoot [32]byte, err error) {
	parts := strings.Split(string(memo), "|")
	if len(parts) != 4 {
		return 0, stateRoot, fmt.Errorf("anchor: memo must have 4 segments, got %d", len(parts))
	}
	if parts[0] != MemoPrefix {
		return 0, stateRoot, fmt.Errorf("anchor: bad memo prefix %q", parts[0])
	}
	if parts[1] != MemoVersion {
		return 0, stateRoot, fmt.Errorf("anchor: unsupported memo version %q", parts[1])
	}
	batchNumber, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return 0, stateRoot, fmt.Errorf("anchor: bad batch number %q: %w", parts[2], err)
	}
	rootHex, ok := strings.CutPrefix(parts[3], "0x")
	if !ok || len(rootHex) != 64 || rootHex != strings.ToLower(rootHex) {
		return 0, stateRoot, fmt.Errorf("anchor: state root must be 0x + 64 lowercase hex chars")
	}
	rootBytes, err := hex.DecodeString(rootHex)
	if err != nil {
		return 0, stateRoot, fmt.Errorf("anchor: state root is not hex: %w", err)
	}
	copy(stateRoot[:], rootBytes)
	return batchNumber, stateRoot, nil
}
