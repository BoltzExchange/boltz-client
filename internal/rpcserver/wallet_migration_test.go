//go:build !unit

package rpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	bitcoin_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestMigrateWalletCredentials(t *testing.T) {
	t.Run("BtcDefaultSubaccount", func(t *testing.T) {
		subaccount := uint64(1)
		expectedDescriptor, err := bitcoin_wallet.DeriveDefaultDescriptor(boltz.Regtest, test.WalletMnemonic)
		require.NoError(t, err)

		credentials := []*onchain.WalletCredentials{{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-btc",
				Currency: boltz.CurrencyBtc,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		}}

		server := &routedBoltzServer{network: boltz.Regtest}
		migrated, warnings, err := server.migrateWalletCredentials(credentials)
		require.NoError(t, err)
		require.True(t, migrated)
		require.Empty(t, warnings)
		require.False(t, credentials[0].Legacy)
		require.Nil(t, credentials[0].Subaccount)
		require.Equal(t, expectedDescriptor, credentials[0].CoreDescriptor)
	})

	t.Run("UnsupportedLegacySubaccountWarns", func(t *testing.T) {
		subaccount := uint64(0)
		credentials := []*onchain.WalletCredentials{{
			WalletInfo: onchain.WalletInfo{
				Name:     "unsupported-btc",
				Currency: boltz.CurrencyBtc,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		}}

		server := &routedBoltzServer{network: boltz.Regtest}
		migrated, warnings, err := server.migrateWalletCredentials(credentials)
		require.NoError(t, err)
		require.False(t, migrated)
		require.Len(t, warnings, 1)
		require.True(t, credentials[0].Legacy)
		require.Contains(t, warnings[0], "Please manually re-import it")

		_, err = server.loginWallet(credentials[0])
		require.Error(t, err)
		require.ErrorContains(t, err, "Please manually re-import it")
	})

	t.Run("InvalidMnemonicWarns", func(t *testing.T) {
		subaccount := uint64(1)
		credentials := []*onchain.WalletCredentials{{
			WalletInfo: onchain.WalletInfo{
				Name:     "broken-btc",
				Currency: boltz.CurrencyBtc,
			},
			Mnemonic:   "not a valid mnemonic",
			Subaccount: &subaccount,
			Legacy:     true,
		}}

		server := &routedBoltzServer{network: boltz.Regtest}
		migrated, warnings, err := server.migrateWalletCredentials(credentials)
		require.NoError(t, err)
		require.False(t, migrated)
		require.Len(t, warnings, 1)
		require.True(t, credentials[0].Legacy)
		require.Contains(t, warnings[0], "Please manually re-import it")
	})

	t.Run("LiquidDefaultSubaccount", func(t *testing.T) {
		subaccount := uint64(1)
		expectedDescriptor, err := liquid_wallet.DeriveDefaultDescriptor(boltz.Regtest, test.WalletMnemonic)
		require.NoError(t, err)

		credentials := []*onchain.WalletCredentials{{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-liquid",
				Currency: boltz.CurrencyLiquid,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		}}

		server := &routedBoltzServer{network: boltz.Regtest}
		migrated, warnings, err := server.migrateWalletCredentials(credentials)
		require.NoError(t, err)
		require.True(t, migrated)
		require.Empty(t, warnings)
		require.False(t, credentials[0].Legacy)
		require.Nil(t, credentials[0].Subaccount)
		require.Equal(t, expectedDescriptor, credentials[0].CoreDescriptor)
	})
}

func TestLegacyBtcWalletMigration(t *testing.T) {
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())

	subaccount := uint64(1)
	legacyWallet := &database.Wallet{
		WalletCredentials: &onchain.WalletCredentials{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-btc",
				Currency: boltz.CurrencyBtc,
				TenantId: database.DefaultTenantId,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		},
	}
	require.NoError(t, cfg.Database.CreateWallet(legacyWallet))
	storedLegacyWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.True(t, storedLegacyWallet.Legacy)

	chain := getOnchain(t, cfg)
	_, _, stop := setup(t, setupOptions{
		cfg:   cfg,
		chain: chain,
	})
	defer stop()

	dbWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.False(t, dbWallet.Legacy)
	require.Nil(t, dbWallet.Subaccount)
	require.NotEmpty(t, dbWallet.CoreDescriptor)

	walletImpl, err := chain.GetAnyWallet(onchain.WalletChecker{
		Id:            &legacyWallet.Id,
		Currency:      boltz.CurrencyBtc,
		AllowReadonly: true,
		TenantId:      &legacyWallet.TenantId,
	})
	require.NoError(t, err)
	_, ok := walletImpl.(*bitcoin_wallet.Wallet)
	require.True(t, ok)
}

func TestLegacyLiquidWalletMigration(t *testing.T) {
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())

	subaccount := uint64(1)
	legacyWallet := &database.Wallet{
		WalletCredentials: &onchain.WalletCredentials{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-liquid",
				Currency: boltz.CurrencyLiquid,
				TenantId: database.DefaultTenantId,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		},
	}
	require.NoError(t, cfg.Database.CreateWallet(legacyWallet))
	storedLegacyWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.True(t, storedLegacyWallet.Legacy)

	chain := getOnchain(t, cfg)
	_, _, stop := setup(t, setupOptions{
		cfg:   cfg,
		chain: chain,
	})
	defer stop()

	dbWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.False(t, dbWallet.Legacy)
	require.Nil(t, dbWallet.Subaccount)
	require.NotEmpty(t, dbWallet.CoreDescriptor)

	walletImpl, err := chain.GetAnyWallet(onchain.WalletChecker{
		Id:            &legacyWallet.Id,
		Currency:      boltz.CurrencyLiquid,
		AllowReadonly: true,
		TenantId:      &legacyWallet.TenantId,
	})
	require.NoError(t, err)
	_, ok := walletImpl.(*liquid_wallet.Wallet)
	require.True(t, ok)
}

