package autoswap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetConfigValue(t *testing.T) {
	cfg := DefaultConfig
	value, err := cfg.GetValue("enabled")
	require.NoError(t, err)
	require.Equal(t, false, value)
}

func TestSetConfigValue(t *testing.T) {
	cfg := DefaultConfig

	require.NoError(t, cfg.SetValue("Enabled", "true"))
	require.Equal(t, true, cfg.Enabled)

	require.NoError(t, cfg.SetValue("enabled", "false"))
	require.Equal(t, false, cfg.Enabled)

	require.Error(t, cfg.SetValue("Auto", "invalid"))

	require.NoError(t, cfg.SetValue("AutoBudget", "123"))
	require.Equal(t, 123, int(cfg.AutoBudget))

	require.NoError(t, cfg.SetValue("MaxFeePercent", "2.5"))
	require.Equal(t, 2.5, float64(cfg.MaxFeePercent))

	require.Error(t, cfg.SetValue("AutoBudget", "invalid"))

	require.Error(t, cfg.SetValue("unknown", "123"))
}
