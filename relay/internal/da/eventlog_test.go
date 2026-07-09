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

// buildCommitLog packs a VerifyBatchesTrustedAggregator log the way the EVM would.
func buildCommitLog(t *testing.T, rollupID uint32, numBatch uint64, stateRoot, exitRoot [32]byte, block uint64) types.Log {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(commitABI))
	if err != nil {
		t.Fatal(err)
	}
	data, err := parsed.Events[commitEventName].Inputs.NonIndexed().Pack(numBatch, stateRoot, exitRoot)
	if err != nil {
		t.Fatal(err)
	}
	aggregator := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	return types.Log{
		Address: common.HexToAddress("0x959922bE3CAee4b8Cd9a407cc3ac1C251C2007B1"),
		Topics: []common.Hash{
			parsed.Events[commitEventName].ID,
			common.BigToHash(new(big.Int).SetUint64(uint64(rollupID))),
			common.BytesToHash(aggregator.Bytes()),
		},
		Data:        data,
		BlockNumber: block,
		TxHash:      common.HexToHash("0xabc123"),
	}
}

func TestEventLogCommitSource_FetchCommits(t *testing.T) {
	var stateRoot, exitRoot [32]byte
	stateRoot[0], stateRoot[31] = 0x3f, 0x1e
	exitRoot[0] = 0xee

	client := &fakeEthClient{logs: []types.Log{buildCommitLog(t, 1, 1024, stateRoot, exitRoot, 555)}}
	src, err := NewEventLogCommitSource(client, common.HexToAddress("0x959922bE3CAee4b8Cd9a407cc3ac1C251C2007B1"), 0)
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
	if c.BatchNumber != 1024 || c.StateRoot != stateRoot || c.DABlock != 555 || c.DATxHash == "" {
		t.Errorf("commit = %+v", c)
	}
	// exitRoot must NOT leak into the state root.
	if c.StateRoot == exitRoot {
		t.Error("stateRoot confused with exitRoot")
	}

	// Query must filter by contract address, topic0 only (rollupID 0 = all),
	// and the exact range.
	if client.query.FromBlock.Uint64() != 500 || client.query.ToBlock.Uint64() != 600 {
		t.Errorf("range = [%v,%v]", client.query.FromBlock, client.query.ToBlock)
	}
	if len(client.query.Addresses) != 1 || len(client.query.Topics) != 1 {
		t.Errorf("query filters = %+v", client.query)
	}
}

func TestEventLogCommitSource_RollupIDFilter(t *testing.T) {
	client := &fakeEthClient{}
	src, err := NewEventLogCommitSource(client, common.Address{}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.FetchCommits(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	// topic[1] filter present and equals rollupID 7.
	if len(client.query.Topics) != 2 || len(client.query.Topics[1]) != 1 {
		t.Fatalf("topics = %+v", client.query.Topics)
	}
	if client.query.Topics[1][0] != common.BigToHash(big.NewInt(7)) {
		t.Errorf("rollupID topic = %v", client.query.Topics[1][0])
	}
}

func TestEventLogCommitSource_TopicMatchesKnownHash(t *testing.T) {
	// cast keccak "VerifyBatchesTrustedAggregator(uint32,uint64,bytes32,bytes32,address)"
	const want = "0xd1ec3a1216f08b6eff72e169ceb548b782db18a6614852618d86bb19f3f9b0d3"
	src, err := NewEventLogCommitSource(&fakeEthClient{}, common.Address{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if src.topic != common.HexToHash(want) {
		t.Errorf("topic0 = %s, want %s", src.topic, want)
	}
}

func TestEventLogCommitSource_RejectsMalformedLog(t *testing.T) {
	client := &fakeEthClient{logs: []types.Log{{ /* no topics */ }}}
	src, err := NewEventLogCommitSource(client, common.Address{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.FetchCommits(context.Background(), 1, 2); err == nil {
		t.Fatal("expected error for malformed log")
	}
}
