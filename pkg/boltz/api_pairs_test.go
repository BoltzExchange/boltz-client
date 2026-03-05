package boltz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnknownCurrencies(t *testing.T) {
	raw := []byte(`{
		"BTC": {
			"L-BTC": { "hash": "known-1" },
			"DOGE": { "hash": "unknown-to" }
		},
		"DOGE": {
			"BTC": { "hash": "unknown-from" }
		}
	}`)

	t.Run("submarine", func(t *testing.T) {
		var pairs SubmarinePairs
		require.NoError(t, json.Unmarshal(raw, &pairs))

		require.Contains(t, pairs, CurrencyBtc)
		require.NotContains(t, pairs, Currency("DOGE"))
		require.Contains(t, pairs[CurrencyBtc], CurrencyLiquid)
		require.Equal(t, "known-1", pairs[CurrencyBtc][CurrencyLiquid].Hash)
	})

	t.Run("reverse", func(t *testing.T) {
		var pairs ReversePairs
		require.NoError(t, json.Unmarshal(raw, &pairs))

		require.Contains(t, pairs, CurrencyBtc)
		require.NotContains(t, pairs, Currency("DOGE"))
		require.Contains(t, pairs[CurrencyBtc], CurrencyLiquid)
		require.Equal(t, "known-1", pairs[CurrencyBtc][CurrencyLiquid].Hash)
	})

	t.Run("chain", func(t *testing.T) {
		var pairs ChainPairs
		require.NoError(t, json.Unmarshal(raw, &pairs))

		require.Contains(t, pairs, CurrencyBtc)
		require.NotContains(t, pairs, Currency("DOGE"))
		require.Contains(t, pairs[CurrencyBtc], CurrencyLiquid)
		require.Equal(t, "known-1", pairs[CurrencyBtc][CurrencyLiquid].Hash)
	})
}
