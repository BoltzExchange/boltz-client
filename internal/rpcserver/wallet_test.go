//go:build !unit

package rpcserver

import (
	"errors"
	"strings"
	"testing"

	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/mock"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestWalletTransactions(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)

	client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
	t.Cleanup(stop)

	t.Run("Pagination", func(t *testing.T) {
		offset := uint64(1)
		limit := uint64(1)
		txId := "test"
		testWallet, walletInfo := newMockWallet(t, chain)
		testWallet.EXPECT().GetTransactions(limit, offset).Return([]*onchain.WalletTransaction{{Id: txId}}, nil)

		response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{
			Id:     walletInfo.Id,
			Offset: &offset,
			Limit:  &limit,
		})
		require.NoError(t, err)
		require.Len(t, response.Transactions, 1)
		require.Equal(t, txId, response.Transactions[0].Id)
	})

	t.Run("Readonly", func(t *testing.T) {
		testWallet, walletInfo := newMockWallet(t, chain)
		testWallet.EXPECT().GetTransactions(mock.Anything, mock.Anything).Return(nil, nil)
		walletInfo.Readonly = true

		response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{Id: walletInfo.Id})
		require.NoError(t, err)
		require.Empty(t, response.Transactions)
	})

	claimTx := "claim"
	refundTx := "refund"
	lockupTx := "lockup"
	fakeSwaps := test.FakeSwaps{
		ReverseSwaps: []database.ReverseSwap{
			{
				ClaimTransactionId: claimTx,
			},
		},
		Swaps: []database.Swap{
			{
				RefundTransactionId: refundTx,
			},
		},
		ChainSwaps: []database.ChainSwap{
			{
				FromData: &database.ChainSwapData{
					LockupTransactionId: lockupTx,
				},
			},
		},
	}
	fakeSwaps.Create(t, cfg.Database)

	tests := []struct {
		desc         string
		transactions []*onchain.WalletTransaction
		fakeSwaps    test.FakeSwaps
		txType       boltzrpc.TransactionType
	}{
		{
			desc: "Claim",
			transactions: []*onchain.WalletTransaction{
				{Id: claimTx},
			},
			txType: boltzrpc.TransactionType_CLAIM,
		},
		{
			desc: "Refund",
			transactions: []*onchain.WalletTransaction{
				{Id: refundTx},
			},
			txType: boltzrpc.TransactionType_REFUND,
		},
		{
			desc: "Lockup",
			transactions: []*onchain.WalletTransaction{
				{Id: lockupTx},
			},
			txType: boltzrpc.TransactionType_LOCKUP,
		},

		{
			desc: "Consolidate",
			transactions: []*onchain.WalletTransaction{
				{Id: "consolidation", IsConsolidation: true},
			},
			txType: boltzrpc.TransactionType_CONSOLIDATION,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testWallet, walletInfo := newMockWallet(t, chain)
			testWallet.EXPECT().GetTransactions(mock.Anything, mock.Anything).Return(test.transactions, nil)

			response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{Id: walletInfo.Id})
			require.NoError(t, err)
			for _, tx := range response.Transactions {
				if tx.Infos[0].Type == test.txType {
					return
				}
			}
			require.Fail(t, "swap not found")
		})
	}
}

