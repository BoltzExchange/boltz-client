package onchain_test

import (
	"errors"
	"testing"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

func TestEstimateFee(t *testing.T) {
	tests := []struct {
		name          string
		currency      boltz.Currency
		primaryFee    float64
		primaryError  error
		fallbackFee   float64
		fallbackError error
		errorFunc     require.ErrorAssertionFunc
		hasFallback   bool
		expectedFee   float64
	}{
		{
			name:         "BTC primary success above minimum",
			currency:     boltz.CurrencyBtc,
			primaryFee:   5.0,
			hasFallback:  true,
			expectedFee:  5.0,
			errorFunc:    require.NoError,
		},
		{
			name:         "BTC primary success below minimum, uses floor",
			currency:     boltz.CurrencyBtc,
			primaryFee:   1.0,
			hasFallback:  true,
			expectedFee:  onchain.FeeFloor[boltz.CurrencyBtc],
			errorFunc:    require.NoError,
		},
		{
			name:         "Liquid primary success above minimum",
			currency:     boltz.CurrencyLiquid,
			primaryFee:   0.5,
			hasFallback:  true,
			expectedFee:  0.5,
			errorFunc:    require.NoError,
		},
		{
			name:         "Liquid primary success below minimum, uses floor",
			currency:     boltz.CurrencyLiquid,
			primaryFee:   0.05,
			hasFallback:  true,
			expectedFee:  onchain.FeeFloor[boltz.CurrencyLiquid],
			errorFunc:    require.NoError,
		},
		{
			name:         "BTC primary fails, fallback succeeds",
			currency:     boltz.CurrencyBtc,
			primaryError: errors.New("primary failed"),
			fallbackFee:  3.0,
			hasFallback:  true,
			expectedFee:  3.0,
			errorFunc:    require.NoError,
		},
		{
			name:         "Liquid primary fails, fallback succeeds",
			currency:     boltz.CurrencyLiquid,
			primaryError: errors.New("primary failed"),
			fallbackFee:  0.2,
			hasFallback:  true,
			expectedFee:  0.2,
			errorFunc:    require.NoError,
		},
		{
			name:         "BTC primary fails, no fallback provider",
			currency:     boltz.CurrencyBtc,
			primaryError: errors.New("primary failed"),
			errorFunc:    require.Error,
		},
		{
			name:         "BTC primary fails, fallback also fails",
			currency:     boltz.CurrencyBtc,
			primaryError: errors.New("primary failed"),
			fallbackError: errors.New("fallback failed"),
			hasFallback:  true,
			errorFunc:    require.Error,
		},
		{
			name:         "Liquid primary fails, fallback also fails",
			currency:     boltz.CurrencyLiquid,
			primaryError: errors.New("primary failed"),
			fallbackError: errors.New("fallback failed"),
			hasFallback:  true,
			errorFunc:    require.Error,
		},
		{
			name:         "Invalid currency",
			currency:     boltz.Currency("invalid"),
			hasFallback:  true,
			errorFunc:    require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := onchainmock.NewMockBlockProvider(t)
			if tt.currency != boltz.Currency("invalid") {
				blocks.EXPECT().EstimateFee().Return(tt.primaryFee, tt.primaryError)
			}
			currency := onchain.Currency{Blocks: blocks}

			if tt.hasFallback {
				mockFallback := onchainmock.NewMockFeeProvider(t)
				if tt.primaryError != nil {
					mockFallback.EXPECT().EstimateFee().Return(tt.fallbackFee, tt.fallbackError)
				}
				currency.FeeFallback = mockFallback
			}

			onchainInstance := &onchain.Onchain{}
			switch tt.currency {
			case boltz.CurrencyBtc:
				onchainInstance.Btc = &currency
			case boltz.CurrencyLiquid:
				onchainInstance.Liquid = &currency
			}

			fee, err := onchainInstance.EstimateFee(tt.currency)
			tt.errorFunc(t, err)
			if err == nil {
				require.Equal(t, tt.expectedFee, fee)
			}
		})
	}
}