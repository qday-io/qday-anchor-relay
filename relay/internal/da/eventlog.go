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

// 【TO-CONFIRM C1】 Assumed commit event signature (spec §7.1). Replace this ABI
// once Vike provides the real contract ABI — only this file changes. The topic
// hash is computed from the parsed ABI, never hand-written.
const assumedCommitABI = `[{
	"type": "event",
	"name": "BatchCommitted",
	"inputs": [
		{"name": "batchNumber", "type": "uint64", "indexed": true},
		{"name": "stateRoot",   "type": "bytes32", "indexed": false}
	],
	"anonymous": false
}]`

const commitEventName = "BatchCommitted"

// EthLogClient is the subset of ethclient.Client the source needs (mockable).
type EthLogClient interface {
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	BlockNumber(ctx context.Context) (uint64, error)
}

var _ EthLogClient = (*ethclient.Client)(nil)

// EventLogCommitSource reads commit events from the DA commit contract via
// eth_getLogs.
type EventLogCommitSource struct {
	client   EthLogClient
	contract common.Address
	abi      abi.ABI
	topic    common.Hash
}

func NewEventLogCommitSource(client EthLogClient, contract common.Address) (*EventLogCommitSource, error) {
	parsed, err := abi.JSON(strings.NewReader(assumedCommitABI))
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
	}, nil
}

func (s *EventLogCommitSource) LatestBlock(ctx context.Context) (uint64, error) {
	return s.client.BlockNumber(ctx)
}

func (s *EventLogCommitSource) FetchCommits(ctx context.Context, from, to uint64) ([]Commit, error) {
	logs, err := s.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{s.contract},
		Topics:    [][]common.Hash{{s.topic}},
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
	if len(lg.Topics) < 2 {
		return Commit{}, fmt.Errorf("da: commit log %s has %d topics, want 2", lg.TxHash, len(lg.Topics))
	}
	// topic[1] = indexed uint64 batchNumber (left-padded to 32 bytes).
	batch := new(big.Int).SetBytes(lg.Topics[1].Bytes())
	if !batch.IsUint64() {
		return Commit{}, fmt.Errorf("da: batch number overflows uint64 in log %s", lg.TxHash)
	}
	var decoded struct{ StateRoot [32]byte }
	if err := s.abi.UnpackIntoInterface(&decoded, commitEventName, lg.Data); err != nil {
		return Commit{}, fmt.Errorf("da: unpack commit log %s: %w", lg.TxHash, err)
	}
	return Commit{
		BatchNumber: batch.Uint64(),
		StateRoot:   decoded.StateRoot,
		DABlock:     lg.BlockNumber,
		DATxHash:    lg.TxHash.Hex(),
	}, nil
}