func TestBumpTransaction(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{chain: chain, cfg: cfg})
	defer stop()

	feeRate := func(value float64) *float64 { return &value }

	someTxId := test.SendToAddress(test.BtcCli, test.GetNewAddress(test.BtcCli), 5000)
	txIdRequest := &boltzrpc.BumpTransactionRequest{
		Previous: &boltzrpc.BumpTransactionRequest_TxId{TxId: someTxId},
	}
	newTxId := "newTransaction"

	tests := []struct {
		desc    string
		setup   func(t *testing.T)
		swaps   *test.FakeSwaps
		request *boltzrpc.BumpTransactionRequest
		wantErr string
	}{
		{
			desc:    "Success",
			request: txIdRequest,
			setup: func(t *testing.T) {
				original := chain.Btc.Chain
				blockProvider := onchainmock.NewMockChainProvider(t)
				rate := float64(5)
				blockProvider.EXPECT().EstimateFee().Return(rate, nil)
				coverChainProvider(t, blockProvider, original)
				chain.Btc.Chain = blockProvider
				t.Cleanup(func() {
					chain.Btc.Chain = original
				})

				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, rate).Return(newTxId, nil)
			},
		},
		{
			desc:    "AlreadyConfirmed",
			request: txIdRequest,
			setup: func(t *testing.T) {
				original := chain.Btc.Chain
				chainProvider := onchainmock.NewMockChainProvider(t)
				chainProvider.EXPECT().IsTransactionConfirmed(someTxId).Return(true, nil)
				chainProvider.EXPECT().GetRawTransaction(someTxId).RunAndReturn(original.GetRawTransaction)
				coverChainProvider(t, chainProvider, original)
				chain.Btc.Chain = chainProvider
				t.Cleanup(func() {
					chain.Btc.Chain = original
				})
			},
			wantErr: "already confirmed",
		},
		{
			desc:    "Error",
			request: txIdRequest,
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return("", errors.New("error"))
			},
			wantErr: "error",
		},
		{
			desc:    "NotFound",
			request: txIdRequest,
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return("", errors.New("not found"))
			},
			wantErr: "does not belong to any wallet",
		},
		{
			desc:    "Readonly",
			request: txIdRequest,
			setup: func(t *testing.T) {
				_, walletInfo := newMockWallet(t, chain)
				walletInfo.Readonly = true
			},
			wantErr: "does not belong to any wallet",
		},
		{
			desc: "FeeRate/Less",
			request: &boltzrpc.BumpTransactionRequest{
				Previous:    txIdRequest.Previous,
				SatPerVbyte: feeRate(0.1),
			},
			wantErr: "fee rate has to be higher",
		},
		{
			desc: "SwapId/Success",
			request: &boltzrpc.BumpTransactionRequest{Previous: &boltzrpc.BumpTransactionRequest_SwapId{
				SwapId: "normal-swap",
			}},
			swaps: &test.FakeSwaps{
				Swaps: []database.Swap{
					{
						Id:                  "normal-swap",
						LockupTransactionId: someTxId,
					},
				},
			},
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return(newTxId, nil)
			},
		},
		{
			desc: "SwapId/NotImplemented",
			request: &boltzrpc.BumpTransactionRequest{Previous: &boltzrpc.BumpTransactionRequest_SwapId{
				SwapId: "refundable",
			}},
			swaps: &test.FakeSwaps{
				Swaps: []database.Swap{
					{
						Id:                  "refundable",
						RefundTransactionId: someTxId,
					},
				},
			},
			setup: func(t *testing.T) {
				newMockWallet(t, chain)
			},
			wantErr: "refund transactions cannot be bumped",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			if tc.swaps != nil {
				tc.swaps.Create(t, cfg.Database)
			}

			_, err := client.BumpTransaction(tc.request)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("Swaps", func(t *testing.T) {
		t.Run("Submarine", func(t *testing.T) {
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Amount: swapAmount,
			})
			require.NoError(t, err)

			_, statusStream := swapStream(t, client, swap.Id)

			lockupTx := test.SendToAddress(test.BtcCli, swap.Address, swap.ExpectedAmount)
			info := statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
			require.Equal(t, lockupTx, info.Swap.LockupTransactionId)

			newLockupTx := test.BumpFee(test.BtcCli, lockupTx)
			info = statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
			require.Equal(t, newLockupTx, info.Swap.LockupTransactionId)
		})

		t.Run("Chain", func(t *testing.T) {
			externalPay := true
			toAddress := test.GetNewAddress(test.LiquidCli)
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Amount: &swapAmount,
				Pair: &boltzrpc.Pair{
					From: boltzrpc.Currency_BTC,
					To:   boltzrpc.Currency_LBTC,
				},
				ToAddress:   &toAddress,
				ExternalPay: &externalPay,
			})
			require.NoError(t, err)

			_, statusStream := swapStream(t, client, swap.Id)
			lockupTx := test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, swap.FromData.Amount)
			info := statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool).GetChainSwap()
			require.Equal(t, lockupTx, info.FromData.GetLockupTransactionId())

			newLockupTx := test.BumpFee(test.BtcCli, lockupTx)
			info = statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool).GetChainSwap()
			require.Equal(t, newLockupTx, info.FromData.GetLockupTransactionId())
		})
	})
}

