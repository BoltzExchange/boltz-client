package mempool

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/boltz"

	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mempoolEndpoint = "https://mempool.space/api"
const liquidEndpoint = "https://liquid.network/api"

func TestInitClient(t *testing.T) {
	assert.Equal(t, mempoolEndpoint, InitClient(mempoolEndpoint).api)
	assert.Equal(t, mempoolEndpoint, InitClient(mempoolEndpoint+"/").api)
}

func ClientTest(t *testing.T, test func(*testing.T, *Client)) {
	// TODO: check if regtest is available
	t.Run("BTC", func(t *testing.T) {
		test(t, InitClient(mempoolEndpoint))
	})
	t.Run("Liquid", func(t *testing.T) {
		test(t, InitClient(liquidEndpoint))
	})
}

func TestGetFeeRecommendation(t *testing.T) {
	ClientTest(t, func(t *testing.T, mc *Client) {
		fees, err := mc.getFeeRecommendation()
		require.NoError(t, err)

		assert.NotEqual(t, 0, fees.FastestFee)
		assert.NotEqual(t, 0, fees.HalfHourFee)
		assert.NotEqual(t, 0, fees.HourFee)
		assert.NotEqual(t, 0, fees.EconomyFee)
		assert.NotEqual(t, 0, fees.MinimumFee)
	})
}

func TestGetBlockHeight(t *testing.T) {
	ClientTest(t, func(t *testing.T, mc *Client) {
		height, err := mc.GetBlockHeight()
		require.NoError(t, err)
		require.NotZero(t, height)
	})
}

func TestGetTx(t *testing.T) {
	mc := InitClient(mempoolEndpoint)
	hex, err := mc.GetTxHex("c95157dc89ea9b531c4bd40e51ced2e1f4910b770c5e0e090d40b93e47ff95fd")
	require.NoError(t, err)
	expected := "0100000000010191e167827563d3556fe53e7f11b7b6f1934185e7cd3822f3a553e48097de090c0100000017160014447ffa2ecf10a546de692082f8a18040e03dd3e0ffffffff0299c203000000000017a9141763c38535a4da845b8044a3c2996d619bb1fcf387e338b9030000000017a9140ee239e8b19b69d03d57d9f2cb8c24f986191a4d8702483045022100f281b3acb4f96a4d85b2608b3347614bc920daa3ec760efe4104a2d5cbd0aff802200b846e3e048fd4959bf76a80d18ae11c9b6fa6f8db2ee72044e0fa53032e51ec0121035305939e188725cef254ccf32be136509c0949a7a1d86fa1b41faffa983524ec00000000"
	require.Equal(t, expected, hex)
}

func TestOnchainGetFee(t *testing.T) {
	onchain := onchain.Onchain{
		Btc: &onchain.Currency{
			Tx: InitClient(mempoolEndpoint),
		},
	}

	tx, err := boltz.NewTxFromHex("0100000000010191e167827563d3556fe53e7f11b7b6f1934185e7cd3822f3a553e48097de090c0100000017160014447ffa2ecf10a546de692082f8a18040e03dd3e0ffffffff0299c203000000000017a9141763c38535a4da845b8044a3c2996d619bb1fcf387e338b9030000000017a9140ee239e8b19b69d03d57d9f2cb8c24f986191a4d8702483045022100f281b3acb4f96a4d85b2608b3347614bc920daa3ec760efe4104a2d5cbd0aff802200b846e3e048fd4959bf76a80d18ae11c9b6fa6f8db2ee72044e0fa53032e51ec0121035305939e188725cef254ccf32be136509c0949a7a1d86fa1b41faffa983524ec00000000", nil)
	require.NoError(t, err)

	fee, err := onchain.GetTransactionFee(boltz.PairBtc, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, 2656, int(fee))
}
