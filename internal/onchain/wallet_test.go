//go:build !unit

package onchain_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
	"github.com/vulpemventures/go-elements/address"
)

const checkInterval = 1 * time.Second

func walletTest(t *testing.T, funded bool, f func(t *testing.T, wallet onchain.Wallet)) {
	test.InitLogger()
	for _, currency := range []boltz.Currency{boltz.CurrencyLiquid, boltz.CurrencyBtc} {
		t.Run(string(currency), func(t *testing.T) {
			t.Parallel()
			backend := test.WalletBackend(t, currency)
			wallet, err := backend.NewWallet(test.WalletCredentials(currency))
			require.NoError(t, err)
			if funded {
				require.NoError(t, test.FundWallet(currency, wallet))
			}
			f(t, wallet)
		})
	}
}

func TestWallet_NewAddress(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		address, err := wallet.NewAddress()
		require.NoError(t, err)
		require.NotEmpty(t, address)

		// Test that subsequent calls return different addresses
		anotherAddress, err := wallet.NewAddress()
		require.NoError(t, err)
		require.NotEmpty(t, anotherAddress)
		require.NotEqual(t, address, anotherAddress)
	})
}

func TestWallet_GetBalance(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		balance, err := wallet.GetBalance()
		require.NoError(t, err)
		require.NotNil(t, balance)
		require.GreaterOrEqual(t, balance.Total, balance.Confirmed)
		require.GreaterOrEqual(t, balance.Total, balance.Unconfirmed)
		require.Equal(t, balance.Total, balance.Confirmed+balance.Unconfirmed)

		address, err := wallet.NewAddress()
		require.NoError(t, err)

		// Get currency from wallet info to determine which CLI to use
		walletInfo := wallet.GetWalletInfo()
		test.SendToAddress(test.GetCli(walletInfo.Currency), address, 10_000_000)

		require.Eventually(t, func() bool {
			require.NoError(t, wallet.Sync())
			balance, err = wallet.GetBalance()
			require.NoError(t, err)
			return balance.Total > 0
		}, 10*time.Second, 250*time.Millisecond)
	})
}

func TestWallet_GetWalletInfo(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		info := wallet.GetWalletInfo()
		require.NotEmpty(t, info.Name)
		require.NotZero(t, info.Id)
		// Currency validation is implicit since walletTest creates wallets for both currencies
	})
}

func TestWallet_Ready(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		ready := wallet.Ready()
		require.True(t, ready)
	})
}

func TestWallet_SendToAddress(t *testing.T) {
	walletTest(t, true, func(t *testing.T, wallet onchain.Wallet) {
		walletInfo := wallet.GetWalletInfo()
		cli := test.GetCli(walletInfo.Currency)
		toAddress := test.GetNewAddress(cli)

		t.Run("Amount", func(t *testing.T) {
			amount := int64(10000)
			args := onchain.WalletSendArgs{
				Address:     toAddress,
				Amount:      uint64(amount),
				SatPerVbyte: 1,
				SendAll:     false,
			}
			txId, err := wallet.SendToAddress(args)
			require.NoError(t, err)
			require.NotEmpty(t, txId)

			test.MineBlock()

			require.Eventually(t, func() bool {
				require.NoError(t, wallet.Sync())

				transactions, err := wallet.GetTransactions(30, 0)
				require.NoError(t, err)
				require.NotNil(t, transactions)
				for _, tx := range transactions {
					if tx.Id == txId {
						searchAddress := toAddress
						if walletInfo.Currency == boltz.CurrencyLiquid {
							addressInfo, err := address.FromConfidential(toAddress)
							require.NoError(t, err)
							searchAddress = addressInfo.Address
						}
						require.NoError(t, err)
						require.True(t, slices.ContainsFunc(tx.Outputs, func(o onchain.TransactionOutput) bool {
							fmt.Println(o.Address, searchAddress)
							return o.Address == searchAddress
						}))
						require.Negative(t, tx.BalanceChange)
						return true
					}
				}
				return false
			}, 5*checkInterval, checkInterval/2)
		})

		t.Run("All", func(t *testing.T) {
			args := onchain.WalletSendArgs{
				Address:     toAddress,
				SatPerVbyte: 1,
				SendAll:     true,
			}

			txId, err := wallet.SendToAddress(args)
			require.NoError(t, err)
			require.NotEmpty(t, txId)

			_, err = wallet.SendToAddress(args)
			require.Error(t, err)

			balance, err := wallet.GetBalance()
			require.NoError(t, err)
			require.Zero(t, balance.Total)

			require.NoError(t, wallet.Sync())

			balance, err = wallet.GetBalance()
			require.NoError(t, err)
			require.Zero(t, balance.Total)

			test.MineBlock()
		})
	})
}

