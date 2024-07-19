package autoswap

import (
	"github.com/BoltzExchange/boltz-client/test"
	"github.com/BoltzExchange/boltz-client/utils"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func getLnSwapper(t *testing.T, cfg *SerializedLnConfig) (*LightningSwapper, *MockRpcProvider) {
	swapper, mockProvider := getSwapper(t)
	if cfg.InboundBalancePercent == 0 && cfg.OutboundBalancePercent == 0 {
		cfg.OutboundBalancePercent = 25
		cfg.InboundBalancePercent = 25
	}
	require.NoError(t, swapper.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: cfg}))
	return swapper.lnSwapper, mockProvider
}

func TestSetLnConfig(t *testing.T) {
	swapper, _ := getSwapper(t)
	defaults := DefaultLightningConfig()
	reset := true
	err := swapper.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Reset_: &reset})
	require.NoError(t, err)
	require.Empty(t, swapper.lnSwapper.err)

	lnSwapper := swapper.lnSwapper

	err = swapper.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{
		Config: &SerializedLnConfig{
			MaxFeePercent:         10,
			InboundBalancePercent: defaults.InboundBalancePercent - 10,
		},
		FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"inbound_balance_percent"}},
	})
	require.NoError(t, err)
	require.Equal(t, defaults.MaxFeePercent, lnSwapper.cfg.MaxFeePercent)
	require.NotEqual(t, defaults.InboundBalancePercent, lnSwapper.cfg.InboundBalancePercent)
	require.False(t, lnSwapper.Running())
	require.Empty(t, lnSwapper.Error())
}

