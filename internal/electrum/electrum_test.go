//go:build !unit

package electrum

import (
	"context"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/require"
)

func client(t *testing.T) *Client {
	client, err := NewClient(onchain.RegtestElectrumConfig.Btc)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Disconnect()
	})
	return client
}

func TestBlockStream(t *testing.T) {
	client := client(t)
	blocks := make(chan *onchain.BlockEpoch)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := client.RegisterBlockListener(ctx, blocks)
		require.NoError(t, err)
		close(blocks)
	}()
	block := <-blocks
	require.NotZero(t, block.Height)
	test.MineBlock()
	block = <-blocks
	require.NotZero(t, block.Height)
	cancel()
	_, ok := <-blocks
	require.False(t, ok)

	height, err := client.GetBlockHeight()
	require.NoError(t, err)
	require.Equal(t, block.Height, height)
}

func TestEstimateFee(t *testing.T) {
	client := client(t)
	fee, err := client.EstimateFee()
	require.NoError(t, err)
	require.NotZero(t, fee)
}
