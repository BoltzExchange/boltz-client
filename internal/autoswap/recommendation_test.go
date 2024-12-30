package autoswap

import (
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRawRecommendation(t *testing.T) {

	limits := &boltzrpc.Limits{
		Minimal: 100,
		Maximal: 1000,
	}

	fees := &boltzrpc.SwapFees{
		Percentage: 10,
		MinerFees:  100,
	}

	tests := []struct {
		name   string
		amount uint64
		params checkParams
		result checks
		budget uint64
	}{
		{
			name:   "AmountBelowMin",
			amount: 50,
			params: checkParams{
				MaxFeePercent: 50,
				Pair: &boltzrpc.PairInfo{
					Limits: limits,
					Fees: &boltzrpc.SwapFees{
						Percentage: 0,
						MinerFees:  0,
					},
				},
			},
			result: checks{
				DismissedReasons: []string{ReasonAmountBelowMin},
			},
		},
		{
			name:   "HighAmount",
			amount: 2000,
			params: checkParams{
				MaxFeePercent: 25,
				Pair: &boltzrpc.PairInfo{
					Limits: limits,
					Fees:   fees,
				},
			},
			result: checks{
				Amount:      limits.Maximal,
				FeeEstimate: 200,
			},
		},
		{
			name:   "MaxFeePercent",
			amount: 500,
			params: checkParams{
				MaxFeePercent: 1,
				Pair: &boltzrpc.PairInfo{
					Limits: limits,
					Fees:   fees,
				},
			},
			result: checks{
				FeeEstimate:      150,
				DismissedReasons: []string{ReasonMaxFeePercent},
			},
		},
		{
			name:   "BudgetExceeded",
			amount: 500,
			budget: 100,
			params: checkParams{
				MaxFeePercent: 50,
				Pair: &boltzrpc.PairInfo{
					Limits: limits,
					Fees:   fees,
				},
			},
			result: checks{
				FeeEstimate:      150,
				DismissedReasons: []string{ReasonBudgetExceeded},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.result.Amount == 0 {
				tc.result.Amount = tc.amount
			}
			if tc.budget == 0 {
				tc.budget = tc.amount
			}
			initialBudget := tc.budget
			tc.params.Budget = &tc.budget
			checks := check(tc.amount, tc.params)
			require.Equal(t, tc.result, checks)
			if initialBudget > checks.FeeEstimate {
				require.Equal(t, initialBudget-checks.FeeEstimate, tc.budget)
			}
		})
	}
}