func TestBudget(t *testing.T) {
	fee := func(amount uint64) *uint64 {
		return &amount
	}

	chainSwaps := []database.ChainSwap{
		{
			OnchainFee: fee(10),
			ServiceFee: fee(15),
			IsAuto:     true,
		},
		{
			OnchainFee: fee(10),
			ServiceFee: fee(15),
			IsAuto:     false,
		},
	}

	swaps := []database.Swap{
		{
			OnchainFee: fee(10),
			ServiceFee: fee(15),
			IsAuto:     true,
		},
		{
			OnchainFee: fee(10),
			ServiceFee: fee(15),
			IsAuto:     false,
		},
	}

	reverseSwaps := []database.ReverseSwap{
		{
			OnchainFee:     fee(10),
			ServiceFee:     fee(10),
			RoutingFeeMsat: fee(5000),
			IsAuto:         true,
		},
		{
			OnchainFee:     fee(10),
			ServiceFee:     fee(10),
			RoutingFeeMsat: fee(5000),
			IsAuto:         false,
		},
	}
	allSwaps := test.FakeSwaps{
		Swaps:        swaps,
		ReverseSwaps: reverseSwaps,
		ChainSwaps:   chainSwaps,
	}

	tests := []struct {
		name            string
		budget          uint64
		interval        time.Duration
		fakeSwaps       test.FakeSwaps
		expected        int64
		currentInterval *database.BudgetInterval
		swapperType     SwapperType
	}{
		{
			name:        "All/Lightning",
			budget:      100,
			interval:    1000 * time.Second,
			fakeSwaps:   allSwaps,
			expected:    50,
			swapperType: Lightning,
		},
		{
			name:        "All/Chain",
			budget:      100,
			interval:    5 * time.Minute,
			fakeSwaps:   allSwaps,
			expected:    75,
			swapperType: Chain,
		},
		{
			name:     "Swaps",
			budget:   100,
			interval: 1000 * time.Second,
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
					},
				},
				ChainSwaps: chainSwaps,
			},
			expected:    80,
			swapperType: Lightning,
		},
		{
			name:     "Reverse Swaps",
			budget:   100,
			interval: 1000 * time.Second,
			fakeSwaps: test.FakeSwaps{
				ReverseSwaps: []database.ReverseSwap{
					{
						OnchainFee:     fee(10),
						ServiceFee:     fee(10),
						RoutingFeeMsat: fee(10000),
						IsAuto:         true,
					},
				},
				ChainSwaps: chainSwaps,
			},
			expected:    70,
			swapperType: Lightning,
		},
		{
			name:     "Auto-Only",
			budget:   100,
			interval: 1000 * time.Second,
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
					},
				},
				ChainSwaps: chainSwaps,
			},
			expected:    100,
			swapperType: Lightning,
		},
		{
			name:     "New/Lightning",
			budget:   100,
			interval: 5 * time.Minute,
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(7 * time.Minute),
					},
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(2 * time.Minute),
					},
				},
				ChainSwaps: []database.ChainSwap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
					},
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     false,
					},
				},
			},
			currentInterval: &database.BudgetInterval{
				StartDate: test.PastDate(8 * time.Minute),
				EndDate:   test.PastDate(3 * time.Minute),
			},
			expected:    80,
			swapperType: Lightning,
		},
		{
			name:     "New/Chain",
			budget:   100,
			interval: 5 * time.Minute,
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(7 * time.Minute),
					},
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(2 * time.Minute),
					},
				},
				ChainSwaps: []database.ChainSwap{
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(7 * time.Minute),
					},
					{
						OnchainFee: fee(10),
						ServiceFee: fee(10),
						IsAuto:     true,
						CreatedAt:  test.PastDate(2 * time.Minute),
					},
				},
			},
			currentInterval: &database.BudgetInterval{
				StartDate: test.PastDate(8 * time.Minute),
				EndDate:   test.PastDate(3 * time.Minute),
			},
			expected:    80,
			swapperType: Chain,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := getTestDb(t)

			c := shared{database: db}
			var swapperType SwapperType
			get := func(createIfMissing bool) (*Budget, error) {
				var cfg budgetConfig
				if tc.swapperType == Lightning {
					cfg = &SerializedLnConfig{
						Budget:         tc.budget,
						BudgetInterval: uint64(tc.interval.Seconds()),
					}
					swapperType = Lightning
				} else {
					cfg = &SerializedChainConfig{
						Budget:         tc.budget,
						BudgetInterval: uint64(tc.interval.Seconds()),
					}
					swapperType = Chain
				}

				return c.GetCurrentBudget(
					createIfMissing,
					swapperType,
					cfg,
					database.DefaultTenantId,
				)
			}

			budget, err := get(false)
			require.NoError(t, err)
			require.Nil(t, budget)

			if tc.currentInterval != nil {
				tc.currentInterval.TenantId = database.DefaultTenantId
				tc.currentInterval.Name = string(tc.swapperType)
				require.NoError(t, db.CreateBudget(*tc.currentInterval))
			}

			budget, err = get(true)
			require.NoError(t, err)
			require.Equal(t, int64(tc.budget), budget.Amount)

			tc.fakeSwaps.Create(t, db)

			budget, err = get(true)
			require.NoError(t, err)
			require.Equal(t, tc.expected, budget.Amount)
		})
	}

	t.Run("Missing", func(t *testing.T) {
		swapper, _ := getLnSwapper(t, &SerializedLnConfig{})
		budget, err := swapper.cfg.GetCurrentBudget(false)
		require.NoError(t, err)
		require.Nil(t, budget)
	})

}

func recommendation(t boltz.SwapType, a uint64, c *lightning.LightningChannel) *lightningRecommendation {
	return &lightningRecommendation{
		Amount:  a,
		Type:    t,
		Channel: c,
	}
}

