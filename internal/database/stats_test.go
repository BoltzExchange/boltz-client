package database_test

import (
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	fee := func(amount uint64) *uint64 {
		return &amount
	}
	serviceFee := func(amount int64) *int64 {
		return &amount
	}

	tests := []struct {
		name      string
		fakeSwaps test.FakeSwaps
		expected  *boltzrpc.SwapStats
		query     database.SwapQuery
	}{
		{
			name: "All",
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						State:          boltzrpc.SwapState_PENDING,
						ExpectedAmount: 100,
						OnchainFee:     fee(10),
						ServiceFee:     serviceFee(15),
						IsAuto:         true,
					},
					{
						ExpectedAmount: 100,
						State:          boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee:     fee(10),
						ServiceFee:     serviceFee(15),
						IsAuto:         false,
					},
				},
				ReverseSwaps: []database.ReverseSwap{
					{
						State:          boltzrpc.SwapState_SERVER_ERROR,
						InvoiceAmount:  100,
						OnchainFee:     fee(10),
						ServiceFee:     serviceFee(10),
						RoutingFeeMsat: fee(5000),
						IsAuto:         true,
					},
					{
						InvoiceAmount: 100,
						State:         boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee:    fee(10),
						ServiceFee:    serviceFee(15),
						IsAuto:        false,
					},
				},
				ChainSwaps: []database.ChainSwap{
					{
						State:      boltzrpc.SwapState_ERROR,
						FromData:   &database.ChainSwapData{Amount: 100},
						OnchainFee: fee(10),
						ServiceFee: serviceFee(15),
						IsAuto:     true,
					},
					{
						FromData:   &database.ChainSwapData{Amount: 100},
						State:      boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee: fee(10),
						ServiceFee: serviceFee(15),
						IsAuto:     false,
					},
				},
			},
			expected: &boltzrpc.SwapStats{
				TotalFees:    150,
				TotalAmount:  300,
				SuccessCount: 3,
				Count:        6,
			},
		},
		{
			name: "Negative fees",
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						ExpectedAmount: 100,
						State:          boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee:     fee(10),
						ServiceFee:     serviceFee(-15),
						IsAuto:         false,
					},
				},
				ReverseSwaps: []database.ReverseSwap{
					{
						InvoiceAmount: 100,
						State:         boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee:    fee(10),
						ServiceFee:    serviceFee(-15),
						IsAuto:        false,
					},
				},
				ChainSwaps: []database.ChainSwap{
					{
						FromData:   &database.ChainSwapData{Amount: 100},
						State:      boltzrpc.SwapState_SUCCESSFUL,
						OnchainFee: fee(10),
						ServiceFee: serviceFee(-15),
						IsAuto:     false,
					},
				},
			},
			expected: &boltzrpc.SwapStats{
				TotalFees:    -15,
				TotalAmount:  300,
				SuccessCount: 3,
				Count:        3,
			},
		},
		{
			name: "Past",
			fakeSwaps: test.FakeSwaps{
				Swaps: []database.Swap{
					{
						OnchainFee: fee(10),
						ServiceFee: serviceFee(10),
						CreatedAt:  test.PastDate(2 * time.Minute),
					},
				},
				ReverseSwaps: []database.ReverseSwap{
					{
						OnchainFee:     fee(10),
						ServiceFee:     serviceFee(10),
						RoutingFeeMsat: fee(5000),
						CreatedAt:      test.PastDate(2 * time.Minute),
					},
				},
				ChainSwaps: []database.ChainSwap{
					{
						OnchainFee: fee(10),
						ServiceFee: serviceFee(15),
						CreatedAt:  test.PastDate(2 * time.Minute),
					},
				},
			},
			query: database.SwapQuery{
				Since: test.PastDate(1 * time.Minute),
			},
			expected: &boltzrpc.SwapStats{
				TotalFees:    0,
				TotalAmount:  0,
				SuccessCount: 0,
				Count:        0,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := database.Database{Path: ":memory:"}
			err := db.Connect()
			require.NoError(t, err)

			tc.fakeSwaps.Create(t, &db)

			stats, err := db.QueryStats(tc.query, []boltz.SwapType{boltz.NormalSwap, boltz.ReverseSwap, boltz.ChainSwap})
			require.NoError(t, err)
			stats.AvgFees = 0
			stats.AvgAmount = 0
			require.Equal(t, tc.expected, stats)
		})
	}
}
