package da

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type fakeEthClient struct {
	logs  []types.Log
	query ethereum.FilterQuery
	head  uint64
}

func (f *fakeEthClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	f.query = q
	return f.logs, nil
}

func (f *fakeEthClient) BlockNumber(ctx context.Context) (uint64, error) { return f.head, nil }

// buildCommitLog packs a BatchCommitted log the way the EVM would.
func buildCommitLog(t *testing.T, batch uint64, root [32]byte, block uint64) types.Log {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(assumedCommitABI))
	if err != nil {
		t.Fatal(err)
	}
	data, err := parsed.Events[commitEventName].Inputs.NonIndexed().Pack(root)
	if err != nil {
		t.Fatal(err)
	}
	return types.Log{
		Address:     common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Topics:      []common.Hash{parsed.Events[commitEventName].ID, common.BigToHash(new(big.Int).SetUint64(batch))},
		Data:        data,
		BlockNumber: block,
		TxHash:      common.HexToHash("0xabc123"),
	}
}

func TestEventLogCommitSource_FetchCommits(t *testing.T) {
	var root [32]byte
	root[0], root[31] = 0x3f, 0x1e

	client := &fakeEthClient{logs: []types.Log{buildCommitLog(t, 1024, root, 555)}}
	src, err := NewEventLogCommitSource(client, common.HexToAddress("0x1111111111111111111111111111111111111111"))
	if err != nil {
		t.Fatal(err)
	}

	commits, err := src.FetchCommits(context.Background(), 500, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("commits = %d, want 1", len(commits))
	}
	c := commits[0]
	if c.BatchNumber != 1024 || c.StateRoot != root || c.DABlock != 555 || c.DATxHash == "" {
		t.Errorf("commit = %+v", c)
	}

	// Query must filter by contract address, topic, and the exact range.
	if client.query.FromBlock.Uint64() != 500 || client.query.ToBlock.Uint64() != 600 {
		t.Errorf("range = [%v,%v]", client.query.FromBlock, client.query.ToBlock)
	}
	if len(client.query.Addresses) != 1 || len(client.query.Topics) != 1 {
		t.Errorf("query filters = %+v", client.query)
	}
}

func TestEventLogCommitSource_RejectsMalformedLog(t *testing.T) {
	client := &fakeEthClient{logs: []types.Log{{ /* no topics */ }}}
	src, err := NewEventLogCommitSource(client, common.Address{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.FetchCommits(context.Background(), 1, 2); err == nil {
		t.Fatal("expected error for malformed log")
	}
}
