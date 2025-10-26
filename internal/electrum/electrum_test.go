//go:build !unit

package electrum_test

import (
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/electrum"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/require"
)

func client(t *testing.T) *electrum.Client {
	client, err := electrum.NewClient(*onchain.RegtestElectrumConfig.Btc)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Disconnect()
	})
	return client
}

func TestBlockStream(t *testing.T) {
	client := client(t)

	var height uint32
	require.Eventually(t, func() bool {
		var err error
		height, err = client.GetBlockHeight()
		require.NoError(t, err)
		return height != 0
	}, 10*time.Second, 100*time.Millisecond)

	test.MineBlock()

	require.Eventually(t, func() bool {
		newHeight, err := client.GetBlockHeight()
		require.NoError(t, err)
		return newHeight == height+1
	}, 10*time.Second, 100*time.Millisecond)
}

func TestEstimateFee(t *testing.T) {
	client := client(t)
	fee, err := client.EstimateFee()
	require.NoError(t, err)
	require.NotZero(t, fee)
}
