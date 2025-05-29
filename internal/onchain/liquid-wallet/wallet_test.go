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
	"github.com/stretchr/testify/require"
)

var fundedWallet *liquid_wallet.Wallet

const syncInterval = 1 * time.Second

func TestMain(m *testing.M) {
	var err error
	fundedWallet, err = test.InitTestWalletLiquid(syncInterval)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(m.Run())
}

func TestWallet_NewAddress(t *testing.T) {
	address, err := fundedWallet.NewAddress()
	require.NoError(t, err)
	require.NotEmpty(t, address)

	anotherAddress, err := fundedWallet.NewAddress()
	require.NoError(t, err)
	require.NotEmpty(t, anotherAddress)
	require.NotEqual(t, address, anotherAddress)
}

func TestWallet_GetBalance(t *testing.T) {
	balance, err := fundedWallet.GetBalance()
	require.NoError(t, err)
	require.NotNil(t, balance)
	require.Greater(t, balance.Total, uint64(0))
	require.Greater(t, balance.Confirmed, uint64(0))
}

func TestWallet_SendToAddress(t *testing.T) {
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
}

func TestWallet_SendFee(t *testing.T) {
	address := test.GetNewAddress(test.LiquidCli)
	amount, fee, err := fundedWallet.GetSendFee(onchain.WalletSendArgs{
		Address:     address,
		SatPerVbyte: 1,
		SendAll:     true,
	})
	require.NoError(t, err)

	balance, err := fundedWallet.GetBalance()
	require.NoError(t, err)
	require.Equal(t, balance.Confirmed, amount+fee)
}
