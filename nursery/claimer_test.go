package nursery

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	onchainmock "github.com/BoltzExchange/boltz-client/mocks/github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/test"
	"testing"
	"time"
)

func fakeOutput(currency boltz.Currency, timeoutBlockHeight uint32, amount uint64) *CheckedOutput {
	return &CheckedOutput{
		Output: &Output{
			findArgs: onchain.OutputArgs{
				Currency: currency,
			},
			OutputDetails: &boltz.OutputDetails{
				SwapId:             test.RandomId(),
				TimeoutBlockHeight: timeoutBlockHeight,
			},
		},
		outputResult: &onchain.OutputResult{
			Value: amount,
		},
	}
}

func TestClaimerShouldSweep(t *testing.T) {
	tests := []struct {
		name            string
		claimer         Claimer
		currentHeight   uint32
		existingOutputs []*CheckedOutput
		output          *CheckedOutput
		want            SweepReason
	}{
		{
			name: "Disabled",
			claimer: Claimer{
				Symbols: []boltz.Currency{boltz.CurrencyBtc},
			},
			output: fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:   ReasonForced,
		},
		{
			name: "Enabled/None",
			claimer: Claimer{
				Interval: time.Second,
				Symbols:  []boltz.Currency{boltz.CurrencyBtc},
			},
			output: fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:   ReasonNone,
		},
		{
			name: "Enabled/Expiry/Low",
			claimer: Claimer{
				Interval:        time.Second,
				Symbols:         []boltz.Currency{boltz.CurrencyBtc},
				ExpiryTolerance: time.Hour,
			},
			currentHeight: 10,
			output:        fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:          ReasonExpiry,
		},
		{
			name: "Enabled/Expiry/High",
			claimer: Claimer{
				Interval:        time.Second,
				Symbols:         []boltz.Currency{boltz.CurrencyBtc},
				ExpiryTolerance: time.Hour,
			},
			currentHeight: 2,
			output:        fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:          ReasonNone,
		},
		{
			name: "Enabled/Count",
			claimer: Claimer{
				Interval: time.Second,
				Symbols:  []boltz.Currency{boltz.CurrencyBtc},
				MaxCount: 2,
			},
			existingOutputs: []*CheckedOutput{
				fakeOutput(boltz.CurrencyBtc, 10, 100),
				fakeOutput(boltz.CurrencyBtc, 10, 100),
			},
			output: fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:   ReasonCount,
		},
		{
			name: "Enabled/Amount",
			claimer: Claimer{
				Interval:   time.Second,
				Symbols:    []boltz.Currency{boltz.CurrencyBtc},
				MaxBalance: 200,
			},
			existingOutputs: []*CheckedOutput{
				fakeOutput(boltz.CurrencyBtc, 10, 100),
				fakeOutput(boltz.CurrencyBtc, 10, 100),
			},
			output: fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:   ReasonAmount,
		},
		{
			name: "Enabled/OtherCurrency",
			claimer: Claimer{
				Interval: time.Second,
				Symbols:  []boltz.Currency{boltz.CurrencyLiquid},
			},
			output: fakeOutput(boltz.CurrencyBtc, 10, 100),
			want:   ReasonNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claimer := tt.claimer
			if claimer.MaxCount == 0 {
				claimer.MaxCount = 200
			}
			if claimer.MaxBalance == 0 {
				claimer.MaxBalance = 2000
			}
			blockProvider := onchainmock.NewMockBlockProvider(t)
			blockProvider.EXPECT().GetBlockHeight().Return(tt.currentHeight, nil).Maybe()
			claimer.Init(&onchain.Onchain{
				Btc: &onchain.Currency{
					Blocks: blockProvider,
				},
				Liquid: &onchain.Currency{
					Blocks: blockProvider,
				},
			})
			for _, output := range tt.existingOutputs {
				claimer.shouldSweep(output)
			}
			if got := claimer.shouldSweep(tt.output); got != tt.want {
				t.Errorf("shouldSweep() = %v, want %v", got, tt.want)
			}
		})
	}
}
