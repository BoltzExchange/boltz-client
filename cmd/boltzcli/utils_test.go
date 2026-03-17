package main

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestSanitizeSwapInfo(t *testing.T) {
	resp := &boltzrpc.GetSwapInfoResponse{
		Swap: &boltzrpc.SwapInfo{
			PrivateKey: "swap-private-key",
		},
		ReverseSwap: &boltzrpc.ReverseSwapInfo{
			PrivateKey: "reverse-private-key",
		},
		ChainSwap: &boltzrpc.ChainSwapInfo{
			FromData: &boltzrpc.ChainSwapData{
				PrivateKey: "from-private-key",
			},
			ToData: &boltzrpc.ChainSwapData{
				PrivateKey: "to-private-key",
			},
		},
	}

	sanitized := sanitizeSwapInfo(resp, true)
	require.Same(t, resp, sanitized)

	sanitized = sanitizeSwapInfo(resp, false)

	require.Equal(t, RedactedValue, sanitized.Swap.PrivateKey)
	require.Equal(t, RedactedValue, sanitized.ReverseSwap.PrivateKey)
	require.Equal(t, RedactedValue, sanitized.ChainSwap.FromData.PrivateKey)
	require.Equal(t, RedactedValue, sanitized.ChainSwap.ToData.PrivateKey)
}
