package mempool

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
