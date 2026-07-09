package da

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Commit source (confirmed by Vike, 2026-07-10): the QDAY DA chain's
// PolygonRollupManager emits VerifyBatchesTrustedAggregator when the trusted
// aggregator verifies batches. The relay only listens — the event is fired by
// other system components. The topic hash is computed from the parsed ABI,
// never hand-written.
const commitABI = `[{
	"type": "event",
	"name": "VerifyBatchesTrustedAggregator",
	"inputs": [
		{"name": "rollupID",   "type": "uint32",  "indexed": true},
		{"name": "numBatch",   "type": "uint64",  "indexed": false},
		{"name": "stateRoot",  "type": "bytes32", "indexed": false},
		{"name": "exitRoot",   "type": "bytes32", "indexed": false},
		{"name": "aggregator", "type": "address", "indexed": true}
	],
	"anonymous": false
}]`

const commitEventName = "VerifyBatchesTrustedAggregator"

// EthLogClient is the subset of ethclient.Client the source needs (mockable).
type EthLogClient interface {
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	BlockNumber(ctx context.Context) (uint64, error)
}

var _ EthLogClient = (*ethclient.Client)(nil)

// EventLogCommitSource reads batch-verification events from the rollup
// manager contract via eth_getLogs, filtered by the event's topic[0].
type EventLogCommitSource struct {
	client   EthLogClient
	contract common.Address
	abi      abi.ABI
	topic    common.Hash
	// rollupID filters topic[1] when non-zero. The DB's batch_number UNIQUE
	// constraint assumes one rollup; with several rollups on one manager the
	// filter is mandatory.【TO-CONFIRM】which rollupID is QDAY2's.
	rollupID uint32
}

func NewEventLogCommitSource(client EthLogClient, contract common.Address, rollupID uint32) (*EventLogCommitSource, error) {
	parsed, err := abi.JSON(strings.NewReader(commitABI))
	if err != nil {
		return nil, fmt.Errorf("da: parse commit ABI: %w", err)
	}
	ev, ok := parsed.Events[commitEventName]
	if !ok {
		return nil, fmt.Errorf("da: event %s missing from ABI", commitEventName)
	}
	return &EventLogCommitSource{
		client:   client,
		contract: contract,
		abi:      parsed,
		topic:    ev.ID,
		rollupID: rollupID,
	}, nil
}

func (s *EventLogCommitSource) LatestBlock(ctx context.Context) (uint64, error) {
	return s.client.BlockNumber(ctx)
}

func (s *EventLogCommitSource) FetchCommits(ctx context.Context, from, to uint64) ([]Commit, error) {
	topics := [][]common.Hash{{s.topic}}
	if s.rollupID != 0 {
		topics = append(topics, []common.Hash{common.BigToHash(new(big.Int).SetUint64(uint64(s.rollupID)))})
	}
	logs, err := s.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{s.contract},
		Topics:    topics,
	})
	if err != nil {
		return nil, fmt.Errorf("da: eth_getLogs [%d,%d]: %w", from, to, err)
	}
	commits := make([]Commit, 0, len(logs))
	for _, lg := range logs {
		c, err := s.parseLog(lg)
		if err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}
	return commits, nil
}

func (s *EventLogCommitSource) parseLog(lg types.Log) (Commit, error) {
	// topics: [0]=event sig, [1]=indexed rollupID, [2]=indexed aggregator.
	if len(lg.Topics) < 3 {
		return Commit{}, fmt.Errorf("da: commit log %s has %d topics, want 3", lg.TxHash, len(lg.Topics))
	}
	// Non-indexed data: (numBatch uint64, stateRoot bytes32, exitRoot bytes32).
	var decoded struct {
		NumBatch  uint64
		StateRoot [32]byte
		ExitRoot  [32]byte
	}
	if err := s.abi.UnpackIntoInterface(&decoded, commitEventName, lg.Data); err != nil {
		return Commit{}, fmt.Errorf("da: unpack commit log %s: %w", lg.TxHash, err)
	}
	return Commit{
		BatchNumber: decoded.NumBatch,
		StateRoot:   decoded.StateRoot,
		DABlock:     lg.BlockNumber,
		DATxHash:    lg.TxHash.Hex(),
	}, nil
}
