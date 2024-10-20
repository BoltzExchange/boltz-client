package rpcserver

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCheckInvoiceExpiry(t *testing.T) {
	tests := []struct {
		name     string
		currency boltzrpc.Currency
		expiry   time.Duration
		age      time.Duration
		valid    bool
	}{
		{
			name:     "Long/BTC",
			expiry:   1 * time.Hour,
			currency: boltzrpc.Currency_BTC,
			valid:    true,
		},
		{
			name:     "Short/BTC",
			expiry:   90 * time.Second,
			currency: boltzrpc.Currency_BTC,
			valid:    false,
		},
		{
			name:     "Long/LBTC",
			expiry:   90 * time.Second,
			currency: boltzrpc.Currency_LBTC,
			valid:    true,
		},
		{
			name: "Short/LBTC",
			// 65s expiry - 10s buffer = 55s
			expiry:   65 * time.Second,
			currency: boltzrpc.Currency_LBTC,
			valid:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &boltzrpc.CreateSwapRequest{
				Pair: &boltzrpc.Pair{
					From: tt.currency,
					To:   boltzrpc.Currency_BTC,
				},
			}

			invoice := &lightning.DecodedInvoice{
				Expiry: time.Now().Add(-tt.age).Add(tt.expiry),
			}

			require.Equal(t, tt.valid, checkInvoiceExpiry(request, invoice))
		})
	}
}
