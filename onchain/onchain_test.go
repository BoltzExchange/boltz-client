//go:build !unit

package onchain_test

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEstimateLowballFee(t *testing.T) {
	boltzApi := &boltz.Api{URL: "https://api.boltz.exchange"}
	chain := &onchain.Onchain{
		Liquid: &onchain.Currency{
			Tx: onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyLiquid),
		},
		Network: boltz.MainNet,
	}

	fee, err := chain.EstimateFee(boltz.CurrencyLiquid, 2)
	require.NoError(t, err)
	require.Equal(t, fee, wallet.MinFeeRate)
}
