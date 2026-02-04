//go:build !unit

package rpcserver

import (
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestChainSwap(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	boltzApi := getBoltz(t, cfg)

	checkSwap := func(t *testing.T, client client.Boltz, id string) {
		response, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{})
		require.NoError(t, err)
		require.NotEmpty(t, response.ChainSwaps)
		for _, swap := range response.ChainSwaps {
			if swap.Id == id {
				fromFee := getTransactionFee(t, chain, parseCurrency(swap.Pair.From), swap.FromData.GetLockupTransactionId())
				require.NoError(t, err)
				if swap.FromData.WalletId == nil {
					fromFee = 0
				}
				toFee := getTransactionFee(t, chain, parseCurrency(swap.Pair.To), swap.ToData.GetLockupTransactionId())
				require.NoError(t, err)
				claimFee := getTransactionFee(t, chain, parseCurrency(swap.Pair.To), swap.ToData.GetTransactionId())
				require.NoError(t, err)

				require.Equal(t, int(fromFee+toFee+claimFee), int(*swap.OnchainFee))
				return
			}
		}
		require.Fail(t, "swap not returned by listswaps", id)
	}

	client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain, boltzApi: boltzApi, node: "standalone"})
	defer stop()

	tests := []struct {
		desc string
		from boltzrpc.Currency
		to   boltzrpc.Currency
	}{
		{"BTC", boltzrpc.Currency_BTC, boltzrpc.Currency_LBTC},
		{"Liquid", boltzrpc.Currency_LBTC, boltzrpc.Currency_BTC},
	}

	t.Run("Recovery", func(t *testing.T) {
		options := setupOptions{cfg: loadConfig(t), node: "standalone"}
		client, _, stop := setup(t, options)

		externalPay := true
		acceptZeroConf := true
		to := test.LiquidCli("getnewaddress")
		swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
			Amount:         &swapAmount,
			Pair:           &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC},
			ExternalPay:    &externalPay,
			ToAddress:      &to,
			AcceptZeroConf: &acceptZeroConf,
		})
		require.NoError(t, err)
		stop()

		test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, swap.FromData.Amount)
		test.MineBlock()

		client, _, stop = setup(t, options)
		defer stop()

		stream, _ := swapStream(t, client, "")
		update := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
		require.Equal(t, swap.Id, update.Id)
		checkTxOutAddress(t, chain, boltz.CurrencyLiquid, update.ToData.GetTransactionId(), update.ToData.GetAddress(), true)
	})

	t.Run("Quote", func(t *testing.T) {
		externalPay := true
		acceptZeroConf := true
		to := test.LiquidCli("getnewaddress")
		pair := &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC}
		pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, pair)
		require.NoError(t, err)
		t.Run("WithinLimits", func(t *testing.T) {
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Amount:         &swapAmount,
				Pair:           pair,
				ExternalPay:    &externalPay,
				ToAddress:      &to,
				AcceptZeroConf: &acceptZeroConf,
			})
			require.NoError(t, err)

			newAmount := swap.FromData.Amount - 10

			stream, statusStream := swapStream(t, client, swap.Id)

			test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, newAmount)
			test.MineBlock()

			info := statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionLockupFailed)
			require.Equal(t, newAmount, info.ChainSwap.FromData.Amount)

			update := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
			require.Equal(t, swap.Id, update.Id)
			checkTxOutAddress(t, chain, boltz.CurrencyLiquid, update.ToData.GetTransactionId(), update.ToData.GetAddress(), true)
		})

		t.Run("OutofLimits", func(t *testing.T) {
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Amount:         &swapAmount,
				Pair:           pair,
				ExternalPay:    &externalPay,
				ToAddress:      &to,
				AcceptZeroConf: &acceptZeroConf,
			})
			require.NoError(t, err)

			test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, pairInfo.Limits.Minimal-10)
			test.MineBlock()

			_, statusStream := swapStream(t, client, swap.Id)
			statusStream(boltzrpc.SwapState_SERVER_ERROR, boltz.TransactionLockupFailed)
		})

		t.Run("Amountless", func(t *testing.T) {
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Pair:           pair,
				ExternalPay:    &externalPay,
				ToAddress:      &to,
				AcceptZeroConf: &acceptZeroConf,
			})
			require.NoError(t, err)

			stream, _ := swapStream(t, client, swap.Id)

			amount := pairInfo.Limits.Minimal
			test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, amount)
			test.MineBlock()

			update := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
			require.Equal(t, amount, update.FromData.Amount)
			require.Equal(t, swap.Id, update.Id)
			checkTxOutAddress(t, chain, boltz.CurrencyLiquid, update.ToData.GetTransactionId(), update.ToData.GetAddress(), true)
		})

		t.Run("InvalidServiceFee", func(t *testing.T) {
			pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, pair)
			require.NoError(t, err)

			pairInfo.Fees.Percentage = 0
			amount := uint64(10000000)

			_, err = client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Pair:           pair,
				ExternalPay:    &externalPay,
				ToAddress:      &to,
				AcceptZeroConf: &acceptZeroConf,
				AcceptedPair:   pairInfo,
				Amount:         &amount,
			})
			require.Error(t, err)
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		originalTx := chain.Btc.Chain
		t.Cleanup(func() {
			chain.Btc.Chain = originalTx
		})
		toWallet := fundedWallet(t, client, boltzrpc.Currency_BTC)

		tests := []struct {
			desc     string
			txMocker chainMocker
			error    string
		}{
			{"LessValue", lessValueChainProvider, "locked up less"},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				chain.Btc.Chain = tc.txMocker(t, originalTx)

				externalPay := true
				swap, err := client.CreateChainSwap(
					&boltzrpc.CreateChainSwapRequest{
						Amount:      &swapAmount,
						ExternalPay: &externalPay,
						ToWalletId:  &toWallet.Id,
						Pair: &boltzrpc.Pair{
							From: boltzrpc.Currency_LBTC,
							To:   boltzrpc.Currency_BTC,
						},
					},
				)
				require.NoError(t, err)

				test.SendToAddress(test.LiquidCli, swap.FromData.LockupAddress, swap.FromData.Amount)
				test.MineBlock()

				stream, statusStream := swapStream(t, client, swap.Id)
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)
				test.MineBlock()
				info := stream(boltzrpc.SwapState_ERROR)
				require.Contains(t, info.ChainSwap.Error, tc.error)
			})
		}
	})

	t.Run("Retry", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		chain.Btc.Chain = flakyChainProvider(t, chain.Btc.Chain)

		acceptZeroConf := false
		externalPay := true
		toWallet := walletId(t, client, boltzrpc.Currency_BTC)
		request := &boltzrpc.CreateChainSwapRequest{
			Amount:         &swapAmount,
			Pair:           &boltzrpc.Pair{From: boltzrpc.Currency_LBTC, To: boltzrpc.Currency_BTC},
			AcceptZeroConf: &acceptZeroConf,
			ExternalPay:    &externalPay,
			ToWalletId:     &toWallet,
		}

		swap, err := client.CreateChainSwap(request)
		require.NoError(t, err)

		test.SendToAddress(test.LiquidCli, swap.FromData.LockupAddress, swap.FromData.Amount)
		test.MineBlock()

		_, statusStream := swapStream(t, client, swap.Id)
		statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)
		test.MineBlock()
		// on first call, the broadcast will fail
		statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionServerConfirmed)
		test.MineBlock()
		// new block triggers a retry, on which the broadcast will succeed
		statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.TransactionClaimed)
		checkSwap(t, client, swap.Id)
	})

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			pair := &boltzrpc.Pair{
				From: tc.from,
				To:   tc.to,
			}
			fromCli := getCli(tc.from)
			toCli := getCli(tc.to)

			refundAddress := fromCli("getnewaddress")
			toAddress := toCli("getnewaddress")

			fromWallet := fundedWallet(t, client, tc.from)
			toWallet := fundedWallet(t, client, tc.to)

			t.Run("InternalWallets", func(t *testing.T) {
				t.Run("EnoughBalance", func(t *testing.T) {
					zeroConf := true
					swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Amount:         &swapAmount,
						Pair:           pair,
						ToWalletId:     &toWallet.Id,
						FromWalletId:   &fromWallet.Id,
						AcceptZeroConf: &zeroConf,
					})
					require.NoError(t, err)
					require.NotEmpty(t, swap.Id)

					stream, _ := swapStream(t, client, swap.Id)
					test.MineBlock()
					stream(boltzrpc.SwapState_SUCCESSFUL)

					checkSwap(t, client, swap.Id)
				})

				t.Run("NoBalance", func(t *testing.T) {
					emptyWallet := emptyWallet(t, client, tc.from)
					_, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Amount:       &swapAmount,
						Pair:         pair,
						ToWalletId:   &toWallet.Id,
						FromWalletId: &emptyWallet.Id,
					})
					require.ErrorContains(t, err, "insufficient balance")
				})
			})

			t.Run("External", func(t *testing.T) {
				externalPay := true
				swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
					Amount:      &swapAmount,
					Pair:        pair,
					ExternalPay: &externalPay,
					ToAddress:   &toAddress,
				})
				require.NoError(t, err)
				require.NotEmpty(t, swap.Id)
				require.NotEmpty(t, swap.ToData.Address)

				status, streamStatus := swapStream(t, client, swap.Id)
				test.SendToAddress(fromCli, swap.FromData.LockupAddress, swap.FromData.Amount)
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
				test.MineBlock()
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionConfirmed)
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)
				test.MineBlock()
				info := status(boltzrpc.SwapState_SUCCESSFUL).ChainSwap

				to := parseCurrency(tc.to)
				checkTxOutAddress(t, chain, to, info.ToData.GetTransactionId(), info.ToData.GetAddress(), true)

				checkSwap(t, client, swap.Id)
			})

			t.Run("Refund", func(t *testing.T) {
				chainPair, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, pair)

				require.NoError(t, err)

				createFailed := func(t *testing.T, refundAddress string) (streamFunc, streamStatusFunc) {
					amount := chainPair.Limits.Minimal
					externalPay := true
					swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Pair:          pair,
						RefundAddress: &refundAddress,
						ToAddress:     &toAddress,
						Amount:        &amount,
						ExternalPay:   &externalPay,
					})
					require.NoError(t, err)

					test.SendToAddress(fromCli, swap.FromData.LockupAddress, amount-100)
					return swapStream(t, client, swap.Id)
				}

				t.Run("Script", func(t *testing.T) {
					boltzApi.DisablePartialSignatures = true

					stream, statusStream := createFailed(t, refundAddress)
					info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).ChainSwap
					test.MineUntil(t, fromCli, int64(info.FromData.TimeoutBlockHeight))
					info = stream(boltzrpc.SwapState_REFUNDED).ChainSwap

					from := parseCurrency(pair.From)
					refundFee := getTransactionFee(t, chain, from, info.FromData.GetTransactionId())
					require.NoError(t, err)
					require.Equal(t, refundFee, *info.OnchainFee)

					checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, false)
				})

				t.Run("Cooperative", func(t *testing.T) {
					boltzApi.DisablePartialSignatures = false

					_, statusStream := createFailed(t, refundAddress)
					info := statusStream(boltzrpc.SwapState_REFUNDED, boltz.TransactionLockupFailed).ChainSwap
					require.Zero(t, info.ServiceFee)

					from := parseCurrency(pair.From)

					refundFee := getTransactionFee(t, chain, from, info.FromData.GetTransactionId())
					require.NoError(t, err)
					require.Equal(t, refundFee, *info.OnchainFee)

					checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)
				})

				if tc.from == boltzrpc.Currency_BTC {

					t.Run("Manual", func(t *testing.T) {
						setup := func(t *testing.T) (*boltzrpc.ChainSwapInfo, streamStatusFunc) {
							_, statusStream := createFailed(t, "")
							info := statusStream(boltzrpc.SwapState_SERVER_ERROR, boltz.TransactionLockupFailed).ChainSwap
							clientInfo, err := client.GetInfo()
							require.NoError(t, err)
							require.Contains(t, clientInfo.RefundableSwaps, info.Id)
							return info, statusStream
						}

						t.Run("Address", func(t *testing.T) {
							info, statusStream := setup(t)

							destination := &boltzrpc.RefundSwapRequest_Address{}
							request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination}
							t.Run("Invalid", func(t *testing.T) {
								destination.Address = "invalid"
								_, err := client.RefundSwap(request)
								requireCode(t, err, codes.InvalidArgument)

								_, err = client.RefundSwap(&boltzrpc.RefundSwapRequest{Id: "invalid"})
								requireCode(t, err, codes.NotFound)
							})

							t.Run("Valid", func(t *testing.T) {
								destination.Address = refundAddress
								_, err := client.RefundSwap(request)
								require.NoError(t, err)

								info = statusStream(boltzrpc.SwapState_REFUNDED, boltz.TransactionLockupFailed).ChainSwap
								require.Zero(t, info.ServiceFee)

								from := parseCurrency(pair.From)

								refundFee := getTransactionFee(t, chain, from, info.FromData.GetTransactionId())
								require.NoError(t, err)
								assert.Equal(t, int(refundFee), int(*info.OnchainFee))

								checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)

								test.MineBlock()

								_, err = client.RefundSwap(request)
								require.Error(t, err)
							})
						})
						t.Run("Wallet", func(t *testing.T) {
							info, statusStream := setup(t)

							destination := &boltzrpc.RefundSwapRequest_WalletId{}
							request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination}

							t.Run("Invalid", func(t *testing.T) {
								destination.WalletId = 234213412341234
								_, err := client.RefundSwap(request)
								requireCode(t, err, codes.NotFound)
							})

							t.Run("Valid", func(t *testing.T) {
								destination.WalletId = fromWallet.Id
								_, err := client.RefundSwap(request)
								require.NoError(t, err)

								info = statusStream(boltzrpc.SwapState_REFUNDED, boltz.TransactionLockupFailed).ChainSwap
								require.Zero(t, info.ServiceFee)

								test.MineBlock()

								require.Eventually(t, func() bool {
									transactions, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{Id: fromWallet.Id})
									require.NoError(t, err)
									for _, transaction := range transactions.Transactions {
										for _, txInfo := range transaction.GetInfos() {
											if txInfo.Type == boltzrpc.TransactionType_REFUND && txInfo.GetSwapId() == info.Id {
												return true
											}
										}
									}
									return false
								}, 10*time.Second, 250*time.Millisecond)

								_, err = client.RefundSwap(request)
								require.Error(t, err)
							})
						})
					})
				}
			})

			if tc.to == boltzrpc.Currency_BTC {
				t.Run("Manual", func(t *testing.T) {
					createClaimable := func(t *testing.T) (*boltzrpc.ChainSwapInfo, streamFunc, streamStatusFunc) {
						response, err := client.CreateWallet(&boltzrpc.WalletParams{
							Currency: tc.to,
							Name:     "temp",
						})
						require.NoError(t, err)
						externalPay := true
						swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
							Pair:        pair,
							ExternalPay: &externalPay,
							ToWalletId:  &response.Wallet.Id,
							Amount:      &swapAmount,
						})
						require.NoError(t, err)

						test.SendToAddress(fromCli, swap.FromData.LockupAddress, swap.FromData.Amount)
						test.MineBlock()

						_, err = client.RemoveWallet(response.Wallet.Id)
						require.NoError(t, err)

						stream, statusStream := swapStream(t, client, swap.Id)
						statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)

						// we dont accept zero conf, so the swap is not claimable yet
						info, err := client.GetInfo()
						require.NoError(t, err)
						require.NotContains(t, info.ClaimableSwaps, swap.Id)

						test.MineBlock()

						update := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionServerConfirmed)

						// we add a small sleep here to avoid the race where boltz says confirmed but the chain provider hasn't synced
						time.Sleep(100 * time.Millisecond)

						info, err = client.GetInfo()
						require.NoError(t, err)
						require.Contains(t, info.ClaimableSwaps, swap.Id)

						return update.ChainSwap, stream, statusStream
					}

					t.Run("Address", func(t *testing.T) {
						info, _, _ := createClaimable(t)

						destination := &boltzrpc.ClaimSwapsRequest_Address{}
						request := &boltzrpc.ClaimSwapsRequest{SwapIds: []string{info.Id}, Destination: destination}
						t.Run("Invalid", func(t *testing.T) {
							destination.Address = "invalid"
							_, err := client.ClaimSwaps(request)
							requireCode(t, err, codes.InvalidArgument)

							_, err = client.ClaimSwaps(&boltzrpc.ClaimSwapsRequest{SwapIds: []string{"invalid"}})
							requireCode(t, err, codes.NotFound)
						})

						t.Run("Valid", func(t *testing.T) {
							destination.Address = toAddress
							response, err := client.ClaimSwaps(request)
							require.NoError(t, err)

							checkTxOutAddress(t, chain, parseCurrency(pair.To), response.TransactionId, toAddress, true)
							checkSwap(t, client, info.Id)

							_, err = client.ClaimSwaps(request)
							requireCode(t, err, codes.NotFound)
						})
					})
					t.Run("Wallet", func(t *testing.T) {
						info, stream, _ := createClaimable(t)

						destination := &boltzrpc.ClaimSwapsRequest_WalletId{}
						request := &boltzrpc.ClaimSwapsRequest{SwapIds: []string{info.Id}, Destination: destination}

						t.Run("Invalid", func(t *testing.T) {
							destination.WalletId = 234213412341234
							_, err := client.ClaimSwaps(request)
							requireCode(t, err, codes.NotFound)
						})

						t.Run("Valid", func(t *testing.T) {
							destination.WalletId = toWallet.Id
							_, err := client.ClaimSwaps(request)
							require.NoError(t, err)

							info = stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
							checkSwap(t, client, info.Id)

							require.Eventually(t, func() bool {
								fromWallet, err := client.GetWalletById(toWallet.Id)
								require.NoError(t, err)
								return fromWallet.Balance.Unconfirmed > 0
							}, 10*time.Second, 250*time.Millisecond)

							_, err = client.ClaimSwaps(request)
							requireCode(t, err, codes.NotFound)
						})
					})
				})
			}
		})
	}
}
