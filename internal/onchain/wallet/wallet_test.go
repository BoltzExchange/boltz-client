//go:build !unit

package wallet_test

import (
	"os"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	onchainWallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var wallets map[boltz.Currency]*onchainWallet.Wallet

func TestMain(m *testing.M) {
	var err error
	wallets, err = test.InitTestWallet(true)
	if err != nil {
		logger.Fatal(err.Error())
	}
	os.Exit(m.Run())
}

func TestWallet_BumpTransactionFee(t *testing.T) {
	wallet := getWallet(boltz.CurrencyBtc)

	notifier := onchainWallet.TransactionNotifier.Get()
	defer onchainWallet.TransactionNotifier.Remove(notifier)

	getTransaction := func(txId string) *onchain.WalletTransaction {
		test.WaitWalletNotifier(t, txId, notifier)
		txs, err := wallet.GetTransactions(0, 0)
		require.NoError(t, err)
		for _, tx := range txs {
			if tx.Id == txId {
				return tx
			}
		}
		require.Fail(t, "transaction not found")
		return nil
	}

	someAddress := test.GetNewAddress(test.BtcCli)
	amount := int64(1000)

	txId, err := wallet.SendToAddress(someAddress, uint64(amount), 1, false)
	require.NoError(t, err)
	tx := getTransaction(txId)

	newTxId, err := wallet.BumpTransactionFee(txId, 2)
	require.NoError(t, err)
	newTx := getTransaction(newTxId)

	oldFee := tx.BalanceChange + amount
	newFee := newTx.BalanceChange + amount
	require.Equal(t, 2*oldFee, newFee)
}

func TestBalance(t *testing.T) {
	balance, err := getWallet(boltz.CurrencyBtc).GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)
}

func getWallet(currency boltz.Currency) *onchainWallet.Wallet {
	return wallets[currency]
}
func TestSend(t *testing.T) {
	for _, currency := range []boltz.Currency{boltz.CurrencyBtc, boltz.CurrencyLiquid} {
		t.Run(string(currency), func(t *testing.T) {
			wallet := getWallet(currency)
			cli := test.GetCli(currency)
			addr := cli("getnewaddress")

			t.Run("Normal", func(t *testing.T) {
				txid, err := wallet.SendToAddress(addr, 10000, 1, false)
				require.NoError(t, err)
				rawTx := cli("getrawtransaction " + txid)
				tx, err := boltz.NewTxFromHex(currency, rawTx, nil)
				require.NoError(t, err)
				if btcTx, ok := tx.(*boltz.BtcTransaction); ok {
					for _, txIn := range btcTx.MsgTx().TxIn {
						require.Equalf(t, wire.MaxTxInSequenceNum-2, txIn.Sequence, "rbf should be enabled")
					}
				} else if liquidTx, ok := tx.(*boltz.LiquidTransaction); ok {
					for _, txIn := range liquidTx.Inputs {
						require.Equalf(t, wire.MaxTxInSequenceNum-1, txIn.Sequence, "rbf should be disabled")
					}
				}
				test.MineBlock()
			})

			minFeeRate := 1.0

			t.Run("SendFee", func(t *testing.T) {
				amount, fee, err := wallet.GetSendFee(addr, 0, minFeeRate, true)
				require.NoError(t, err)

				balance, err := wallet.GetBalance()
				require.NoError(t, err)
				require.Equal(t, balance.Confirmed, amount+fee)
			})
		})
	}
}

func TestReal(t *testing.T) {
	btcWallet := getWallet(boltz.CurrencyBtc)

	subaccounts, err := btcWallet.GetSubaccounts(true)
	require.NoError(t, err)
	require.NotZero(t, subaccounts)

	balance, err := btcWallet.GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)

	_, err = btcWallet.SetSubaccount(nil)
	require.NoError(t, err)
	balance, err = btcWallet.GetBalance()
	require.NoError(t, err)
	require.Zero(t, balance.Total)
}

