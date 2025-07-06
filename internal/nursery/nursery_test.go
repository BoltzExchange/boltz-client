package nursery

import (
	"errors"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	lnmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const defaultFeeLimitPpm = uint64(1000)

func setup(t *testing.T) *Nursery {
	chain := &onchain.Onchain{
		Btc:     &onchain.Currency{},
		Liquid:  &onchain.Currency{},
		Network: boltz.Regtest,
	}
	chain.Init()

	db := database.Database{Path: ":memory:"}
	require.NoError(t, db.Connect())

	nursery := New(
		nil,
		defaultFeeLimitPpm,
		boltz.Regtest,
		nil,
		chain,
		&boltz.Api{URL: boltz.Regtest.DefaultBoltzUrl},
		&db,
	)

	return nursery
}

func TestPayReverseSwap(t *testing.T) {
	t.Run("MaxRoutingFee", func(t *testing.T) {
		testInvoice := "lnbcrt10m1p5y4z9epp5hh09qu0605hcjvc5r6dv3ma0z45h7pxjcp4xv383avzxk4yf0tlsdqqcqzzsxqyz5vqsp5nzsy8g59gvlp694x7rc7gxfllk0wswl95vvk5eguc30jrvcqeuws9qxpqysgqmfdaryxsaze7s26ew6y4zu3hk8p9sj8ezcpcvt6rchjuxva5zvwyq7897ffw4mjmsg6efugt5k7qhfy04j6wxnlzpfu48r5mjsruzugqjp04ec"
		testInvoiceAmount := uint64(1_000_000)
		maxRoutingFeePpm := uint64(100)
		expectedLimit := uint(maxRoutingFeePpm) // 1 million sat invoice

		setup := func(t *testing.T) *Nursery {
			nursery := setup(t)
			mockLightning := lnmock.NewMockLightningNode(t)
			mockLightning.EXPECT().
				PayInvoice(mock.Anything, testInvoice, expectedLimit, mock.Anything, mock.Anything).
				Return(&lightning.PayInvoiceResponse{
					FeeMsat: 1,
				}, nil)
			mockLightning.EXPECT().
				PaymentStatus(mock.Anything).
				Return(nil, errors.New("invoice not found"))
			nursery.lightning = mockLightning
			return nursery
		}
		testSwap := &database.ReverseSwap{
			Id:            "test-swap",
			Invoice:       testInvoice,
			InvoiceAmount: testInvoiceAmount,
		}

		t.Run("Custom", func(t *testing.T) {
			nursery := setup(t)
			swap := testSwap
			swap.RoutingFeeLimitPpm = &maxRoutingFeePpm

			err := nursery.payReverseSwap(swap)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // wait for the pay call to run in the goroutine
		})

		t.Run("Default", func(t *testing.T) {
			nursery := setup(t)
			nursery.maxRoutingFeePpm = maxRoutingFeePpm
			err := nursery.payReverseSwap(testSwap)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // wait for the pay call to run in the goroutine
		})

	})
	t.Run("ExternalPay", func(t *testing.T) {
		nursery := setup(t)
		swap := &database.ReverseSwap{
			ExternalPay: true,
		}

		err := nursery.payReverseSwap(swap)
		require.NoError(t, err) // Should not attempt to pay external swaps
	})

	t.Run("NoLightning", func(t *testing.T) {
		nursery := setup(t)
		nursery.lightning = nil
		err := nursery.payReverseSwap(&database.ReverseSwap{
			ExternalPay: false,
		})
		require.Error(t, err)
		require.Equal(t, "no lightning node available to pay invoice", err.Error())
	})
}

func TestChooseDirectOutput(t *testing.T) {
	test.InitLogger()
	nursery := setup(t)
	tests := []struct {
		name           string
		outputs        []*onchain.Output
		expected       *onchain.Output
		swap           *database.ReverseSwap
		feeEstimations boltz.FeeEstimations
		err            require.ErrorAssertionFunc
	}{
		{
			name: "Match",
			outputs: []*onchain.Output{
				{
					TxId:  "tx1",
					Value: 1000,
				},
				{
					TxId:  "tx3",
					Value: 15000,
				},
				{
					TxId:  "tx2",
					Value: 9000,
				},
			},
			expected: &onchain.Output{
				TxId:  "tx2",
				Value: 9000,
			},
			swap: &database.ReverseSwap{
				Pair: boltz.Pair{
					From: boltz.CurrencyBtc,
					To:   boltz.CurrencyLiquid,
				},
				ServiceFeePercent: 1,
				InvoiceAmount:     10000,
			},
			feeEstimations: boltz.FeeEstimations{
				boltz.CurrencyLiquid: 0.1,
				boltz.CurrencyBtc:    1,
			},
			err: require.NoError,
		},
		{
			name: "NoMatch",
			outputs: []*onchain.Output{
				{
					TxId:  "tx1",
					Value: 500,
				},
				{
					TxId:  "tx2",
					Value: 600,
				},
			},
			swap: &database.ReverseSwap{
				Pair: boltz.Pair{
					From: boltz.CurrencyBtc,
					To:   boltz.CurrencyBtc,
				},
				InvoiceAmount:     10000,
				ServiceFeePercent: 1,
			},
			feeEstimations: boltz.FeeEstimations{
				boltz.CurrencyLiquid: 0.1,
				boltz.CurrencyBtc:    1,
			},
			expected: nil,
			err:      require.NoError,
		},
		{
			name: "Liquid/UnknownValue",
			outputs: []*onchain.Output{
				{
					TxId:  "tx1",
					Value: 0,
				},
			},
			swap: &database.ReverseSwap{
				Pair: boltz.Pair{
					From: boltz.CurrencyBtc,
					To:   boltz.CurrencyLiquid,
				},
				InvoiceAmount:     10000,
				ServiceFeePercent: 1,
			},
			feeEstimations: boltz.FeeEstimations{
				boltz.CurrencyLiquid: 0.1,
				boltz.CurrencyBtc:    1,
			},
			expected: &onchain.Output{
				TxId:  "tx1",
				Value: 0,
			},
			err: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := nursery.chooseDirectOutput(test.swap, test.feeEstimations, test.outputs)
			test.err(t, err)
			require.Equal(t, test.expected, output)
		})
	}
}