func TestStrategies(t *testing.T) {

	channels := []*lightning.LightningChannel{
		{
			OutboundSat: 100,
			InboundSat:  100,
			Capacity:    200,
			Id:          1,
		},
		{
			OutboundSat: 50,
			InboundSat:  150,
			Capacity:    200,
			Id:          2,
		},
		{
			OutboundSat: 500,
			InboundSat:  100,
			Capacity:    600,
			Id:          3,
		},
	}

	tests := []struct {
		name         string
		config       *SerializedLnConfig
		veverseSwaps []database.ReverseSwap
		outcome      []*lightningRecommendation
		channels     []*lightning.LightningChannel
		err          error
		reserve      utils.Percentage
	}{
		{
			name: "PerChannel/Low",
			config: &SerializedLnConfig{
				PerChannel:             true,
				InboundBalancePercent:  40,
				OutboundBalancePercent: 40, // ignored because only reverse swaps are executed
				SwapType:               "reverse",
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 500, channels[2]),
			},
		},
		{
			name: "PerChannel/High",
			config: &SerializedLnConfig{
				PerChannel:            true,
				InboundBalancePercent: 10,
				SwapType:              "reverse",
			},
			outcome: nil,
		},
		{
			name: "TotalBalance/Reverse",
			config: &SerializedLnConfig{
				InboundBalancePercent:  40,
				OutboundBalancePercent: 40,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Reverse",
			config: &SerializedLnConfig{
				InboundBalancePercent:  40,
				OutboundBalancePercent: 40,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Normal",
			config: &SerializedLnConfig{
				InboundBalancePercent:  40,
				OutboundBalancePercent: 40,
			},
			channels: []*lightning.LightningChannel{
				{
					OutboundSat: 100,
					InboundSat:  100,
					Capacity:    200,
					Id:          1,
				},
				{
					OutboundSat: 150,
					InboundSat:  50,
					Capacity:    200,
					Id:          2,
				},
				{
					OutboundSat: 100,
					InboundSat:  500,
					Capacity:    600,
					Id:          3,
				},
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Inbound",
			config: &SerializedLnConfig{
				SwapType:        "reverse",
				InboundBalance:  400,
				OutboundBalance: 200, // ignored because only reverse swaps are executed
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 650, nil),
			},
		},
		{
			name: "TotalBalance/Outbound",
			config: &SerializedLnConfig{
				SwapType:        "normal",
				OutboundBalance: 700,
				InboundBalance:  200, // ignored because only normal swaps are executed
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 350, nil),
			},
		},
		{
			name: "TotalBalance/Both/Above",
			config: &SerializedLnConfig{
				OutboundBalance: 400,
				InboundBalance:  400,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 200, nil),
			},
			channels: []*lightning.LightningChannel{
				{
					OutboundSat: 300,
					InboundSat:  700,
					Capacity:    1000,
				},
			},
		},
		{
			name: "TotalBalance/Both/Below",
			config: &SerializedLnConfig{
				OutboundBalance: 400,
				InboundBalance:  400,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 200, nil),
			},
			channels: []*lightning.LightningChannel{
				{
					OutboundSat: 700,
					InboundSat:  300,
					Capacity:    1000,
				},
			},
		},
		{
			name: "TotalBalance/None",
			config: &SerializedLnConfig{
				OutboundBalance: 400,
				InboundBalance:  300,
			},
			outcome: nil,
		},
		{
			name: "Reserve",
			config: &SerializedLnConfig{
				InboundBalancePercent: 50,
				SwapType:              "reverse",
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 600, nil),
			},
			reserve: 10,
			channels: []*lightning.LightningChannel{
				{
					OutboundSat: 700,
					InboundSat:  300,
					Capacity:    1000,
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewLightningConfig(tc.config, shared{onchain: getOnchain()})
			cfg.reserve = tc.reserve
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
		name      string
		config    *SerializedLnConfig
		channels  []*lightning.LightningChannel
		fakeSwaps test.FakeSwaps
		dismissed DismissedChannels
	}{
		{
			name: "Pending Swaps",
			config: &SerializedLnConfig{
				FailureBackoff: 1000,
			},
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						State:   boltzrpc.SwapState_PENDING,
						ChanIds: []lightning.ChanId{1},
						IsAuto:  true,
					},
					{
						State:   boltzrpc.SwapState_PENDING,
						ChanIds: []lightning.ChanId{2},
						IsAuto:  false,
					},
				},
				ReverseSwaps: []database.ReverseSwap{
					{
						State:  boltzrpc.SwapState_PENDING,
						IsAuto: true,
					},
					{
						State:   boltzrpc.SwapState_SUCCESSFUL,
						IsAuto:  true,
						ChanIds: []lightning.ChanId{3},
					},
					{
						State:   boltzrpc.SwapState_SUCCESSFUL,
						IsAuto:  false,
						ChanIds: []lightning.ChanId{2},
					},
				},
			},
			dismissed: DismissedChannels{
				0: []string{ReasonPendingSwap},
				1: []string{ReasonPendingSwap},
			},
		},
		{
			name: "Failed Swaps",
			config: &SerializedLnConfig{
				FailureBackoff: 1000,
			},
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						State:   boltzrpc.SwapState_ERROR,
						ChanIds: []lightning.ChanId{1},
						IsAuto:  true,
					},
					{
						State:   boltzrpc.SwapState_ERROR,
						ChanIds: []lightning.ChanId{2},
						IsAuto:  false,
					},
				},
				ReverseSwaps: []database.ReverseSwap{
					{
						State:     boltzrpc.SwapState_ERROR,
						CreatedAt: test.PastDate(2000 * time.Second),
						IsAuto:    true,
						ChanIds:   []lightning.ChanId{2},
					},
					{
						State:   boltzrpc.SwapState_ERROR,
						ChanIds: []lightning.ChanId{3},
						IsAuto:  false,
					},
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
			swapper, _ := getLnSwapper(t, tc.config)
			db := swapper.database

			tc.fakeSwaps.Create(t, db)

			dismissed, err := swapper.cfg.getDismissedChannels()
			require.NoError(t, err)
			require.Equal(t, tc.dismissed, dismissed)
		})
	}

}

