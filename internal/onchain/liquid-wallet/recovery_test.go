//go:build !unit

package liquid_wallet_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

func storeFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	require.NoError(t, err)

	sort.Strings(files)
	return files
}

func fileNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}

func TestWallet_RecreatesWithCorruptedLegacyStore(t *testing.T) {
	cfg := test.LiquidBackendConfig(t)
	credentials := test.WalletCredentials(boltz.CurrencyLiquid)

	backend, err := liquid_wallet.NewBackend(cfg)
	require.NoError(t, err)
	wallet := newWallet(t, backend, credentials)

	const txCount = 3
	for i := 0; i < txCount; i++ {
		address, err := wallet.NewAddress()
		require.NoError(t, err)

		txId := test.SendToAddress(test.LiquidCli, address, 1_000_000)
		txHex := test.GetRawTransaction(test.LiquidCli, txId)
		require.NoError(t, wallet.ApplyTransaction(txHex))
	}

	persisted := storeFiles(t, cfg.DataDir)
	require.Len(t, persisted, txCount)
	require.Equal(t, []string{
		"000000000000",
		"000000000001",
		"000000000002",
	}, fileNames(persisted))

	replacement, err := os.ReadFile(persisted[2])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(persisted[1], replacement, 0o600))

	recoveredBackend, err := liquid_wallet.NewBackend(cfg)
	require.NoError(t, err)
	recoveredWallet := newWallet(t, recoveredBackend, credentials)

	balance, err := recoveredWallet.GetBalance()
	require.NoError(t, err)
	require.Zero(t, balance.Total)
	require.Zero(t, balance.Confirmed)
	require.Zero(t, balance.Unconfirmed)

	transactions, err := recoveredWallet.GetTransactions(0, 0)
	require.NoError(t, err)
	require.Empty(t, transactions)

	require.Empty(t, storeFiles(t, cfg.DataDir))

	require.Eventually(t, func() bool {
		err := recoveredWallet.Sync()
		require.NoError(t, err)
		transactions, err = recoveredWallet.GetTransactions(txCount, 0)
		require.NoError(t, err)
		return len(transactions) == txCount
	}, 5*time.Second, 250*time.Millisecond)
}