type noopChainProvider struct{}

func (noopChainProvider) EstimateFee() (float64, error)               { return 1, nil }
func (noopChainProvider) GetBlockHeight() (uint32, error)             { return 0, nil }
func (noopChainProvider) GetRawTransaction(string) (string, error)    { return "", nil }
func (noopChainProvider) BroadcastTransaction(string) (string, error) { return "", nil }
func (noopChainProvider) IsTransactionConfirmed(string) (bool, error) { return false, nil }
func (noopChainProvider) GetUnspentOutputs(string) ([]*onchain.Output, error) {
	return nil, nil
}
func (noopChainProvider) Disconnect() {}

type testWalletBackend struct{}

func (testWalletBackend) DeriveDefaultDescriptor(mnemonic string) (string, error) {
	return "descriptor:" + mnemonic, nil
}

func (testWalletBackend) NewWallet(credentials *onchain.WalletCredentials) (onchain.Wallet, error) {
	return &testWallet{info: credentials.WalletInfo}, nil
}

type testWallet struct {
	info onchain.WalletInfo
}

func (wallet *testWallet) NewAddress() (string, error)                          { return "addr", nil }
func (wallet *testWallet) SendToAddress(onchain.WalletSendArgs) (string, error) { return "txid", nil }
func (wallet *testWallet) Ready() bool                                          { return true }
func (wallet *testWallet) GetBalance() (*onchain.Balance, error)                { return &onchain.Balance{}, nil }
func (wallet *testWallet) GetWalletInfo() onchain.WalletInfo                    { return wallet.info }
func (wallet *testWallet) Disconnect() error                                    { return nil }
func (wallet *testWallet) GetTransactions(uint64, uint64) ([]*onchain.WalletTransaction, error) {
	return nil, nil
}
func (wallet *testWallet) BumpTransactionFee(string, float64) (string, error) { return "", nil }
func (wallet *testWallet) GetSendFee(onchain.WalletSendArgs) (uint64, uint64, error) {
	return 0, 0, nil
}
func (wallet *testWallet) GetOutputs(string) ([]*onchain.Output, error) { return nil, nil }
func (wallet *testWallet) Sync() error                                  { return nil }
func (wallet *testWallet) FullScan() error                              { return nil }
func (wallet *testWallet) ApplyTransaction(string) error                { return nil }

func newTestOnchain() *onchain.Onchain {
	chain := &onchain.Onchain{
		Network: boltz.Regtest,
		Btc:     &onchain.Currency{Chain: noopChainProvider{}},
		Liquid:  &onchain.Currency{Chain: noopChainProvider{}},
		WalletSyncIntervals: map[boltz.Currency]time.Duration{
			boltz.CurrencyBtc:    time.Hour,
			boltz.CurrencyLiquid: time.Hour,
		},
	}
	chain.Init()
	return chain
}

func TestImportWalletDoesNotMigrateLegacyWallets(t *testing.T) {
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())

	subaccount := uint64(1)
	legacyWallet := &database.Wallet{
		WalletCredentials: &onchain.WalletCredentials{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-btc",
				Currency: boltz.CurrencyBtc,
				TenantId: database.DefaultTenantId,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		},
	}
	require.NoError(t, cfg.Database.CreateWallet(legacyWallet))

	chain := newTestOnchain()
	t.Cleanup(chain.Disconnect)

	server := &routedBoltzServer{
		database: cfg.Database,
		network:  boltz.Regtest,
		onchain:  chain,
		walletBackends: map[boltz.Currency]onchain.WalletBackend{
			boltz.CurrencyBtc: testWalletBackend{},
		},
	}

	mnemonic, err := onchain.GenerateMnemonic()
	require.NoError(t, err)
	err = server.importWallet(context.Background(), &onchain.WalletCredentials{
		WalletInfo: onchain.WalletInfo{
			Name:     "fresh-wallet",
			Currency: boltz.CurrencyBtc,
			TenantId: database.DefaultTenantId,
		},
		Mnemonic: mnemonic,
	}, "")
	require.NoError(t, err)

	dbWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.True(t, dbWallet.Legacy)
	require.NotNil(t, dbWallet.Subaccount)
	require.Empty(t, dbWallet.CoreDescriptor)
}

func TestVerifyWalletPasswordDoesNotMigrateLegacyWallets(t *testing.T) {
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())

	subaccount := uint64(1)
	legacyWallet := &database.Wallet{
		WalletCredentials: &onchain.WalletCredentials{
			WalletInfo: onchain.WalletInfo{
				Name:     "legacy-liquid",
				Currency: boltz.CurrencyLiquid,
				TenantId: database.DefaultTenantId,
			},
			Mnemonic:   test.WalletMnemonic,
			Subaccount: &subaccount,
			Legacy:     true,
		},
	}
	require.NoError(t, cfg.Database.CreateWallet(legacyWallet))

	server := &routedBoltzServer{database: cfg.Database}
	response, err := server.VerifyWalletPassword(context.Background(), &boltzrpc.VerifyWalletPasswordRequest{Password: ""})
	require.NoError(t, err)
	require.True(t, response.Correct)

	dbWallet, err := cfg.Database.GetWallet(legacyWallet.Id)
	require.NoError(t, err)
	require.True(t, dbWallet.Legacy)
	require.NotNil(t, dbWallet.Subaccount)
	require.Empty(t, dbWallet.CoreDescriptor)
}
