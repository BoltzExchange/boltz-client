package boltz

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCheckFees(t *testing.T) {
	var feeEstimations = FeeEstimations{
		CurrencyBtc:    2,
		CurrencyLiquid: 0.11,
	}
	type args struct {
		swapType       SwapType
		pair           Pair
		sendAmount     uint64
		receiveAmount  uint64
		serviceFee     Percentage
		feeEstimations FeeEstimations
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Reverse/Liquid",
			args: args{
				swapType:       ReverseSwap,
				pair:           Pair{From: CurrencyBtc, To: CurrencyLiquid},
				sendAmount:     50000,
				receiveAmount:  49530,
				serviceFee:     0.1,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Reverse/Btc",
			args: args{
				swapType:       ReverseSwap,
				pair:           Pair{From: CurrencyBtc, To: CurrencyBtc},
				sendAmount:     50000,
				receiveAmount:  49220,
				serviceFee:     0.5,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Submarine/Liquid",
			args: args{
				swapType:       NormalSwap,
				pair:           Pair{From: CurrencyLiquid, To: CurrencyBtc},
				sendAmount:     50198,
				receiveAmount:  50000,
				serviceFee:     0.1,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Submarine/Btc",
			args: args{
				swapType:       NormalSwap,
				pair:           Pair{From: CurrencyBtc, To: CurrencyBtc},
				sendAmount:     50552,
				receiveAmount:  50000,
				serviceFee:     0.5,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Chain/Btc",
			args: args{
				swapType:       ChainSwap,
				pair:           Pair{From: CurrencyBtc, To: CurrencyLiquid},
				sendAmount:     50000,
				receiveAmount:  49228,
				serviceFee:     0.1,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Chain/Liquid",
			args: args{
				swapType:       ChainSwap,
				pair:           Pair{From: CurrencyLiquid, To: CurrencyBtc},
				sendAmount:     50000,
				receiveAmount:  49272,
				serviceFee:     0.1,
				feeEstimations: feeEstimations,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			includeClaim := true
			require.NoError(t, CheckAmounts(tt.args.swapType, tt.args.pair, tt.args.sendAmount, tt.args.receiveAmount, tt.args.serviceFee, tt.args.feeEstimations, includeClaim))

			// boltz trying to scam us
			tt.args.receiveAmount -= AbsoluteFeeToleranceSat + 100
			require.Error(t, CheckAmounts(tt.args.swapType, tt.args.pair, tt.args.sendAmount, tt.args.receiveAmount, tt.args.serviceFee, tt.args.feeEstimations, includeClaim))
		})
	}
}

func Test_checkTolerance(t *testing.T) {
	type args struct {
		expected uint64
		actual   uint64
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Equal",
			args: args{
				expected: 100,
				actual:   100,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Absolute/Within",
			args: args{
				expected: 25,
				actual:   100,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Absolute/Outside",
			args: args{
				expected: AbsoluteFeeToleranceSat,
				actual:   AbsoluteFeeToleranceSat * 3,
			},
			wantErr: assert.Error,
		},
		{
			name: "Relative/Within",
			args: args{
				expected: 100000 - AbsoluteFeeToleranceSat*2,
				actual:   100000,
			},
			wantErr: assert.NoError,
		},
		{
			name: "Relative/Outside",
			args: args{
				expected: 70000,
				actual:   100000,
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, checkTolerance(tt.args.expected, tt.args.actual), fmt.Sprintf("checkTolerance(%v, %v)", tt.args.expected, tt.args.actual))
		})
	}
}
