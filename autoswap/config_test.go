package autoswap

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestGetConfigValue(t *testing.T) {
	cfg := NewConfig(DefaultConfig())
	value, err := cfg.GetValue("enabled")
	require.NoError(t, err)
	require.Equal(t, false, value)
}

func TestSetConfigValue(t *testing.T) {
	cfg := NewConfig(DefaultConfig())

	require.NoError(t, cfg.SetValue("Enabled", "true"))
	require.Equal(t, true, cfg.Enabled)

	require.NoError(t, cfg.SetValue("enabled", "false"))
	require.Equal(t, false, cfg.Enabled)

	require.NoError(t, cfg.SetValue("Budget", "123"))
	require.Equal(t, 123, int(cfg.Budget))

	require.NoError(t, cfg.SetValue("MaxFeePercent", "2.5"))
	require.Equal(t, 2.5, float64(cfg.MaxFeePercent))

	require.Error(t, cfg.SetValue("Budget", "invalid"))

	require.Error(t, cfg.SetValue("unknown", "123"))

	require.NoError(t, cfg.SetValue("Currency", "LBTC"))
	require.Equal(t, boltzrpc.Currency_LBTC, cfg.Currency)

	require.NoError(t, cfg.SetValue("Currency", "BTC"))
	require.Equal(t, boltzrpc.Currency_BTC, cfg.Currency)
}

func TestGetPair(t *testing.T) {
	cfg := NewConfig(DefaultConfig())

	pair := cfg.GetPair(boltz.NormalSwap)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.From)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)

	pair = cfg.GetPair(boltz.ReverseSwap)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)

	require.NoError(t, cfg.SetValue("Currency", "BTC"))

	pair = cfg.GetPair(boltz.ReverseSwap)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)
}

func TestConfigInit(t *testing.T) {
	tt := []struct {
		name string
		cfg  *SerializedConfig
		err  bool
	}{
		{"Default", DefaultConfig(), false},
		{
			name: "MissingMax",
			cfg: &SerializedConfig{
				MinBalancePercent: 25,
			},
			err: true,
		},
		{
			name: "ValidReverse",
			cfg: &SerializedConfig{
				MaxBalancePercent: 75,
				SwapType:          "reverse",
			},
			err: false,
		},
		{
			name: "MinGreaterMax/Percent",
			cfg: &SerializedConfig{
				MinBalancePercent: 75,
				MaxBalancePercent: 25,
			},
			err: true,
		},

		{
			name: "MinGreaterMax/Abs",
			cfg: &SerializedConfig{
				MinBalance: 10000,
				MaxBalance: 5000,
			},
			err: true,
		},
		{
			name: "PerChannel/SubmarineForbidden",
			cfg: &SerializedConfig{
				MinBalance: 10000,
				PerChannel: true,
				SwapType:   "submarine",
			},
			err: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig(tc.cfg)
			err := cfg.Init()
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