func TestWallet_GetSendFee(t *testing.T) {
	walletTest(t, true, func(t *testing.T, wallet onchain.Wallet) {
		walletInfo := wallet.GetWalletInfo()
		cli := test.GetCli(walletInfo.Currency)
		toAddress := test.GetNewAddress(cli)

		// Test fee calculation for specific amount
		amount, fee, err := wallet.GetSendFee(onchain.WalletSendArgs{
			Address:     toAddress,
			Amount:      10000,
			SatPerVbyte: 1,
		})
		require.NoError(t, err)
		require.Equal(t, uint64(10000), amount)
		require.Greater(t, fee, uint64(0))

		// Test fee calculation for send all
		balance, err := wallet.GetBalance()
		require.NoError(t, err)
		require.Greater(t, balance.Total, uint64(0))

		amount, fee, err = wallet.GetSendFee(onchain.WalletSendArgs{
			Address:     toAddress,
			SatPerVbyte: 1,
			SendAll:     true,
		})
		require.NoError(t, err)
		require.Equal(t, balance.Total, amount+fee)
	})
}

func TestWallet_GetTransactions(t *testing.T) {
	walletTest(t, true, func(t *testing.T, wallet onchain.Wallet) {
		transactions, err := wallet.GetTransactions(0, 0)
		require.NoError(t, err)
		require.NotNil(t, transactions)

		first := transactions[0]
		count := len(transactions)

		transactions, err = wallet.GetTransactions(uint64(count), 0)
		require.NoError(t, err)
		require.NotNil(t, transactions)
		require.Equal(t, count, len(transactions))
		require.Equal(t, first, transactions[0])

		transactions, err = wallet.GetTransactions(uint64(count), 1)
		require.NoError(t, err)
		require.NotNil(t, transactions)
		require.Equal(t, count-1, len(transactions))
		require.NotEqual(t, first, transactions[0])

		transactions, err = wallet.GetTransactions(1, 1)
		require.NoError(t, err)
		require.NotNil(t, transactions)
		require.Equal(t, 1, len(transactions))
		require.NotEqual(t, first, transactions[0])
	})
}

func TestWallet_Disconnect(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		err := wallet.Disconnect()
		require.NoError(t, err)
	})
}

func TestWallet_BumpTransactionFee(t *testing.T) {
	// Only test BTC as Liquid doesn't support RBF
	t.Run("BTC", func(t *testing.T) {
		backend := test.WalletBackend(t, boltz.CurrencyBtc)
		wallet, err := backend.NewWallet(test.WalletCredentials(boltz.CurrencyBtc))
		require.NoError(t, err)
		require.NoError(t, test.FundWallet(boltz.CurrencyBtc, wallet))

		cli := test.GetCli(boltz.CurrencyBtc)
		toAddress := test.GetNewAddress(cli)

		// Send a transaction first
		txId, err := wallet.SendToAddress(onchain.WalletSendArgs{
			Address:     toAddress,
			Amount:      1000,
			SatPerVbyte: 1,
		})
		require.NoError(t, err)
		require.NotEmpty(t, txId)

		// Wait a bit to ensure transaction is in mempool
		time.Sleep(2 * time.Second)

		// Try to bump the fee
		newTxId, err := wallet.BumpTransactionFee(txId, 3)
		require.NoError(t, err)
		require.NotEmpty(t, newTxId)
		require.NotEqual(t, txId, newTxId)

		test.MineBlock()
	})
}