func TestWallet(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	// the main setup function already created a wallet
	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
	_, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	walletParams := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "test", Password: &password}

	response, err := client.CreateWallet(walletParams)
	require.NoError(t, err)

	credentials, err := client.GetWalletCredentials(response.Wallet.Id, &password)
	require.NoError(t, err)
	require.NotEmpty(t, credentials.Mnemonic)
	require.NotEmpty(t, credentials.CoreDescriptor)

	_, err = client.GetWallet(walletParams.Name)
	require.NoError(t, err)

	_, err = client.RemoveWallet(response.Wallet.Id)
	require.NoError(t, err)

	mnemonic := "invalid"
	_, err = client.ImportWallet(walletParams, &boltzrpc.WalletCredentials{Mnemonic: &mnemonic})
	require.Error(t, err)

	_, err = client.GetWallet(walletParams.Name)
	require.Error(t, err)

	_, err = client.ImportWallet(walletParams, credentials)
	require.NoError(t, err)

	/*
		_, err = client.GetWallet(info)
		require.Error(t, err)

	*/

	_, err = client.GetWallet(walletName(boltzrpc.Currency_LBTC))
	require.NoError(t, err)
}

func TestUnlock(t *testing.T) {
	password := "password"
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())
	walletCredentials := test.WalletCredentials(boltz.CurrencyBtc)
	encryptedCredentials, err := walletCredentials.Encrypt(password)
	require.NoError(t, err)
	encryptedWallet := &database.Wallet{
		WalletCredentials: encryptedCredentials,
	}
	err = cfg.Database.CreateWallet(encryptedWallet)
	require.NoError(t, err)

	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	_, err = client.GetInfo()
	require.Error(t, err)

	require.Error(t, client.Unlock("wrong"))
	require.NoError(t, client.Unlock(password))

	test.MineBlock()

	waitForSync(t, client)

	_, err = client.GetInfo()
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(encryptedWallet.Id, nil)
	require.Error(t, err)

	c, err := client.GetWalletCredentials(encryptedWallet.Id, &password)
	require.NoError(t, err)
	require.Equal(t, walletCredentials.Mnemonic, *c.Mnemonic)

	wrongPassword := "wrong"
	second := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "new", Password: &wrongPassword}
	_, err = client.CreateWallet(second)
	require.Error(t, err)

	second.Password = &password
	_, err = client.CreateWallet(second)
	require.NoError(t, err)
}

func TestCreateWallet(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	t.Run("WithPassword", func(t *testing.T) {
		testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
		_, err := client.GetWalletCredentials(testWallet.Id, nil)
		require.NoError(t, err)

		// after creating one with a password, the first one will be encrypted as well
		secondParams := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_BTC, Password: &password}
		secondWallet, err := client.CreateWallet(secondParams)
		require.NoError(t, err)

		_, err = client.GetWalletCredentials(testWallet.Id, nil)
		require.Error(t, err)

		_, err = client.GetWalletCredentials(testWallet.Id, &password)
		require.NoError(t, err)

		_, err = client.RemoveWallet(secondWallet.Wallet.Id)
		require.NoError(t, err)
	})

	t.Run("DuplicateName", func(t *testing.T) {
		_, err := client.CreateWallet(&boltzrpc.WalletParams{Name: walletName(boltzrpc.Currency_BTC), Currency: boltzrpc.Currency_BTC})
		require.Error(t, err)
	})

}

