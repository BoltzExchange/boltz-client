//go:build !unit

package wallet_test

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	onchainWallet "github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/test"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

var wallet *onchainWallet.Wallet
var credentials *onchainWallet.Credentials

func TestMain(m *testing.M) {
	var err error
	wallet, credentials, err = test.InitTestWallet(boltz.CurrencyBtc, true)
	if err != nil {
		logger.Fatal(err.Error())
	}
	os.Exit(m.Run())
}

func TestBalance(t *testing.T) {
	balance, err := wallet.GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)
}
func TestSend(t *testing.T) {
	txid, err := wallet.SendToAddress(test.BtcCli("getnewaddress"), 10000, 1)
	fmt.Println(txid)
	require.NoError(t, err)
	test.MineBlock()
}

func TestFee(t *testing.T) {
	fee, err := wallet.EstimateFee(1)
	require.NoError(t, err)
	require.NotZero(t, fee)
}

func TestBlockStream(t *testing.T) {
	blocks := make(chan *onchain.BlockEpoch)
	stop := make(chan bool)
	go func() {
		err := wallet.RegisterBlockListener(blocks, stop)
		require.NoError(t, err)
		close(blocks)
	}()
	test.MineBlock()
	block := <-blocks
	stop <- true
	_, ok := <-blocks
	require.False(t, ok)
	require.NotEqual(t, 0, block.Height)

	height, err := wallet.GetBlockHeight()
	require.NoError(t, err)
	require.Equal(t, block.Height, height)
}

func TestReal(t *testing.T) {
	subaccounts, err := wallet.GetSubaccounts(true)
	require.NoError(t, err)
	require.NotZero(t, subaccounts)

	balance, err := wallet.GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)

	_, err = wallet.SetSubaccount(nil)
	require.NoError(t, err)
	balance, err = wallet.GetBalance()
	require.NoError(t, err)
	require.Zero(t, balance.Total)
}

func TestImportSlip132Wallet(t *testing.T) {
	tests := []struct {
		name        string
		credentials onchainWallet.Credentials
	}{
		{
			"Xpub",
			onchainWallet.Credentials{
				Xpub:     "vpub5XzEwP9YWe4cJD6pB3njrMgWahQbzHhfGAyuErnswtPuzm6QdLqHH79DSZ6YW3McdE1pwxvr7wHU2nMtVbPZ1jW4tqg8ggx4ZV19U7i69pd",
				Currency: boltz.CurrencyBtc,
			},
		},
		{
			"CoreDescriptor",
			onchainWallet.Credentials{
				CoreDescriptor: "wpkh([72411c95/84'/1'/0']tpubDC2Q4xK4XH72JQHXbEJa4shGP8ScAPNVNuAWszA2wo6Qjzf4zo2ke69SshBpmJv8CKDX76QN64QPiiSJjC69hGgUtV2AgiVSzSQ6zgpZFGU/1/*)#tv66wgk5",
				Currency:       boltz.CurrencyBtc,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wallet, err := onchainWallet.Login(&tc.credentials)
			require.NoError(t, err)

			accounts, err := wallet.GetSubaccounts(false)
			require.NoError(t, err)
			_, err = wallet.SetSubaccount(&accounts[0].Pointer)
			require.NoError(t, err)

			address, err := wallet.NewAddress()
			require.NoError(t, err)
			require.NotEmpty(t, address)
		})
	}
}

func TestEncryptedCredentials(t *testing.T) {
	password := "password"
	encrypted, err := credentials.Encrypt(password)
	require.NoError(t, err)

	_, err = encrypted.Encrypt(password)
	require.Error(t, err)

	_, err = onchainWallet.Login(encrypted)
	require.Error(t, err)

	decrypted, err := encrypted.Decrypt(password)
	require.NoError(t, err)
	require.Equal(t, credentials, decrypted)

	wallet, err := onchainWallet.Login(decrypted)
	require.NoError(t, err)

	_, err = decrypted.Decrypt(password)
	require.Error(t, err)

	require.NoError(t, wallet.Remove())
}
