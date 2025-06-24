//go:build !unit

package liquid_wallet_test

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

var backend *liquid_wallet.BlockchainBackend

const syncInterval = 1 * time.Second

func defaultConfig() liquid_wallet.Config {
	return liquid_wallet.Config{
		Network:      boltz.Regtest,
		DataDir:      "test-data",
		SyncInterval: syncInterval,
	}
}

func TestMain(m *testing.M) {
	var err error
	backend, err = liquid_wallet.NewBlockchainBackend(defaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	test.InitLogger()
	os.Exit(m.Run())
}

func TestWallet_GetBalance(t *testing.T) {
	wallet := newWallet(t, backend)
	balance, err := wallet.GetBalance()
	require.NoError(t, err)
	require.NotNil(t, balance)
	require.Equal(t, &onchain.Balance{}, balance)

	address, err := wallet.NewAddress()
	require.NoError(t, err)
	require.NotEmpty(t, address)

	amount := uint64(10000)
	test.SendToAddress(test.LiquidCli, address, amount)

	require.Eventually(t, func() bool {
		balance, err = wallet.GetBalance()
		require.NoError(t, err)
		return balance.Total == amount && balance.Unconfirmed == amount
	}, 10*syncInterval, syncInterval)

	test.MineBlock()

	require.Eventually(t, func() bool {
		balance, err = wallet.GetBalance()
		require.NoError(t, err)
		return balance.Total == amount && balance.Confirmed == amount
	}, 10*syncInterval, syncInterval)
}

func TestWallet_Funded(t *testing.T) {
	fundedWallet, err := liquid_wallet.NewWallet(backend, test.WalletCredentials(boltz.CurrencyLiquid))
	require.NoError(t, err)
	require.NoError(t, test.FundWallet(boltz.CurrencyLiquid, fundedWallet))

	t.Run("SendToAddress", func(t *testing.T) {
		address := test.GetNewAddress(test.LiquidCli)
		amount := int64(10000)
		txId, err := fundedWallet.SendToAddress(onchain.WalletSendArgs{
			Address:     address,
			Amount:      uint64(amount),
			SatPerVbyte: 1,
			SendAll:     false,
		})
		require.NoError(t, err)
		require.NotEmpty(t, txId)
		test.MineBlock()

		require.Eventually(t, func() bool {
			transactions, err := fundedWallet.GetTransactions(0, 0)
			require.NoError(t, err)
			require.NotNil(t, transactions)
			for _, tx := range transactions {
				if tx.Id == txId {
					// TODO: fix this
					//require.True(t, slices.ContainsFunc(tx.Outputs, func(o onchain.TransactionOutput) bool {
					//return o.Address == address
					//}))
					fee := int64(tx.Outputs[0].Amount)
					require.Equal(t, -amount-fee, tx.BalanceChange)
					return true
				}
			}
			return false
		}, 5*syncInterval, syncInterval/2)
	})

	t.Run("SendFee", func(t *testing.T) {
		address := test.GetNewAddress(test.LiquidCli)
		amount, fee, err := fundedWallet.GetSendFee(onchain.WalletSendArgs{
			Address:     address,
			SatPerVbyte: 1,
			SendAll:     true,
		})
		require.NoError(t, err)

		balance, err := fundedWallet.GetBalance()
		require.NoError(t, err)
		require.Equal(t, balance.Total, amount+fee)
	})
}

func newWallet(t *testing.T, backend *liquid_wallet.BlockchainBackend) *liquid_wallet.Wallet {
	mnemonic, err := liquid_wallet.GenerateMnemonic(boltz.Regtest)
	require.NoError(t, err)
	wallet, err := liquid_wallet.NewWallet(backend, &onchain.WalletCredentials{
		WalletInfo: onchain.WalletInfo{
			Currency: boltz.CurrencyLiquid,
		},
		Mnemonic: mnemonic,
	})
	require.NoError(t, err)
	return wallet
}

func TestWallet_NewAddress(t *testing.T) {
	wallet := newWallet(t, backend)
	address, err := wallet.NewAddress()
	require.NoError(t, err)
	require.NotEmpty(t, address)

	anotherAddress, err := wallet.NewAddress()
	require.NoError(t, err)
	require.NotEmpty(t, anotherAddress)
	require.NotEqual(t, address, anotherAddress)
}

func TestWallet_AutoConsolidate(t *testing.T) {
	numTxns := 3
	cfg := defaultConfig()
	cfg.ConsolidationThreshold = uint64(numTxns)
	backend, err := liquid_wallet.NewBlockchainBackend(cfg)
	require.NoError(t, err)
	wallet := newWallet(t, backend)

	amount := uint64(500)
	for i := 0; i < numTxns; i++ {
		address, err := wallet.NewAddress()
		require.NoError(t, err)
		require.NotEmpty(t, address)
		test.SendToAddress(test.LiquidCli, address, amount)
		time.Sleep(100 * time.Millisecond)
	}
	test.MineBlock()

	require.Eventually(t, func() bool {
		txes, err := wallet.GetTransactions(0, 0)
		require.NoError(t, err)
		if len(txes) == numTxns+1 {
			balance, err := wallet.GetBalance()
			require.NoError(t, err)
			for _, tx := range txes {
				// look for the consolidation tx
				if tx.BalanceChange != int64(amount) {
					total := uint64(numTxns) * amount
					change := uint64(tx.BalanceChange)
					require.Equal(t, balance.Total, total+change)
					return true
				}
			}
		}
		return false
	}, 30*syncInterval, syncInterval)
}
