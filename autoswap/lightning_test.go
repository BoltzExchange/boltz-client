package autoswap

import (
	"fmt"
	"golang.org/x/exp/rand"
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
	if cfg.MaxBalancePercent == 0 && cfg.MinBalancePercent == 0 {
		cfg.MinBalancePercent = 25
		cfg.MaxBalancePercent = 75
	}
	require.NoError(t, swapper.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: cfg}))
	return swapper.lnSwapper, mockProvider
}

func randomId() string {
	return fmt.Sprint(rand.Uint32())
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
			MaxFeePercent:     10,
			MaxBalancePercent: defaults.MaxBalancePercent - 10,
		},
		FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"max_balance_percent"}},
	})
	require.NoError(t, err)
	require.Equal(t, defaults.MaxFeePercent, lnSwapper.cfg.MaxFeePercent)
	require.NotEqual(t, defaults.MaxBalancePercent, lnSwapper.cfg.MaxBalancePercent)
	require.False(t, lnSwapper.Running())
	require.Empty(t, lnSwapper.Error())
}

func TestBudget(t *testing.T) {
	fee := func(amount uint64) *uint64 {
		return &amount
	}

	tests := []struct {
		name            string
		config          *SerializedLnConfig
		swaps           []database.Swap
		reverseSwaps    []database.ReverseSwap
		expected        int64
		currentInterval *database.BudgetInterval
	}{
		{
			name: "Normal Swaps",
			config: &SerializedLnConfig{
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
			config: &SerializedLnConfig{
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
			config: &SerializedLnConfig{
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
			config: &SerializedLnConfig{
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
			swapper, _ := getLnSwapper(t, tc.config)
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
		swapper, _ := getLnSwapper(t, &SerializedLnConfig{})
		budget, err := swapper.GetCurrentBudget(false)
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
		config       *SerializedLnConfig
		veverseSwaps []database.ReverseSwap
		outcome      []*lightningRecommendation
		channels     []*lightning.LightningChannel
		err          error
	}{
		{
			name: "PerChannel/Low",
			config: &SerializedLnConfig{
				PerChannel:        true,
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
				SwapType:          "reverse",
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 200, channels[2]),
			},
		},
		{
			name: "PerChannel/High",
			config: &SerializedLnConfig{
				PerChannel:        true,
				MaxBalancePercent: 90,
				SwapType:          "reverse",
			},
			outcome: nil,
		},
		{
			name: "TotalBalance/Reverse",
			config: &SerializedLnConfig{
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Reverse",
			config: &SerializedLnConfig{
				MaxBalancePercent: 60,
				MinBalancePercent: 40,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Normal",
			config: &SerializedLnConfig{
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
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 150, nil),
			},
		},
		{
			name: "TotalBalance/Max",
			config: &SerializedLnConfig{
				SwapType:   "reverse",
				MaxBalance: 600,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 350, nil),
			},
		},
		{
			name: "TotalBalance/Min",
			config: &SerializedLnConfig{
				SwapType:   "normal",
				MinBalance: 400,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 400, nil),
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
			config: &SerializedLnConfig{
				MinBalance: 400,
				MaxBalance: 600,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.NormalSwap, 200, nil),
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
			config: &SerializedLnConfig{
				MinBalance: 400,
				MaxBalance: 600,
			},
			outcome: []*lightningRecommendation{
				recommendation(boltz.ReverseSwap, 200, nil),
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
			config: &SerializedLnConfig{
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
			require.NoError(t, cfg.Init(nil))
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
		config       *SerializedLnConfig
		channels     []*lightning.LightningChannel
		swaps        []database.Swap
		reverseSwaps []database.ReverseSwap
		dismissed    DismissedChannels
	}{
		{
			name: "Pending Swaps",
			config: &SerializedLnConfig{
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
			config: &SerializedLnConfig{
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
			swapper, _ := getLnSwapper(t, tc.config)
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

			validated, err := swapper.validateRecommendations([]*lightningRecommendation{tc.recommendation}, int64(tc.config.Budget))
			require.NoError(t, err)
			require.Equal(t, tc.outcome, validated[0].DismissedReasons)
		})
	}
}
