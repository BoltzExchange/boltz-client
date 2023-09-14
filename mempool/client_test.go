package mempool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mempoolEndpoint = "https://mempool.space/api"

func TestInitClient(t *testing.T) {
	assert.Equal(t, mempoolEndpoint, initClient(mempoolEndpoint).api)
	assert.Equal(t, mempoolEndpoint, initClient(mempoolEndpoint+"/").api)
}

func TestGetFeeRecommendation(t *testing.T) {
	mc := initClient(mempoolEndpoint)

	fees, err := mc.getFeeRecommendation()
	require.Nil(t, err)

	assert.NotEqual(t, 0, fees.FastestFee)
	assert.NotEqual(t, 0, fees.HalfHourFee)
	assert.NotEqual(t, 0, fees.HourFee)
	assert.NotEqual(t, 0, fees.EconomyFee)
	assert.NotEqual(t, 0, fees.MinimumFee)
}

func TestBlockStream(t *testing.T) {

	mc := initClient(mempoolEndpoint)

	blocks, err := mc.startBlockStream()
	require.Nil(t, err)

	block := <-blocks

	assert.NotEqual(t, 0, block.Height)

}