func TestWallet_GetOutputs(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		t.Skip("TODO")

		// Get a new address
		address, err := wallet.NewAddress()
		require.NoError(t, err)
		require.NotEmpty(t, address)

		// Get outputs for the address
		outputs, err := wallet.GetOutputs(address)
		require.NoError(t, err)
		require.NotNil(t, outputs)
	})
}

func TestWallet_ApplyTransaction(t *testing.T) {
	walletTest(t, false, func(t *testing.T, wallet onchain.Wallet) {
		t.Skip("TODO")
		// This test would require a valid transaction hex
		// For now, we just test that the method exists and handles invalid input gracefully
		err := wallet.ApplyTransaction("invalid_hex")
		require.Error(t, err)
	})
}

func TestWallet_ImportCredentials(t *testing.T) {
	tests := []struct {
		name        string
		currency    boltz.Currency
		credentials onchain.WalletCredentials
		shouldError bool
	}{
		/*
			{
				name: "Xpub/BTC",
				credentials: onchain.WalletCredentials{
					WalletInfo: onchain.WalletInfo{
						Currency: boltz.CurrencyBtc,
					},
					Xpub: "vpub5XzEwP9YWe4cJD6pB3njrMgWahQbzHhfGAyuErnswtPuzm6QdLqHH79DSZ6YW3McdE1pwxvr7wHU2nMtVbPZ1jW4tqg8ggx4ZV19U7i69pd",
				},
				shouldError: false,
			},
		*/
		{
			name:     "CoreDescriptor/BTC",
			currency: boltz.CurrencyBtc,
			credentials: onchain.WalletCredentials{
				CoreDescriptor: "wpkh([72411c95/84'/1'/0']tpubDC2Q4xK4XH72JQHXbEJa4shGP8ScAPNVNuAWszA2wo6Qjzf4zo2ke69SshBpmJv8CKDX76QN64QPiiSJjC69hGgUtV2AgiVSzSQ6zgpZFGU/1/*)#tv66wgk5",
			},
			shouldError: false,
		},
		{
			name:     "CoreDescriptor/Liquid",
			currency: boltz.CurrencyLiquid,
			credentials: onchain.WalletCredentials{
				CoreDescriptor: "ct(slip77(099d2fa0d9e56478d00ba3044a55aa9878a2f0e1c0fd1c57962573994771f87a),elwpkh([a2e8a626/84'/1'/0']tpubDC2Q4xK4XH72HUSL1DTS5ZCyqTKGV71RSCYS46eE9ei45qPLFWEVNr1gmkSXw6NCXmnLdnCx6YPv5fFMenHBmM4UXfPXP56MwikvmPFsh2b/0/*))#60v4fm2h",
			},
			shouldError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend := test.WalletBackend(t, tc.currency)
			tc.credentials.WalletInfo = test.WalletInfo(tc.currency)
			wallet, err := backend.NewWallet(&tc.credentials)
			if tc.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Test basic functionality
				address, err := wallet.NewAddress()
				require.NoError(t, err)
				require.NotEmpty(t, address)

				// Clean up
				require.NoError(t, wallet.Disconnect())
			}
		})
	}
}

func TestWallet_EncryptedCredentials(t *testing.T) {
	password := "test_password"
	credentials := &onchain.WalletCredentials{
		WalletInfo: onchain.WalletInfo{
			Currency: boltz.CurrencyBtc,
		},
		Mnemonic: test.WalletMnemonic,
	}

	// Test encryption
	encrypted, err := credentials.Encrypt(password)
	require.NoError(t, err)
	require.True(t, encrypted.Encrypted())

	// Test that already encrypted credentials can't be encrypted again
	_, err = encrypted.Encrypt(password)
	require.Error(t, err)

	// Test decryption
	decrypted, err := encrypted.Decrypt(password)
	require.NoError(t, err)
	require.False(t, decrypted.Encrypted())
	require.Equal(t, credentials.Mnemonic, decrypted.Mnemonic)

	// Test that decrypted credentials can't be decrypted again
	_, err = decrypted.Decrypt(password)
	require.Error(t, err)
}