func TestCheckSwapRecommendation(t *testing.T) {

	tests := []struct {
		name           string
		config         *SerializedLnConfig
		recommendation *lightningRecommendation
		outcome        []string
	}{
		{
			name: "MaxFeePercent/High",
			config: &SerializedLnConfig{
				MaxFeePercent: 25,
			},
			recommendation: recommendation(boltz.NormalSwap, 100, nil),
			outcome:        nil,
		},
		{
			name: "MaxFeePercent/High",
			config: &SerializedLnConfig{
				MaxFeePercent: 25,
				Budget:        150,
			},
			recommendation: recommendation(boltz.NormalSwap, 100, nil),
			outcome:        nil,
		},
		{
			name: "MaxFeePercent/Low",
			config: &SerializedLnConfig{
				MaxFeePercent: 10,
			},
			recommendation: recommendation(boltz.NormalSwap, 100, nil),
			outcome:        []string{ReasonMaxFeePercent},
		},
		{
			name: "LowAmount",
			config: &SerializedLnConfig{
				MaxFeePercent: 25,
			},
			recommendation: recommendation(boltz.NormalSwap, 99, nil),
			outcome:        []string{ReasonAmountBelowMin},
		},
		{
			name: "HighAmount",
			config: &SerializedLnConfig{
				MaxFeePercent: 25,
			},
			recommendation: recommendation(boltz.NormalSwap, 100000, nil),
			outcome:        nil,
		},
		{
			name: "BudgetExceeded",
			config: &SerializedLnConfig{
				MaxFeePercent: 25,
				Budget:        10,
			},
			recommendation: recommendation(boltz.NormalSwap, 10000, nil),
			outcome:        []string{ReasonBudgetExceeded},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.config.Budget == 0 {
				tc.config.Budget = tc.recommendation.Amount
			}

			swapper, ln := getLnSwapper(t, tc.config)
			pairInfo := newPairInfo()
			ln.EXPECT().GetAutoSwapPairInfo(mock.Anything, mock.Anything).Return(pairInfo, nil)

			validated, err := swapper.cfg.validateRecommendations([]*lightningRecommendation{tc.recommendation}, int64(tc.config.Budget))
			require.NoError(t, err)
			require.Equal(t, tc.outcome, validated[0].DismissedReasons)
		})
	}
}
