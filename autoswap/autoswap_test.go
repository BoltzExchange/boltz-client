package autoswap

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/onchain"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func getTestDb(t *testing.T) *database.Database {
	db := &database.Database{
		// it's important to use a shared cache here since the SQL driver can open up multiple connections
		// which would otherwise cause the memory db to disappear
		Path: "file:" + t.TempDir() + "?mode=memory&cache=shared",
	}
	require.NoError(t, db.Connect())
	return db
}

var PairInfo = &boltzrpc.PairInfo{
	Limits: &boltzrpc.Limits{
		Maximal: 100,
		Minimal: 1000,
	},
	Fees: &boltzrpc.SwapFees{
		Percentage: 10,
		MinerFees:  10,
	},
}

func getOnchain() *onchain.Onchain {
	chain := &onchain.Onchain{
		Network: boltz.Regtest,
	}
	chain.Init()
	return chain
}

func getSwapper(t *testing.T) (*AutoSwap, *MockRpcProvider) {
	mockProvider := NewMockRpcProvider(t)
	swapper := &AutoSwap{}
	swapper.Init(getTestDb(t), getOnchain(), t.TempDir()+"/autoswap.toml", mockProvider)
	return swapper, mockProvider
}

const oldConfig = `
acceptZeroConf = true
budget = "100000"
budgetInterval = "604800"
channelPollInterval = "30"
currency = "LBTC"
enabled = false
failureBackoff = "3600"
inboundBalance = "0"
inboundBalancePercent = 25.0
maxFeePercent = 1.0
maxSwapAmount = "0"
outboundBalance = "0"
outboundBalancePercent = 25.0
perChannel = false
staticAddress = ""
swapType = ""
wallet = ""
`

func TestOldConfig(t *testing.T) {
	swapper, _ := getSwapper(t)
	err := os.WriteFile(swapper.configPath, []byte(oldConfig), 0644)
	require.NoError(t, err)

	require.NoError(t, swapper.LoadConfig())
	require.NotEmpty(t, swapper.cfg.Lightning)
	require.True(t, swapper.cfg.Lightning[0].AcceptZeroConf)
}
