package autoswap

import (
	"github.com/BoltzExchange/boltz-client/lightning"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"

	"github.com/stretchr/testify/require"
)

var fees = &boltzrpc.Fees{
	Percentage: 10,
	Miner: &boltzrpc.MinerFees{
		Normal:  10,
		Reverse: 10,
	},
}
var limits = &boltzrpc.Limits{
	Minimal: 100,
	Maximal: 1000,
}

func getTestDb(t *testing.T) *database.Database {
	db := &database.Database{
		Path: ":memory:",
	}
	require.NoError(t, db.Connect())
	return db
}

func getSwapper(t *testing.T, cfg *Config) *AutoSwapper {
	swapper := &AutoSwapper{
		ExecuteSwap: func(_ *boltzrpc.CreateSwapRequest) error {
			return nil
		},
		ExecuteReverseSwap: func(_ *boltzrpc.CreateReverseSwapRequest) error {
			return nil
		},
		ListChannels: func() ([]*lightning.LightningChannel, error) {
			return nil, nil
		},
		GetServiceInfo: func(pair boltz.Pair) (*boltzrpc.Fees, *boltzrpc.Limits, error) {
			return fees, limits, nil
		},
	}
	swapper.Init(getTestDb(t), nil, ".")
	// don't call `LoadConfig` here because the test configs might not be completely valid
	swapper.cfg = cfg
	return swapper
}

func fakeSwap(onchainFee uint64, serviceFee uint64, isAuto bool, age uint64) database.Swap {
	swap := database.Swap{
		Id:         "TEST",
		OnchainFee: &onchainFee,
		ServiceFee: &serviceFee,
		IsAuto:     isAuto,
	}
	if age != 0 {
		swap.CreatedAt = time.Now().Add(time.Duration(-age) * time.Second)
	}
	return swap
}

func fakeReverseSwap(onchainFee uint64, serviceFee uint64, routingFeeMsat uint64, isAuto bool, age uint64) database.ReverseSwap {
	swap := database.ReverseSwap{
		Id:             "TEST",
		OnchainFee:     &onchainFee,
		ServiceFee:     &serviceFee,
		RoutingFeeMsat: &routingFeeMsat,
		IsAuto:         isAuto,
	}
	return swap
}

func TestBudget(t *testing.T) {

	tests := []struct {
		name            string
		config          *Config
		swaps           []database.Swap
		reverseSwaps    []database.ReverseSwap
		expected        int64
		currentInterval *database.BudgetInterval
	}{
		{
			name: "Normal Swaps",
			config: &Config{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(10, 10, true, 0),
			},
			expected: 80,
		},
		{
			name: "Reverse Swaps",
			config: &Config{
				Budget:         100,
				BudgetInterval: 1000,
			},
			reverseSwaps: []database.ReverseSwap{
				fakeReverseSwap(10, 10, 10000, true, 0),
			},
			expected: 70,
		},
		{
			name: "Auto-Only",
			config: &Config{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(10, 10, false, 0),
			},
			expected: 100,
		},
		{
			name: "New",
			config: &Config{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(10, 10, true, 1500),
			},
			currentInterval: &database.BudgetInterval{
				StartDate: time.Now().Add(time.Duration(-2000) * time.Second),
				EndDate:   time.Now().Add(time.Duration(-1000) * time.Second),
			},
			expected: 100,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			swapper := getSwapper(t, tc.config)
			db := swapper.database

			if tc.currentInterval != nil {
				require.NoError(t, db.CreateBudget(*tc.currentInterval))
			}

			budget, err := swapper.GetCurrentBudget(true)
			require.NoError(t, err)
			require.Equal(t, int64(tc.config.Budget), budget.Amount)

			for _, swap := range tc.swaps {
				require.NoError(t, db.CreateSwap(swap))
			}

			for _, reverseSwap := range tc.reverseSwaps {
				require.NoError(t, db.CreateReverseSwap(reverseSwap))
			}

			budget, err = swapper.GetCurrentBudget(true)

			require.NoError(t, err)
			require.Equal(t, tc.expected, budget.Amount)
		})
	}

	t.Run("Missing", func(t *testing.T) {
		swapper := getSwapper(t, &Config{})
		budget, err := swapper.GetCurrentBudget(false)
		require.NoError(t, err)
		require.Nil(t, budget)
	})

}

