package rpcserver

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCheckInvoiceExpiry(t *testing.T) {
	tests := []struct {
		name          string
		currency      boltzrpc.Currency
		expiry        time.Duration
		age           time.Duration
		zeroConf      bool
		afterZeroConf bool
	}{
		{
			name:          "Long/BTC",
			expiry:        1 * time.Hour,
			currency:      boltzrpc.Currency_BTC,
			zeroConf:      false,
			afterZeroConf: false,
		},
		{
			name:          "Short/BTC",
			expiry:        90 * time.Second,
			currency:      boltzrpc.Currency_BTC,
			zeroConf:      false,
			afterZeroConf: true,
		},
		{
			name:          "Long/LBTC",
			expiry:        90 * time.Second,
			currency:      boltzrpc.Currency_LBTC,
			zeroConf:      false,
			afterZeroConf: false,
		},
		{
			name: "Short/LBTC",
			// 65s expiry - 10s buffer = 55s
			expiry:        65 * time.Second,
			currency:      boltzrpc.Currency_LBTC,
			zeroConf:      false,
			afterZeroConf: true,
		},
		{
			name:          "NoChange",
			expiry:        90 * time.Second,
			currency:      boltzrpc.Currency_LBTC,
			zeroConf:      true,
			afterZeroConf: true,
		},
		{
			name:          "Old",
			expiry:        1 * time.Hour,
			age:           55 * time.Minute,
			currency:      boltzrpc.Currency_BTC,
			zeroConf:      false,
			afterZeroConf: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &boltzrpc.CreateSwapRequest{
				ZeroConf: &tt.zeroConf,
				Pair: &boltzrpc.Pair{
					From: tt.currency,
					To:   boltzrpc.Currency_BTC,
				},
			}
			invoice, err := zpay32.NewInvoice(
				&chaincfg.RegressionNetParams,
				[32]byte{},
				time.Now().Add(-tt.age),
				zpay32.Expiry(tt.expiry),
				zpay32.Description("test"),
			)
			require.NoError(t, err)

			checkInvoiceExpiry(request, invoice)

			require.Equal(t, tt.afterZeroConf, *request.ZeroConf)
		})
	}
}