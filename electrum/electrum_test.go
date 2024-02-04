//go:build !unit

package electrum

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/test"
	"github.com/stretchr/testify/require"
)

var url = "localhost:19001"

func client(t *testing.T) *Client {
	client, err := NewClient(url, false)
	require.NoError(t, err)
	return client
}

func TestBlockStream(t *testing.T) {
	client := client(t)
	blocks := make(chan *onchain.BlockEpoch)
	stop := make(chan bool)
	go func() {
		err := client.RegisterBlockListener(blocks, stop)
		require.NoError(t, err)
		close(blocks)
	}()
	block := <-blocks
	require.NotZero(t, block.Height)
	test.MineBlock()
	block = <-blocks
	require.NotZero(t, block.Height)
	stop <- true
	_, ok := <-blocks
	require.False(t, ok)

	height, err := client.GetBlockHeight()
	require.NoError(t, err)
	require.Equal(t, block.Height, height)
}

func TestEstimateFee(t *testing.T) {
	client := client(t)
	fee, err := client.EstimateFee(1)
	require.NoError(t, err)
	require.NotZero(t, fee)
}
