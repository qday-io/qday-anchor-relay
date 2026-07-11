package abelian

import (
	"fmt"
)

type CoinID struct {
	TxID  string
	Index uint8
}

// Define methods for CoinID.
func NewCoinID(txHash string, index uint8) *CoinID {
	return &CoinID{
		TxID:  txHash,
		Index: index,
	}
}

func (id CoinID) String() string {
	return fmt.Sprintf("%s:%d", id.TxID, id.Index)
}

type Coin struct {
	TxVersion    uint32
	TxID         string
	Index        uint8
	BlockHash    string
	BlockHeight  int64
	Value        int64
	SerialNumber string
	TxVoutData   []byte
	RingID       string
	RingIndex    uint8
}

func (coin *Coin) ID() *CoinID {
	return &CoinID{
		TxID:  coin.TxID,
		Index: coin.Index,
	}
}
func (coin *Coin) SetRingInfo(ringID string, ringIndex uint8) {
	coin.RingID = ringID
	coin.RingIndex = ringIndex
}
func NewCoin(
	txVersion uint32,
	txID string,
	index uint8,
	blockHash string,
	blockHeight int64,
	value int64,
	serialNumber string,
	txVoutData []byte,
) *Coin {
	return &Coin{
		TxVersion:    txVersion,
		TxID:         txID,
		Index:        index,
		BlockHash:    blockHash,
		BlockHeight:  blockHeight,
		Value:        value,
		SerialNumber: serialNumber,
		TxVoutData:   txVoutData,
	}
}
