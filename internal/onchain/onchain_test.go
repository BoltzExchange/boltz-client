package onchain_test

import (
	"testing"
	"time"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

func TestEstimateFee(t *testing.T) {

	tests := []struct {
		name     string
		currency boltz.Currency
		fee      float64
		err      error
		expected float64
	}{
		{currency: boltz.CurrencyBtc, fee: 1.0, name: "BTC", expected: 2.0},
		{currency: boltz.CurrencyBtc, fee: 4.0, name: "BTC", expected: 4.0},
		{currency: boltz.CurrencyLiquid, fee: 3.0, name: "LBTC", expected: 3.0},
		{currency: boltz.CurrencyLiquid, fee: 0.0001, name: "LBTC", expected: 0.1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := onchainmock.NewMockChainProvider(t)
			mock.EXPECT().EstimateFee().RunAndReturn(func() (float64, error) {
				return tc.fee, tc.err
			}).Maybe()
			onchainInstance := &onchain.Onchain{
				Btc:    &onchain.Currency{Chain: mock},
				Liquid: &onchain.Currency{Chain: mock},
			}
			fee, err := onchainInstance.EstimateFee(tc.currency)
			require.NoError(t, err)
			require.Equal(t, tc.expected, fee)
		})
	}
}

/*
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
			name:        "BTC primary success above minimum",
			currency:    boltz.CurrencyBtc,
			primaryFee:  5.0,
			hasFallback: true,
			expectedFee: 5.0,
			errorFunc:   require.NoError,
		},
		{
			name:        "BTC primary success below minimum, uses floor",
			currency:    boltz.CurrencyBtc,
			primaryFee:  1.0,
			hasFallback: true,
			expectedFee: onchain.FeeFloor[boltz.CurrencyBtc],
			errorFunc:   require.NoError,
		},
		{
			name:        "Liquid primary success above minimum",
			currency:    boltz.CurrencyLiquid,
			primaryFee:  0.5,
			hasFallback: true,
			expectedFee: 0.5,
			errorFunc:   require.NoError,
		},
		{
			name:        "Liquid primary success below minimum, uses floor",
			currency:    boltz.CurrencyLiquid,
			primaryFee:  0.05,
			hasFallback: true,
			expectedFee: onchain.FeeFloor[boltz.CurrencyLiquid],
			errorFunc:   require.NoError,
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
			name:          "BTC primary fails, fallback also fails",
			currency:      boltz.CurrencyBtc,
			primaryError:  errors.New("primary failed"),
			fallbackError: errors.New("fallback failed"),
			hasFallback:   true,
			errorFunc:     require.Error,
		},
		{
			name:          "Liquid primary fails, fallback also fails",
			currency:      boltz.CurrencyLiquid,
			primaryError:  errors.New("primary failed"),
			fallbackError: errors.New("fallback failed"),
			hasFallback:   true,
			errorFunc:     require.Error,
		},
		{
			name:        "Invalid currency",
			currency:    boltz.Currency("invalid"),
			hasFallback: true,
			errorFunc:   require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := onchainmock.NewMockBlockProvider(t)
			if tt.currency != boltz.Currency("invalid") {
				blocks.EXPECT().EstimateFee().Return(tt.primaryFee, tt.primaryError)
			}
			currency := onchain.Currency{Chain: blocks}

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
*/

func mockBlockProvider(t *testing.T) *onchainmock.MockChainProvider {
	blockProvider := onchainmock.NewMockChainProvider(t)
	blockProvider.EXPECT().Disconnect().Return().Maybe()
	return blockProvider
}

func TestWalletSync(t *testing.T) {
	syncInterval := 100 * time.Millisecond
	setup := func(t *testing.T) *onchain.Onchain {
		onchainInstance := &onchain.Onchain{
			Btc: &onchain.Currency{
				Chain: mockBlockProvider(t),
			},
			Liquid: &onchain.Currency{
				Chain: mockBlockProvider(t),
			},
			WalletSyncIntervals: map[boltz.Currency]time.Duration{
				boltz.CurrencyBtc:    syncInterval,
				boltz.CurrencyLiquid: syncInterval,
			},
		}
		onchainInstance.Init()
		return onchainInstance
	}

	t.Run("Remove", func(t *testing.T) {
		onchainInstance := setup(t)
		wallet := onchainmock.NewMockWallet(t)
		done := make(chan struct{})
		wallet.EXPECT().Sync().RunAndReturn(func() error {
			go func() {
				onchainInstance.RemoveWallet(wallet.GetWalletInfo().Id)
				close(done)
			}()
			return nil
		}).Once()
		wallet.EXPECT().GetWalletInfo().Return(onchain.WalletInfo{Id: 1, Currency: boltz.CurrencyBtc}).Maybe()
		onchainInstance.AddWallet(wallet)

		select {
		case <-done:
		case <-time.After(3 * syncInterval):
			require.Fail(t, "timed out while waiting for remove")
		}
		require.Empty(t, onchainInstance.Wallets)

		// we sleep for a few more cycles - if the sync is called again the test will fail
		// since we only expect it to be called once above
		time.Sleep(2 * syncInterval)
	})

	t.Run("Disconnect", func(t *testing.T) {
		onchainInstance := setup(t)

		done := make(chan struct{})
		wallet := onchainmock.NewMockWallet(t)
		sync := wallet.EXPECT().Sync().RunAndReturn(func() error {
			go func() {
				onchainInstance.Disconnect()
				println("disconnect")
				close(done)
			}()
			return nil
		}).Once()
		wallet.EXPECT().Disconnect().Return(nil).NotBefore(sync).Once()
		wallet.EXPECT().GetWalletInfo().Return(onchain.WalletInfo{Id: 1, Currency: boltz.CurrencyBtc}).Maybe()
		onchainInstance.AddWallet(wallet)

		select {
		case <-done:
		case <-time.After(3 * onchainInstance.WalletSyncIntervals[boltz.CurrencyBtc]):
			require.Fail(t, "timed out while waiting for disconnect")
		}
	})

}