func TestWalletSendReceive(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{chain: chain})
	defer stop()

	t.Run("Receive", func(t *testing.T) {
		mockWallet, walletInfo := newMockWallet(t, chain)
		errWallet, errInfo := newMockWallet(t, chain)

		address := "test"
		mockWallet.EXPECT().NewAddress().Return(address, nil)
		errWallet.EXPECT().NewAddress().Return("", errors.New("error"))

		response, err := client.WalletReceive(walletInfo.Id)
		require.NoError(t, err)
		require.Equal(t, address, response.Address)

		_, err = client.WalletReceive(errInfo.Id)
		require.Error(t, err)
	})

	t.Run("SendFee", func(t *testing.T) {
		nodeWallet, err := client.GetWallet(strings.ToUpper(cfg.Node))
		require.NoError(t, err)
		testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)

		swapRelated := true
		sendAll := true
		t.Run("NodeWallet", func(t *testing.T) {
			_, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            nodeWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.Error(t, err)
		})

		t.Run("AddressRequired", func(t *testing.T) {
			swapRelated := false
			_, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            nodeWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.Error(t, err)
		})

		t.Run("Normal", func(t *testing.T) {
			response, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            testWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.NoError(t, err)
			require.NotZero(t, response.Fee)
			require.NotZero(t, response.FeeRate)
		})

		t.Run("CustomFee", func(t *testing.T) {
			satPerVbyte := 10.0
			response, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            testWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
				SatPerVbyte:   &satPerVbyte,
			})
			require.NoError(t, err)
			require.NotZero(t, response.Fee)
			require.Equal(t, satPerVbyte, response.FeeRate)
		})
	})

	t.Run("Send", func(t *testing.T) {
		satPerVbyte := 10.0
		sendAll := true

		defaultEstimate, err := chain.EstimateFee(boltz.CurrencyBtc)
		require.NoError(t, err)

		tests := []struct {
			desc    string
			request *boltzrpc.WalletSendRequest
			setup   mockWalletSetup
			result  string
			err     require.ErrorAssertionFunc
		}{
			{
				desc: "Normal",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					Amount:  1000,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: defaultEstimate,
					}).Return("txid", nil)
				},
			},
			{
				desc: "CustomFee",
				request: &boltzrpc.WalletSendRequest{
					Address:     "address",
					Amount:      1000,
					SatPerVbyte: &satPerVbyte,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: satPerVbyte,
						SendAll:     false,
					}).Return("txid", nil)
				},
			},
			{
				desc: "SendAll",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					SendAll: &sendAll,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						SatPerVbyte: defaultEstimate,
						SendAll:     sendAll,
					}).Return("txid", nil)
				},
			},
			{
				desc: "MissingAddress",
				request: &boltzrpc.WalletSendRequest{
					Amount: 1000,
				},
				err: require.Error,
			},
			{
				desc: "MissingAmount",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
				},
				err: require.Error,
			},
			{
				desc: "WalletError",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					Amount:  1000,
				},
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: defaultEstimate,
					}).Return("", errors.New("wallet error"))
				},
				err: require.Error,
			},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				mockWallet, walletInfo := newMockWallet(t, chain)
				if tc.setup != nil {
					tc.setup(mockWallet)
				}
				tc.request.Id = walletInfo.Id

				response, err := client.WalletSend(tc.request)
				tc.err(t, err)
				if err == nil {
					require.Equal(t, tc.result, response.TxId)
				}
			})
		}
	})
}

func TestImportDuplicateCredentials(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
	credentials, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	// duplicates are only allowed for different currencies
	second := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_LBTC}
	_, err = client.ImportWallet(second, credentials)
	require.Error(t, err)

	second.Currency = boltzrpc.Currency_BTC
	credentials.CoreDescriptor = nil
	_, err = client.ImportWallet(second, credentials)
	require.NoError(t, err)
}

func TestChangePassword(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
	_, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	correct, err := client.VerifyWalletPassword("")
	require.NoError(t, err)
	require.True(t, correct)

	err = client.ChangeWalletPassword("", password)
	require.NoError(t, err)

	correct, err = client.VerifyWalletPassword("")
	require.NoError(t, err)
	require.False(t, correct)

	correct, err = client.VerifyWalletPassword(password)
	require.NoError(t, err)
	require.True(t, correct)

	_, err = client.GetWalletCredentials(testWallet.Id, nil)
	require.Error(t, err)

	_, err = client.GetWalletCredentials(testWallet.Id, &password)
	require.NoError(t, err)
}

// the order of tests is important here.
// since the refund tests mine a lot of blocks at once, channel force closes can happen
// so after these are finished, other tests which do LN payments might fail

func TestLegacyWallet(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{
		cfg:   cfg,
		chain: chain,
	})

	rpcWallet := emptyWallet(t, client, boltzrpc.Currency_LBTC)

	walletImpl, err := chain.GetAnyWallet(onchain.WalletChecker{Id: &rpcWallet.Id})
	require.NoError(t, err)

	_, ok := walletImpl.(*liquid_wallet.Wallet)
	require.True(t, ok)

	stop()

	dbWallet, err := cfg.Database.GetWallet(rpcWallet.Id)
	require.NoError(t, err)
	require.False(t, dbWallet.Legacy)

	_, err = cfg.Database.Exec("UPDATE wallets SET legacy = TRUE WHERE id = ?", rpcWallet.Id)
	require.NoError(t, err)

	chain = getOnchain(t, cfg)
	_, _, stop = setup(t, setupOptions{
		cfg:   cfg,
		chain: chain,
	})
	defer stop()

	walletImpl, err = chain.GetAnyWallet(onchain.WalletChecker{Id: &rpcWallet.Id})
	require.NoError(t, err)
	_, ok = walletImpl.(*wallet.Wallet)
	require.True(t, ok)
}