func TestStrategies(t *testing.T) {

	channels := []*lightning.LightningChannel{
		{
			LocalSat:  100,
			RemoteSat: 100,
			Capacity:  200,
			Id:        1,
		},
		{
			LocalSat:  50,
			RemoteSat: 150,
			Capacity:  200,
			Id:        2,
		},
		{
			LocalSat:  500,
			RemoteSat: 100,
			Capacity:  600,
			Id:        3,
		},
	}

	tests := []struct {
		name         string
		config       *Config
		veverseSwaps []database.ReverseSwap
		outcome      []*rawRecommendation
		channels     []*lightning.LightningChannel
		err          error
	}{
		{
			name: "PerChannel/Low",
			config: &Config{
				PerChannel:        true,
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
			},
			outcome: []*rawRecommendation{
				{
					Type:    boltz.NormalSwap,
					Amount:  50,
					Channel: channels[1],
				},
				{
					Type:    boltz.ReverseSwap,
					Amount:  200,
					Channel: channels[2],
				},
			},
		},
		{
			name: "PerChannel/High",
			config: &Config{
				PerChannel:        true,
				MaxBalancePercent: 75,
				MinBalancePercent: 25,
			},
			outcome: []*rawRecommendation{
				{
					Type:    boltz.ReverseSwap,
					Amount:  200,
					Channel: channels[2],
				},
			},
		},
		{
			name: "PerChannel/OnlyNormalSwap",
			config: &Config{
				PerChannel:        true,
				MaxBalancePercent: 75,
				MinBalancePercent: 25,
				Type:              boltz.NormalSwap,
			},
			outcome: nil,
		},
		{
			name: "TotalBalance/Reverse",
			config: &Config{
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.ReverseSwap,
					Amount: 150,
				},
			},
		},
		{
			name: "TotalBalance/Normal",
			config: &Config{
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
			},
			channels: []*lightning.LightningChannel{
				{
					LocalSat:  100,
					RemoteSat: 100,
					Capacity:  200,
					Id:        1,
				},
				{
					LocalSat:  150,
					RemoteSat: 50,
					Capacity:  200,
					Id:        2,
				},
				{
					LocalSat:  100,
					RemoteSat: 500,
					Capacity:  600,
					Id:        3,
				},
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.NormalSwap,
					Amount: 150,
				},
			},
		},
		{
			name: "TotalBalance/Max",
			config: &Config{
				Type:       boltz.ReverseSwap,
				MaxBalance: 600,
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.ReverseSwap,
					Amount: 350,
				},
			},
		},
		{
			name: "TotalBalance/Min",
			config: &Config{
				Type:       boltz.NormalSwap,
				MinBalance: 400,
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.NormalSwap,
					Amount: 400,
				},
			},
			channels: []*lightning.LightningChannel{
				{
					LocalSat:  300,
					RemoteSat: 700,
					Capacity:  1000,
				},
			},
		},
		{
			name: "TotalBalance/Both/Above",
			config: &Config{
				MinBalance: 400,
				MaxBalance: 600,
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.NormalSwap,
					Amount: 200,
				},
			},
			channels: []*lightning.LightningChannel{
				{
					LocalSat:  300,
					RemoteSat: 700,
					Capacity:  1000,
				},
			},
		},
		{
			name: "TotalBalance/Both/Below",
			config: &Config{
				MinBalance: 400,
				MaxBalance: 600,
			},
			outcome: []*rawRecommendation{
				{
					Type:   boltz.ReverseSwap,
					Amount: 200,
				},
			},
			channels: []*lightning.LightningChannel{
				{
					LocalSat:  700,
					RemoteSat: 500,
					Capacity:  1000,
				},
			},
		},
		{
			name: "TotalBalance/None",
			config: &Config{
				MinBalance: 400,
				MaxBalance: 700,
			},
			outcome: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.config.Init())
			if tc.channels == nil {
				tc.channels = channels
			}
			recommendations := tc.config.strategy(tc.channels)

			require.Equal(t, tc.outcome, recommendations)
		})

	}
}

