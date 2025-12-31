package utils

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateSwapQuote(t *testing.T) {
	fees := &boltzrpc.SwapFees{
		Percentage: 0.5,
		MinerFees:  1000,
	}

	t.Run("submarine swap from send amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.NormalSwap, 100000, 0, fees)

		assert.Equal(t, uint64(100000), quote.SendAmount)
		assert.Equal(t, uint64(98507), quote.ReceiveAmount)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
		assert.Equal(t, uint64(493), quote.BoltzFee)
	})

	t.Run("submarine swap from receive amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.NormalSwap, 0, 100000, fees)

		assert.Equal(t, uint64(100000), quote.ReceiveAmount)
		assert.Equal(t, uint64(101500), quote.SendAmount)
		assert.Equal(t, uint64(500), quote.BoltzFee)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
	})

	t.Run("reverse swap from send amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.ReverseSwap, 100000, 0, fees)

		assert.Equal(t, uint64(100000), quote.SendAmount)
		assert.Equal(t, uint64(98500), quote.ReceiveAmount)
		assert.Equal(t, uint64(500), quote.BoltzFee)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
	})

	t.Run("reverse swap from receive amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.ReverseSwap, 0, 100000, fees)

		assert.Equal(t, uint64(100000), quote.ReceiveAmount)
		assert.Equal(t, uint64(101508), quote.SendAmount)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
		assert.Equal(t, uint64(508), quote.BoltzFee)
	})

	t.Run("chain swap from send amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.ChainSwap, 100000, 0, fees)

		assert.Equal(t, uint64(100000), quote.SendAmount)
		assert.Equal(t, uint64(98500), quote.ReceiveAmount)
		assert.Equal(t, uint64(500), quote.BoltzFee)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
	})

	t.Run("chain swap from receive amount", func(t *testing.T) {
		quote := CalculateSwapQuote(boltz.ChainSwap, 0, 100000, fees)

		assert.Equal(t, uint64(100000), quote.ReceiveAmount)
		assert.Equal(t, uint64(101508), quote.SendAmount)
		assert.Equal(t, uint64(508), quote.BoltzFee)
		assert.Equal(t, uint64(1000), quote.NetworkFee)
	})

	t.Run("submarine round trip", func(t *testing.T) {
		receiveAmount := uint64(100000)
		quote1 := CalculateSwapQuote(boltz.NormalSwap, 0, receiveAmount, fees)

		quote2 := CalculateSwapQuote(boltz.NormalSwap, quote1.SendAmount, 0, fees)

		// Should get the same receive amount (within rounding)
		require.InDelta(t, receiveAmount, quote2.ReceiveAmount, 1)
	})

	t.Run("reverse round trip", func(t *testing.T) {
		sendAmount := uint64(100000)
		quote1 := CalculateSwapQuote(boltz.ReverseSwap, sendAmount, 0, fees)

		quote2 := CalculateSwapQuote(boltz.ReverseSwap, 0, quote1.ReceiveAmount, fees)

		// Should get the same send amount (within rounding due to ceiling)
		require.InDelta(t, sendAmount, quote2.SendAmount, 1)
	})
}