func TestImportWallet(t *testing.T) {
	tests := []struct {
		name        string
		credentials onchainWallet.Credentials
		err         bool
	}{
		{
			name: "Xpub/Btc",
			credentials: onchainWallet.Credentials{
				WalletInfo: onchain.WalletInfo{
					Currency: boltz.CurrencyBtc,
				},
				Xpub: "vpub5XzEwP9YWe4cJD6pB3njrMgWahQbzHhfGAyuErnswtPuzm6QdLqHH79DSZ6YW3McdE1pwxvr7wHU2nMtVbPZ1jW4tqg8ggx4ZV19U7i69pd",
			},
		},
		{
			name: "Xpub/Liquid",
			credentials: onchainWallet.Credentials{
				WalletInfo: onchain.WalletInfo{
					Currency: boltz.CurrencyLiquid,
				},
				Xpub: "vpub5XzEwP9YWe4cJD6pB3njrMgWahQbzHhfGAyuErnswtPuzm6QdLqHH79DSZ6YW3McdE1pwxvr7wHU2nMtVbPZ1jW4tqg8ggx4ZV19U7i69pd",
			},
			err: true,
		},
		{
			name: "CoreDescriptor/Btc",
			credentials: onchainWallet.Credentials{
				WalletInfo: onchain.WalletInfo{
					Currency: boltz.CurrencyBtc,
				},
				CoreDescriptor: "wpkh([72411c95/84'/1'/0']tpubDC2Q4xK4XH72JQHXbEJa4shGP8ScAPNVNuAWszA2wo6Qjzf4zo2ke69SshBpmJv8CKDX76QN64QPiiSJjC69hGgUtV2AgiVSzSQ6zgpZFGU/1/*)#tv66wgk5",
			},
		},
		{
			name: "CoreDescriptor/Liquid",
			credentials: onchainWallet.Credentials{
				WalletInfo: onchain.WalletInfo{
					Currency: boltz.CurrencyLiquid,
				},
				CoreDescriptor: "ct(slip77(099d2fa0d9e56478d00ba3044a55aa9878a2f0e1c0fd1c57962573994771f87a),elwpkh([a2e8a626/84'/1'/0']tpubDC2Q4xK4XH72HUSL1DTS5ZCyqTKGV71RSCYS46eE9ei45qPLFWEVNr1gmkSXw6NCXmnLdnCx6YPv5fFMenHBmM4UXfPXP56MwikvmPFsh2b/0/*))#60v4fm2h",
			},
		},
		{
			name: "CoreDescriptor/Liquid/Multiple",
			credentials: onchainWallet.Credentials{
				WalletInfo: onchain.WalletInfo{
					Currency: boltz.CurrencyLiquid,
				},
				CoreDescriptor: "ct(slip77(28edd9ac380841b8ba1bc51e188f45f3db497eca97a81539e7ede3a1eff22049),elwpkh([48aca338/84'/1'/0']tpubDC2Q4xK4XH72GZPMueFxNKSYGJvUWgFFFmMF91ThA23DhC4GUvbQ5Krpxn1SBiTNowRujrfppf7YCqLj8i6X6ggeUPVTKQHCHTMTrJW7SMp/0/*))#vcah5hc6\n" +
					"ct(slip77(28edd9ac380841b8ba1bc51e188f45f3db497eca97a81539e7ede3a1eff22049),elwpkh([48aca338/84'/1'/0']tpubDC2Q4xK4XH72GZPMueFxNKSYGJvUWgFFFmMF91ThA23DhC4GUvbQ5Krpxn1SBiTNowRujrfppf7YCqLj8i6X6ggeUPVTKQHCHTMTrJW7SMp/1/*))#eenpvgd9",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wallet, err := onchainWallet.Login(&tc.credentials)
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				accounts, err := wallet.GetSubaccounts(false)
				require.NoError(t, err)
				_, err = wallet.SetSubaccount(&accounts[0].Pointer)
				require.NoError(t, err)

				address, err := wallet.NewAddress()
				require.NoError(t, err)
				require.NotEmpty(t, address)
			}
		})
	}
}

func TestEncryptedCredentials(t *testing.T) {
	password := "password"
	credentials := &onchainWallet.Credentials{
		WalletInfo: onchain.WalletInfo{
			Currency: boltz.CurrencyBtc,
		},
		Mnemonic: test.WalletMnemonic,
	}
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

	require.NoError(t, wallet.Disconnect())
}

func TestAutoConsolidate(t *testing.T) {
	var walletConfig = onchainWallet.Config{
		AutoConsolidateThreshold: 3,
		MaxInputs:                3,
	}
	onchainWallet.UpdateConfig(walletConfig)
	mnemonic, err := onchainWallet.GenerateMnemonic()
	require.NoError(t, err)
	credentials := &onchainWallet.Credentials{
		WalletInfo: onchain.WalletInfo{
			Currency: boltz.CurrencyLiquid,
		},
		Mnemonic: mnemonic,
	}
	wallet, err := onchainWallet.Login(credentials)
	require.NoError(t, err)
	_, err = wallet.SetSubaccount(nil)
	require.NoError(t, err)

	cli := test.LiquidCli
	numTxns := int(walletConfig.AutoConsolidateThreshold) + 1
	notifier := onchainWallet.TransactionNotifier.Get()
	defer onchainWallet.TransactionNotifier.Remove(notifier)
	amount := uint64(1000)
	for i := 0; i < numTxns; i++ {
		addr, err := wallet.NewAddress()
		require.NoError(t, err)
		test.SendToAddress(cli, addr, amount)
	}
	test.MineBlock()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case <-notifier:
		case <-timeout:
			t.Fatal("timeout")
		}
		txs, err := wallet.GetTransactions(0, 0)
		require.NoError(t, err)
		// wait for all txns to be picked up, including the consolidation txn
		if len(txs) == numTxns+1 {
			consolidated := false
			for _, tx := range txs {
				if tx.IsConsolidation {
					consolidated = true
					require.Less(t, tx.Outputs[0].Amount, amount*walletConfig.MaxInputs)
				}
			}
			require.True(t, consolidated)
			require.Len(t, txs, numTxns+1)
			break
		}
	}

}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  onchainWallet.Config
		wantErr bool
	}{
		{
			"Consolidate/Valid",
			onchainWallet.Config{
				AutoConsolidateThreshold: 100,
			},
			false,
		},
		{
			"Consolidate/Less",
			onchainWallet.Config{
				AutoConsolidateThreshold: 5,
			},
			true,
		},

		{
			"Consolidate/High",
			onchainWallet.Config{
				AutoConsolidateThreshold: 1000,
			},
			true,
		},
		{
			"Consolidate/Disabled",
			onchainWallet.Config{
				AutoConsolidateThreshold: 0,
			},
			false,
		},
		{
			"MaxInputs/High",
			onchainWallet.Config{
				MaxInputs: 1000,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
