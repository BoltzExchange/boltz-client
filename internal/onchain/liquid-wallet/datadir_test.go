package liquid_wallet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareWalletDataDir(t *testing.T) {
	backend := &BlockchainBackend{
		cfg: Config{
			DataDir: t.TempDir(),
		},
	}

	current, legacy, err := backend.prepareWalletDataDir(7)
	require.NoError(t, err)
	require.False(t, legacy)
	require.Equal(t, walletDataDir(backend.cfg.DataDir, 7), current)
}

func TestPrepareWalletDataDirLegacyFallback(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "liquid-regtest"), 0o700))
	backend := &BlockchainBackend{
		cfg: Config{
			DataDir: root,
		},
	}

	current, legacy, err := backend.prepareWalletDataDir(3)
	require.NoError(t, err)
	require.True(t, legacy)
	require.Equal(t, root, current)
}