func TestDismissedChannels(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		channels     []*lightning.LightningChannel
		swaps        []database.Swap
		reverseSwaps []database.ReverseSwap
		dismissed    DismissedChannels
	}{
		{
			name: "Pending Swaps",
			config: &Config{
				FailureBackoff: 1000,
			},
			swaps: []database.Swap{
				{
					Id:      "TEST",
					State:   boltzrpc.SwapState_PENDING,
					ChanIds: []lightning.ChanId{1},
					IsAuto:  true,
				},
			},
			reverseSwaps: []database.ReverseSwap{
				{
					Id:      "TEST",
					State:   boltzrpc.SwapState_PENDING,
					IsAuto:  true,
					ChanIds: []lightning.ChanId{2},
				},
				{
					Id:      "TEST1",
					State:   boltzrpc.SwapState_SUCCESSFUL,
					IsAuto:  true,
					ChanIds: []lightning.ChanId{3},
				},
			},
			dismissed: DismissedChannels{
				1: []string{ReasonPendingSwap},
				2: []string{ReasonPendingSwap},
			},
		},
		{
			name: "Failed Swaps",
			config: &Config{
				FailureBackoff: 1000,
			},
			swaps: []database.Swap{
				{
					Id:      "TEST",
					State:   boltzrpc.SwapState_ERROR,
					ChanIds: []lightning.ChanId{1},
					IsAuto:  true,
				},
			},
			reverseSwaps: []database.ReverseSwap{
				{
					Id:        "TEST",
					State:     boltzrpc.SwapState_ERROR,
					CreatedAt: time.Now().Add(time.Duration(-2000) * time.Second),
					IsAuto:    true,
					ChanIds:   []lightning.ChanId{2},
				},
			},
			dismissed: DismissedChannels{
				1: []string{ReasonFailedSwap},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			swapper := getSwapper(t, tc.config)
			db := swapper.database

			for _, swap := range tc.swaps {
				swap.PairId = boltz.PairBtc
				require.NoError(t, db.CreateSwap(swap))
			}

			for _, reverseSwap := range tc.reverseSwaps {
				reverseSwap.PairId = boltz.PairBtc
				require.NoError(t, db.CreateReverseSwap(reverseSwap))
			}

			dismissed, err := swapper.getDismissedChannels()
			require.NoError(t, err)
			require.Equal(t, tc.dismissed, dismissed)
		})
	}

}

func TestCheckSwapRecommendation(t *testing.T) {

	tests := []struct {
		name           string
		config         *Config
		recommendation *rawRecommendation
		outcome        []string
	}{
		{
			name: "MaxFeePercent/High",
			config: &Config{
				MaxFeePercent: 25,
			},
			recommendation: &rawRecommendation{
				Type:   boltz.NormalSwap,
				Amount: 100,
			},
			outcome: nil,
		},
		{
			name: "MaxFeePercent/High",
			config: &Config{
				MaxFeePercent: 25,
				Budget:        150,
			},
			recommendation: &rawRecommendation{

				Type:   boltz.NormalSwap,
				Amount: 100,
			},
			outcome: nil,
		},
		{
			name: "MaxFeePercent/Low",
			config: &Config{
				MaxFeePercent: 10,
			},
			recommendation: &rawRecommendation{
				Type:   boltz.NormalSwap,
				Amount: 100,
			},
			outcome: []string{ReasonMaxFeePercent},
		},
		{
			name: "LowAmount",
			config: &Config{
				MaxFeePercent: 25,
			},
			recommendation: &rawRecommendation{
				Type:   boltz.NormalSwap,
				Amount: 99,
			},
			outcome: []string{ReasonAmountBelowMin},
		},
		{
			name: "HighAmount",
			config: &Config{
				MaxFeePercent: 25,
			},
			recommendation: &rawRecommendation{
				Type:   boltz.NormalSwap,
				Amount: 100000,
			},
			outcome: nil,
		},
		{
			name: "BudgetExceeded",
			config: &Config{
				MaxFeePercent: 25,
				Budget:        10,
			},
			recommendation: &rawRecommendation{
				Type:   boltz.NormalSwap,
				Amount: 10000,
			},
			outcome: []string{ReasonBudgetExceeded},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.config.Budget == 0 {
				tc.config.Budget = tc.recommendation.Amount
			}

			swapper := getSwapper(t, tc.config)

			validated, err := swapper.validateRecommendations([]*rawRecommendation{tc.recommendation}, int64(tc.config.Budget))
			require.NoError(t, err)
			require.Equal(t, tc.outcome, validated[0].DismissedReasons)
		})
	}
}
