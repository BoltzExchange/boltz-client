package autoswap

import (
	"fmt"
	rand2 "golang.org/x/exp/rand"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/lightning"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"

	"github.com/stretchr/testify/require"
)

func getTestDb(t *testing.T) *database.Database {
	db := &database.Database{
		Path: ":memory:",
	}
	require.NoError(t, db.Connect())
	return db
}

func getSwapper(t *testing.T, cfg *SerializedConfig) *AutoSwapper {
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
		GetPairInfo: func(pair *boltzrpc.Pair, swapType boltz.SwapType) (*PairInfo, error) {
			return &PairInfo{
				Limits{
					MinAmount: 100,
					MaxAmount: 1000,
				},
				10,
				10,
			}, nil

		},
	}
	swapper.Init(getTestDb(t), nil, t.TempDir()+"/autoswap.toml")
	if cfg.MaxBalancePercent == 0 && cfg.MinBalancePercent == 0 {
		cfg.MinBalancePercent = 25
		cfg.MaxBalancePercent = 75
	}
	require.NoError(t, swapper.SetConfig(cfg))
	return swapper
}

func randomId() string {
	return fmt.Sprint(rand2.Uint32())
}
func fakeSwap(swap database.Swap) database.Swap {
	swap.EntityId = database.DefaultEntityId
	swap.Id = randomId()
	return swap
}

func fakeReverseSwap(reverseSwap database.ReverseSwap) database.ReverseSwap {
	reverseSwap.EntityId = database.DefaultEntityId
	reverseSwap.Id = randomId()
	return reverseSwap
}

func pastDate(duration time.Duration) time.Time {
	return time.Now().Add(-duration)
}

func TestBudget(t *testing.T) {
	fee := func(amount uint64) *uint64 {
		return &amount
	}

	tests := []struct {
		name            string
		config          *SerializedConfig
		swaps           []database.Swap
		reverseSwaps    []database.ReverseSwap
		expected        int64
		currentInterval *database.BudgetInterval
	}{
		{
			name: "Normal Swaps",
			config: &SerializedConfig{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(database.Swap{
					OnchainFee: fee(10),
					ServiceFee: fee(10),
					IsAuto:     true,
				}),
			},
			expected: 80,
		},
		{
			name: "Reverse Swaps",
			config: &SerializedConfig{
				Budget:         100,
				BudgetInterval: 1000,
			},
			reverseSwaps: []database.ReverseSwap{
				fakeReverseSwap(database.ReverseSwap{
					OnchainFee:     fee(10),
					ServiceFee:     fee(10),
					RoutingFeeMsat: fee(10000),
					IsAuto:         true,
				}),
			},
			expected: 70,
		},
		{
			name: "Auto-Only",
			config: &SerializedConfig{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(database.Swap{
					OnchainFee: fee(10),
					ServiceFee: fee(10),
				}),
			},
			expected: 100,
		},
		{
			name: "New",
			config: &SerializedConfig{
				Budget:         100,
				BudgetInterval: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(database.Swap{
					OnchainFee: fee(10),
					ServiceFee: fee(10),
					IsAuto:     true,
					CreatedAt:  pastDate(1500 * time.Second),
				}),
			},
			currentInterval: &database.BudgetInterval{
				StartDate: pastDate(2000 * time.Second),
				EndDate:   pastDate(1000 * time.Second),
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
		swapper := getSwapper(t, &SerializedConfig{})
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
		config       *SerializedConfig
		veverseSwaps []database.ReverseSwap
		outcome      []*rawRecommendation
		channels     []*lightning.LightningChannel
		err          error
	}{
		{
			name: "PerChannel/Low",
			config: &SerializedConfig{
				PerChannel:        true,
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
				SwapType:          "reverse",
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
			name: "PerChannel/High",
			config: &SerializedConfig{
				PerChannel:        true,
				MaxBalancePercent: 90,
				SwapType:          "reverse",
			},
			outcome: nil,
		},
		{
			name: "TotalBalance/Reverse",
			config: &SerializedConfig{
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
			name: "TotalBalance/Reverse",
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
				SwapType:   "reverse",
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
			config: &SerializedConfig{
				SwapType:   "normal",
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
				MinBalance: 400,
				MaxBalance: 700,
			},
			outcome: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig(tc.config)
			require.NoError(t, cfg.Init())
			if tc.channels == nil {
				tc.channels = channels
			}
			recommendations := cfg.strategy(tc.channels)

			require.Equal(t, tc.outcome, recommendations)
		})

	}
}

func TestDismissedChannels(t *testing.T) {
	tests := []struct {
		name         string
		config       *SerializedConfig
		channels     []*lightning.LightningChannel
		swaps        []database.Swap
		reverseSwaps []database.ReverseSwap
		dismissed    DismissedChannels
	}{
		{
			name: "Pending Swaps",
			config: &SerializedConfig{
				FailureBackoff: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(database.Swap{
					State:   boltzrpc.SwapState_PENDING,
					ChanIds: []lightning.ChanId{1},
					IsAuto:  true,
				}),
			},
			reverseSwaps: []database.ReverseSwap{
				fakeReverseSwap(database.ReverseSwap{
					State:  boltzrpc.SwapState_PENDING,
					IsAuto: true,
				}),
				fakeReverseSwap(database.ReverseSwap{
					State:   boltzrpc.SwapState_SUCCESSFUL,
					IsAuto:  true,
					ChanIds: []lightning.ChanId{3},
				}),
			},
			dismissed: DismissedChannels{
				0: []string{ReasonPendingSwap},
				1: []string{ReasonPendingSwap},
			},
		},
		{
			name: "Failed Swaps",
			config: &SerializedConfig{
				FailureBackoff: 1000,
			},
			swaps: []database.Swap{
				fakeSwap(database.Swap{

					State:   boltzrpc.SwapState_ERROR,
					ChanIds: []lightning.ChanId{1},
					IsAuto:  true,
				}),
			},
			reverseSwaps: []database.ReverseSwap{
				fakeReverseSwap(database.ReverseSwap{
					State:     boltzrpc.SwapState_ERROR,
					CreatedAt: pastDate(2000 * time.Second),
					IsAuto:    true,
					ChanIds:   []lightning.ChanId{2},
				}),
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
				swap.Pair = boltz.PairBtc
				require.NoError(t, db.CreateSwap(swap))
			}

			for _, reverseSwap := range tc.reverseSwaps {
				reverseSwap.Pair = boltz.PairBtc
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
		config         *SerializedConfig
		recommendation *rawRecommendation
		outcome        []string
	}{
		{
			name: "MaxFeePercent/High",
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
			config: &SerializedConfig{
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
