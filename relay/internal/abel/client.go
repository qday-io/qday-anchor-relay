// Package abel wraps abelian-sdk-go-v2 for the relay: wallet loading, coin
// scanning, memo-tx submission, and confirmation tracking (spec §7.2–7.3).
// Every SDK call here is verified against docs/sdk-verified-api.md.
package abel

import (
	"fmt"

	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"
)

// Client is the subset of the SDK RPC client the relay uses, as an interface
// so scanner/submitter/confirmer are unit-testable without an abec node.
type Client interface {
	ChainHeight() (int64, error)
	BlockByHeight(height int64) (*abelian.Block, error)
	Block(blockID string) (*abelian.Block, error)
	RawTx(txID string) (*abelian.Tx, error)
	SendSignedTx(rawTxHex string) (txID string, err error)
	RingBlockGroup(height int64) ([][]byte, error)
	MempoolTxIDs() ([]string, error)
}

type sdkClient struct {
	inner *abelian.Client
}

// NewClient builds the real SDK-backed client. TLS cert path is optional
// (abec defaults to TLS; plain HTTP endpoints skip it).
func NewClient(endpoint, user, pass, tlsCertPath string) (Client, error) {
	opts := []abelian.ClientOption{}
	if user != "" || pass != "" {
		opts = append(opts, abelian.WithAuth(user, pass))
	}
	if tlsCertPath != "" {
		opts = append(opts, abelian.WithTLS(tlsCertPath))
	}
	inner, err := abelian.NewClient(abelian.NewClientConfig(endpoint, opts...))
	if err != nil {
		return nil, fmt.Errorf("abel: new client: %w", err)
	}
	return &sdkClient{inner: inner}, nil
}

func (c *sdkClient) ChainHeight() (int64, error) {
	info, err := c.inner.GetChainInfo()
	if err != nil {
		return 0, err
	}
	return info.NumBlocks, nil
}

func (c *sdkClient) BlockByHeight(height int64) (*abelian.Block, error) {
	return c.inner.GetBlockByHeight(height)
}

func (c *sdkClient) Block(blockID string) (*abelian.Block, error) {
	return c.inner.GetBlock(blockID)
}

func (c *sdkClient) RawTx(txID string) (*abelian.Tx, error) {
	return c.inner.GetRawTx(txID)
}

func (c *sdkClient) SendSignedTx(rawTxHex string) (string, error) {
	return c.inner.SendRawTx(rawTxHex)
}

func (c *sdkClient) RingBlockGroup(height int64) ([][]byte, error) {
	return abelian.GetRingBlockGroupByHeight(c.inner, height)
}

func (c *sdkClient) MempoolTxIDs() ([]string, error) {
	return c.inner.GetRawMempool()
}
