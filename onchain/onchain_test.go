//go:build !unit

package onchain_test

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	onchainmock "github.com/BoltzExchange/boltz-client/mocks/github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEstimateLowballFee(t *testing.T) {
	boltzApi := &boltz.Api{URL: "https://api.boltz.exchange"}
	blocks := onchainmock.NewMockBlockProvider(t)
	chain := &onchain.Onchain{
		Liquid: &onchain.Currency{
			Tx:     onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyLiquid),
			Blocks: blocks,
		},
		Network: boltz.MainNet,
	}
	blocks.EXPECT().EstimateFee().Return(0.1, nil)

	fee, err := chain.EstimateFee(boltz.CurrencyLiquid, true)
	require.NoError(t, err)
	require.Equal(t, fee, wallet.MinFeeRate)

	fee, err = chain.EstimateFee(boltz.CurrencyLiquid, false)
	require.NoError(t, err)
	require.Greater(t, fee, wallet.MinFeeRate)
}
