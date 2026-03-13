//go:build !unit

package rpcserver

import (
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestFundingAddress(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	boltzClient, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
	defer stop()

	fundedWallet(t, boltzClient, boltzrpc.Currency_BTC)
	fundedWallet(t, boltzClient, boltzrpc.Currency_LBTC)

	currencyPtr := func(currency boltzrpc.Currency) *boltzrpc.Currency {
		return &currency
	}

	requireFundingAddress := func(t *testing.T, boltzClient client.Boltz, currency boltzrpc.Currency) *boltzrpc.FundingAddressInfo {
		t.Helper()

		fundingAddress, err := boltzClient.CreateFundingAddress(&boltzrpc.CreateFundingAddressRequest{
			Currency: currency,
		})
		require.NoError(t, err)

		return fundingAddress
	}

	requireConfirmedFundingAddress := func(t *testing.T, boltzClient client.Boltz, currency boltzrpc.Currency, amount uint64) *boltzrpc.FundingAddressInfo {
		t.Helper()

		fundingAddress := requireFundingAddress(t, boltzClient, currency)
		stream := fundingAddressStream(t, boltzClient, fundingAddress.Id)
		test.SendToAddress(getCli(currency), fundingAddress.Address, amount)
		test.MineBlock()
		stream(boltz.FundingAddressConfirmed)

		return fundingAddress
	}

	requireRefundInputCount := func(t *testing.T, currency boltzrpc.Currency, txId string, expected int) {
		t.Helper()

		transaction, err := chain.GetTransaction(parseCurrency(currency), txId, nil, false)
		require.NoError(t, err)

		if tx, ok := transaction.(*boltz.BtcTransaction); ok {
			require.Len(t, tx.MsgTx().TxIn, expected)
			return
		}
		if tx, ok := transaction.(*boltz.LiquidTransaction); ok {
			require.Len(t, tx.Inputs, expected)
			return
		}

		require.FailNow(t, "unexpected transaction type")
	}

	currencies := []struct {
		desc     string
		currency boltzrpc.Currency
	}{
		{"BTC", boltzrpc.Currency_BTC},
		{"Liquid", boltzrpc.Currency_LBTC},
	}

	t.Run("CreateAndList", func(t *testing.T) {
		for _, tc := range currencies {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireFundingAddress(t, boltzClient, tc.currency)
				require.NotEmpty(t, fundingAddress.Id)
				require.NotEmpty(t, fundingAddress.Address)
				require.NotEmpty(t, fundingAddress.BoltzPublicKey)
				require.NotZero(t, fundingAddress.TimeoutBlockHeight)
				require.Equal(t, boltz.FundingAddressCreated.String(), fundingAddress.Status)

				list, err := boltzClient.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{})
				require.NoError(t, err)

				found := false
				for _, fa := range list.FundingAddresses {
					if fa.Id == fundingAddress.Id {
						found = true
						require.Equal(t, tc.currency, fa.Currency)
						break
					}
				}
				require.True(t, found, "created funding address should be in list")

				list, err = boltzClient.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{
					Currency: &tc.currency,
				})
				require.NoError(t, err)
				require.NotEmpty(t, list.FundingAddresses)
				for _, fa := range list.FundingAddresses {
					require.Equal(t, tc.currency, fa.Currency)
				}
			})
		}
	})

	t.Run("Restore", func(t *testing.T) {
		mnemonic, err := boltzClient.GetSwapMnemonic()
		require.NoError(t, err)
		require.NotEmpty(t, mnemonic.Mnemonic)

		fundingAddress := requireConfirmedFundingAddress(t, boltzClient, boltzrpc.Currency_BTC, swapAmount)

		_, err = cfg.Database.Exec("DELETE FROM fundingAddresses WHERE id = ?", fundingAddress.Id)
		require.NoError(t, err)

		listBefore, err := boltzClient.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{
			Currency: currencyPtr(boltzrpc.Currency_BTC),
		})
		require.NoError(t, err)
		for _, entry := range listBefore.FundingAddresses {
			require.NotEqual(t, fundingAddress.Id, entry.Id)
		}

		restoreResponse, err := boltzClient.Restore(&boltzrpc.RestoreRequest{
			Mnemonic: mnemonic.Mnemonic,
		})
		require.NoError(t, err)
		require.NotEmpty(t, restoreResponse.FundingAddresses)

		var restored *boltzrpc.FundingAddressInfo
		for _, entry := range restoreResponse.FundingAddresses {
			if entry.Id == fundingAddress.Id {
				restored = entry
				break
			}
		}
		require.NotNil(t, restored)
		require.Equal(t, fundingAddress.Currency, restored.Currency)
		require.Equal(t, fundingAddress.TimeoutBlockHeight, restored.TimeoutBlockHeight)
		require.NotEmpty(t, restored.Address)
		require.NotEmpty(t, restored.Status)

		_, err = boltzClient.Restore(&boltzrpc.RestoreRequest{
			Mnemonic: mnemonic.Mnemonic,
		})
		require.NoError(t, err)

		listAfter, err := boltzClient.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{
			Currency: currencyPtr(boltzrpc.Currency_BTC),
		})
		require.NoError(t, err)
		count := 0
		for _, entry := range listAfter.FundingAddresses {
			if entry.Id == fundingAddress.Id {
				count++
			}
		}
		require.Equal(t, 1, count)

		t.Run("Errors", func(t *testing.T) {
			_, err := boltzClient.Restore(&boltzrpc.RestoreRequest{})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)

			_, err = boltzClient.Restore(&boltzrpc.RestoreRequest{
				Mnemonic: "invalid mnemonic words",
			})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)
		})
	})

	t.Run("FundSwap", func(t *testing.T) {
		for _, tc := range currencies {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount)

				pair := &boltzrpc.Pair{From: tc.currency, To: boltzrpc.Currency_BTC}
				if tc.currency == boltzrpc.Currency_BTC {
					pair = pairBtc
				}
				quote, err := boltzClient.GetSwapQuote(&boltzrpc.GetSwapQuoteRequest{
					Type: boltzrpc.SwapType_SUBMARINE,
					Pair: pair,
					Amount: &boltzrpc.GetSwapQuoteRequest_SendAmount{
						SendAmount: swapAmount,
					},
				})
				require.NoError(t, err)

				swap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
					Amount: quote.ReceiveAmount,
					Pair:   pair,
				})
				require.NoError(t, err)

				_, err = boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
					FundingAddressId: fundingAddress.Id,
					SwapId:           swap.Id,
				})
				require.NoError(t, err)

				swapStreamFn, _ := swapStream(t, boltzClient, swap.Id)
				test.MineBlock()
				info := swapStreamFn(boltzrpc.SwapState_SUCCESSFUL)
				require.Equal(t, swap.Id, info.Swap.Id)
			})
		}

		t.Run("LockupReplacement", func(t *testing.T) {
			fundingAddress := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)
			fundingStream := fundingAddressStream(t, boltzClient, fundingAddress.Id)

			lockupTx := test.SendToAddress(test.BtcCli, fundingAddress.Address, swapAmount)
			info := fundingStream(boltz.FundingAddressMempool)
			require.Equal(t, lockupTx, info.GetLockupTransactionId())

			swap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
				FundingAddress: &fundingAddress.Id,
			})
			require.NoError(t, err)

			info = fundingStream(boltz.FundingAddressMempool)
			require.Equal(t, swap.Id, info.GetSwapId())

			newLockupTx := test.BumpFee(test.BtcCli, lockupTx)
			info = fundingStream(boltz.FundingAddressMempool)
			require.Equal(t, newLockupTx, info.GetLockupTransactionId())

			swapStreamFn, _ := swapStream(t, boltzClient, swap.Id)
			test.MineBlock()
			fundingStream(boltz.FundingAddressConfirmed)
			swapInfo := swapStreamFn(boltzrpc.SwapState_SUCCESSFUL)
			require.Equal(t, swap.Id, swapInfo.Swap.Id)
		})
	})

	t.Run("QuoteUsesCurrency", func(t *testing.T) {
		for _, tc := range currencies {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount)

				quote, err := boltzClient.GetSwapQuote(&boltzrpc.GetSwapQuoteRequest{
					Type: boltzrpc.SwapType_SUBMARINE,
					Amount: &boltzrpc.GetSwapQuoteRequest_FundingAddressId{
						FundingAddressId: fundingAddress.Id,
					},
				})
				require.NoError(t, err)
				require.Equal(t, swapAmount, quote.SendAmount)
				require.NotNil(t, quote.PairInfo)
				require.Equal(t, tc.currency, quote.PairInfo.Pair.From)
				require.Equal(t, boltzrpc.Currency_BTC, quote.PairInfo.Pair.To)
			})
		}
	})

	t.Run("CreateSwap", func(t *testing.T) {
		for _, tc := range currencies {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount+10000)

				swap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
					FundingAddress: &fundingAddress.Id,
				})
				require.NoError(t, err)

				swapStreamFn, _ := swapStream(t, boltzClient, swap.Id)
				test.MineBlock()
				info := swapStreamFn(boltzrpc.SwapState_SUCCESSFUL)
				require.Equal(t, swap.Id, info.Swap.Id)
			})
		}
	})

	t.Run("FundChainSwap", func(t *testing.T) {
		chainPairs := []struct {
			desc string
			from boltzrpc.Currency
			to   boltzrpc.Currency
		}{
			{"BTC->LBTC", boltzrpc.Currency_BTC, boltzrpc.Currency_LBTC},
			{"LBTC->BTC", boltzrpc.Currency_LBTC, boltzrpc.Currency_BTC},
		}

		for _, tc := range chainPairs {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.from, swapAmount)

				externalPay := true
				acceptZeroConf := true
				toAddress := getCli(tc.to)("getnewaddress")
				pair := &boltzrpc.Pair{From: tc.from, To: tc.to}

				chainSwap, err := boltzClient.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
					Amount:         &swapAmount,
					Pair:           pair,
					ExternalPay:    &externalPay,
					ToAddress:      &toAddress,
					AcceptZeroConf: &acceptZeroConf,
				})
				require.NoError(t, err)

				_, err = boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
					FundingAddressId: fundingAddress.Id,
					SwapId:           chainSwap.Id,
				})
				require.NoError(t, err)

				swapStreamFn, _ := swapStream(t, boltzClient, chainSwap.Id)
				test.MineBlock()
				info := swapStreamFn(boltzrpc.SwapState_SUCCESSFUL)
				require.Equal(t, chainSwap.Id, info.ChainSwap.Id)
			})
		}
	})

	t.Run("CreateChainSwap", func(t *testing.T) {
		chainPairs := []struct {
			desc string
			from boltzrpc.Currency
			to   boltzrpc.Currency
		}{
			{"BTC->LBTC", boltzrpc.Currency_BTC, boltzrpc.Currency_LBTC},
			{"LBTC->BTC", boltzrpc.Currency_LBTC, boltzrpc.Currency_BTC},
		}

		for _, tc := range chainPairs {
			t.Run(tc.desc, func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.from, swapAmount)

				acceptZeroConf := true
				toAddress := getCli(tc.to)("getnewaddress")
				chainSwap, err := boltzClient.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
					Pair:           &boltzrpc.Pair{To: tc.to},
					ToAddress:      &toAddress,
					AcceptZeroConf: &acceptZeroConf,
					FundingAddress: &fundingAddress.Id,
				})
				require.NoError(t, err)

				swapStreamFn, _ := swapStream(t, boltzClient, chainSwap.Id)
				test.MineBlock()
				info := swapStreamFn(boltzrpc.SwapState_SUCCESSFUL)
				require.Equal(t, chainSwap.Id, info.ChainSwap.Id)
			})
		}
	})

	t.Run("Errors", func(t *testing.T) {
		t.Run("CreateSwapNeedsFunds", func(t *testing.T) {
			fundingAddress := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)

			_, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
				FundingAddress: &fundingAddress.Id,
			})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)
		})

		t.Run("CreateChainSwapNeedsFunds", func(t *testing.T) {
			fundingAddress := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)

			acceptZeroConf := true
			toAddress := test.LiquidCli("getnewaddress")
			_, err := boltzClient.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Pair:           &boltzrpc.Pair{To: boltzrpc.Currency_LBTC},
				ToAddress:      &toAddress,
				AcceptZeroConf: &acceptZeroConf,
				FundingAddress: &fundingAddress.Id,
			})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)
		})

		t.Run("InvalidFundingId", func(t *testing.T) {
			swap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
				Amount: swapAmount,
				Pair:   pairBtc,
			})
			require.NoError(t, err)

			_, err = boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
				FundingAddressId: "invalid-id",
				SwapId:           swap.Id,
			})
			require.Error(t, err)
			requireCode(t, err, codes.NotFound)
		})

		t.Run("MissingFundingId", func(t *testing.T) {
			_, err := boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
				SwapId: "some-swap-id",
			})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)
		})

		t.Run("MissingSwapId", func(t *testing.T) {
			fundingAddress := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)

			_, err := boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
				FundingAddressId: fundingAddress.Id,
			})
			require.Error(t, err)
			requireCode(t, err, codes.InvalidArgument)
		})

		t.Run("CurrencyMismatch", func(t *testing.T) {
			btcFunding := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)
			lbtcFunding := requireFundingAddress(t, boltzClient, boltzrpc.Currency_LBTC)

			btcSwap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
				Amount: swapAmount,
				Pair:   pairBtc,
			})
			require.NoError(t, err)

			lbtcSwap, err := boltzClient.CreateSwap(&boltzrpc.CreateSwapRequest{
				Amount: swapAmount,
				Pair:   &boltzrpc.Pair{From: boltzrpc.Currency_LBTC, To: boltzrpc.Currency_BTC},
			})
			require.NoError(t, err)

			_, err = boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
				FundingAddressId: lbtcFunding.Id,
				SwapId:           btcSwap.Id,
			})
			require.Error(t, err)

			_, err = boltzClient.FundSwap(&boltzrpc.FundSwapRequest{
				FundingAddressId: btcFunding.Id,
				SwapId:           lbtcSwap.Id,
			})
			require.Error(t, err)
		})
	})

	t.Run("Tenants", func(t *testing.T) {
		_, write, _ := createTenant(t, boltzClient, "funding-test-tenant")
		tenant := client.NewBoltzClient(write)

		for _, tc := range currencies {
			t.Run(tc.desc, func(t *testing.T) {
				tenantFunding := requireFundingAddress(t, tenant, tc.currency)
				adminFunding := requireFundingAddress(t, boltzClient, tc.currency)

				tenantList, err := tenant.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{
					Currency: &tc.currency,
				})
				require.NoError(t, err)
				for _, fa := range tenantList.FundingAddresses {
					require.Equal(t, tenantFunding.TenantId, fa.TenantId)
					require.NotEqual(t, adminFunding.Id, fa.Id)
				}

				adminList, err := boltzClient.ListFundingAddresses(&boltzrpc.ListFundingAddressesRequest{
					Currency: &tc.currency,
				})
				require.NoError(t, err)

				foundTenant := false
				foundAdmin := false
				for _, fa := range adminList.FundingAddresses {
					if fa.Id == tenantFunding.Id {
						foundTenant = true
					}
					if fa.Id == adminFunding.Id {
						foundAdmin = true
					}
				}
				require.False(t, foundTenant, "admin should not see tenant funding addresses")
				require.True(t, foundAdmin, "admin should see its own funding address")
			})
		}

		t.Run("PermissionDenied", func(t *testing.T) {
			adminFunding := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)

			swap, err := tenant.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice: func() *string {
					node := cfg.LND
					_, err := connectLightning(nil, node)
					require.NoError(t, err)

					invoice, err := node.CreateInvoice(swapAmount, nil, 3600, "test")
					require.NoError(t, err)
					return &invoice.PaymentRequest
				}(),
			})
			require.NoError(t, err)

			_, err = tenant.FundSwap(&boltzrpc.FundSwapRequest{
				FundingAddressId: adminFunding.Id,
				SwapId:           swap.Id,
			})
			require.Error(t, err)
			requireCode(t, err, codes.PermissionDenied)
		})
	})

	t.Run("Refund", func(t *testing.T) {
		t.Run("ToAddress", func(t *testing.T) {
			for _, tc := range currencies {
				t.Run(tc.desc, func(t *testing.T) {
					fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount)

					cli := getCli(tc.currency)
					destAddress := cli("getnewaddress")
					claimResp, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
						FundingAddressId: fundingAddress.Id,
						Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
					})
					require.NoError(t, err)
					require.NotEmpty(t, claimResp.TransactionId)
					checkTxOutAddress(t, chain, parseCurrency(tc.currency), claimResp.TransactionId, destAddress, true)

					test.MineBlock()
				})
			}
		})

		t.Run("ToWallet", func(t *testing.T) {
			for _, tc := range currencies {
				t.Run(tc.desc, func(t *testing.T) {
					wallet := fundedWallet(t, boltzClient, tc.currency)
					fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount)

					claimResp, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
						FundingAddressId: fundingAddress.Id,
						Destination:      &boltzrpc.RefundFundingAddressRequest_WalletId{WalletId: wallet.Id},
					})
					require.NoError(t, err)
					require.NotEmpty(t, claimResp.TransactionId)

					test.MineBlock()
				})
			}
		})

		t.Run("MultipleUtxos", func(t *testing.T) {
			for _, tc := range currencies {
				t.Run(tc.desc, func(t *testing.T) {
					fundingAddress := requireFundingAddress(t, boltzClient, tc.currency)
					stream := fundingAddressStream(t, boltzClient, fundingAddress.Id)
					cli := getCli(tc.currency)

					test.SendToAddress(cli, fundingAddress.Address, swapAmount)
					test.SendToAddress(cli, fundingAddress.Address, swapAmount)
					test.MineBlock()
					stream(boltz.FundingAddressConfirmed)

					time.Sleep(1 * time.Second)

					destAddress := cli("getnewaddress")
					claimResp, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
						FundingAddressId: fundingAddress.Id,
						Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
					})
					require.NoError(t, err)
					require.NotEmpty(t, claimResp.TransactionId)

					checkTxOutAddress(t, chain, parseCurrency(tc.currency), claimResp.TransactionId, destAddress, true)
					requireRefundInputCount(t, tc.currency, claimResp.TransactionId, 2)

					test.MineBlock()
				})
			}
		})

		t.Run("Errors", func(t *testing.T) {
			t.Run("NotFunded", func(t *testing.T) {
				fundingAddress := requireFundingAddress(t, boltzClient, boltzrpc.Currency_BTC)

				destAddress := test.BtcCli("getnewaddress")
				_, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
					FundingAddressId: fundingAddress.Id,
					Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
				})
				require.Error(t, err)
				requireCode(t, err, codes.FailedPrecondition)
			})

			t.Run("InvalidFundingId", func(t *testing.T) {
				destAddress := test.BtcCli("getnewaddress")
				_, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
					FundingAddressId: "invalid-id",
					Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
				})
				require.Error(t, err)
				requireCode(t, err, codes.NotFound)
			})

			t.Run("MissingFundingId", func(t *testing.T) {
				destAddress := test.BtcCli("getnewaddress")
				_, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
					Destination: &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
				})
				require.Error(t, err)
				requireCode(t, err, codes.InvalidArgument)
			})

			t.Run("Destination", func(t *testing.T) {
				fundingAddress := requireConfirmedFundingAddress(t, boltzClient, boltzrpc.Currency_BTC, swapAmount)

				_, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
					FundingAddressId: fundingAddress.Id,
				})
				require.Error(t, err)
				requireCode(t, err, codes.InvalidArgument)

				invalidWalletId := uint64(999999)
				_, err = boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
					FundingAddressId: fundingAddress.Id,
					Destination:      &boltzrpc.RefundFundingAddressRequest_WalletId{WalletId: invalidWalletId},
				})
				require.Error(t, err)
				requireCode(t, err, codes.NotFound)
			})
		})

		t.Run("PermissionDenied", func(t *testing.T) {
			_, write, _ := createTenant(t, boltzClient, "refund-funding-tenant")
			tenantClient := client.NewBoltzClient(write)

			adminFunding := requireConfirmedFundingAddress(t, boltzClient, boltzrpc.Currency_BTC, swapAmount)

			destAddress := test.BtcCli("getnewaddress")
			_, err := tenantClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
				FundingAddressId: adminFunding.Id,
				Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
			})
			require.Error(t, err)
			requireCode(t, err, codes.PermissionDenied)
		})

		t.Run("Expired", func(t *testing.T) {
			for _, tc := range currencies {
				t.Run(tc.desc, func(t *testing.T) {
					fundingAddress := requireConfirmedFundingAddress(t, boltzClient, tc.currency, swapAmount)

					stream := fundingAddressStream(t, boltzClient, fundingAddress.Id)
					cli := getCli(tc.currency)
					test.MineUntil(t, test.GetCli(parseCurrency(tc.currency)), int64(fundingAddress.TimeoutBlockHeight))
					stream(boltz.FundingAddressExpired)

					destAddress := cli("getnewaddress")
					claimResp, err := boltzClient.RefundFundingAddress(&boltzrpc.RefundFundingAddressRequest{
						FundingAddressId: fundingAddress.Id,
						Destination:      &boltzrpc.RefundFundingAddressRequest_Address{Address: destAddress},
					})
					require.NoError(t, err)
					require.NotEmpty(t, claimResp.TransactionId)
					checkTxOutAddress(t, chain, parseCurrency(tc.currency), claimResp.TransactionId, destAddress, true)

					test.MineBlock()
				})
			}
		})
	})
}
