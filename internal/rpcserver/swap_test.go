//go:build !unit

package rpcserver

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestSwapInfo(t *testing.T) {
	cfg := loadConfig(t)
	admin, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	_, write, _ := createTenant(t, admin, "test")
	tenant := client.NewBoltzClient(write)

	stream, _ := swapStream(t, admin, "")
	tenantStream, err := tenant.GetSwapInfoStream("")
	require.NoError(t, err)

	go func() {
		for {
			_, err := tenantStream.Recv()
			if err != nil {
				return
			}
			require.Fail(t, "tenant should not receive updates for admin swap")
		}
	}()

	check := func(t *testing.T, id string) {
		_, err = admin.GetSwapInfo(id)
		require.NoError(t, err)

		stream, err := admin.GetSwapInfoStream(id)
		require.NoError(t, err)
		require.NoError(t, stream.CloseSend())

		_, err = tenant.GetSwapInfo(id)
		requireCode(t, err, codes.PermissionDenied)

		stream, err = tenant.GetSwapInfoStream(id)
		require.NoError(t, err)
		_, err = stream.Recv()
		requireCode(t, err, codes.PermissionDenied)
	}
	externalPay := true

	swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{})
	require.NoError(t, err)
	info := stream(boltzrpc.SwapState_PENDING)
	require.Equal(t, swap.Id, info.Swap.Id)

	check(t, swap.Id)

	reverseSwap, err := admin.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
	require.NoError(t, err)
	info = stream(boltzrpc.SwapState_PENDING)
	require.Equal(t, reverseSwap.Id, info.ReverseSwap.Id)

	check(t, reverseSwap.Id)

	toWallet := fundedWallet(t, admin, boltzrpc.Currency_LBTC)
	chainSwap, err := admin.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
		Amount:      &swapAmount,
		Pair:        &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC},
		ExternalPay: &externalPay,
		ToWalletId:  &toWallet.Id,
	})
	require.NoError(t, err)
	info = stream(boltzrpc.SwapState_PENDING)
	require.Equal(t, chainSwap.Id, info.ChainSwap.Id)

	check(t, chainSwap.Id)

	t.Run("List", func(t *testing.T) {
		unify := true
		request := &boltzrpc.ListSwapsRequest{Unify: &unify}

		swaps, err := admin.ListSwaps(request)
		require.NoError(t, err)
		require.Len(t, swaps.AllSwaps, 3)

		limit := uint64(1)
		request.Limit = &limit
		swaps, err = admin.ListSwaps(request)
		require.NoError(t, err)
		require.Len(t, swaps.AllSwaps, int(limit))
		require.Equal(t, boltzrpc.SwapType_CHAIN, swaps.AllSwaps[0].Type)

		offset := uint64(1)
		limit = 2
		request.Offset = &offset
		swaps, err = admin.ListSwaps(request)
		require.NoError(t, err)
		require.Len(t, swaps.AllSwaps, int(limit))
		require.Equal(t, boltzrpc.SwapType_REVERSE, swaps.AllSwaps[0].Type)
		require.Equal(t, boltzrpc.SwapType_SUBMARINE, swaps.AllSwaps[1].Type)

		// offset and limit only support for unify
		unify = false
		_, err = admin.ListSwaps(request)
		requireCode(t, err, codes.InvalidArgument)

	})

	t.Run("PaymentHash", func(t *testing.T) {
		t.Run("WithoutInvoice", func(t *testing.T) {
			info, err := admin.GetSwapInfo(swap.Id)
			require.NoError(t, err)

			preimage, err := hex.DecodeString(info.Swap.Preimage)
			require.NoError(t, err)
			paymentHash := sha256.Sum256(preimage)
			now, err := admin.GetSwapInfoByPaymentHash(paymentHash[:])
			require.NoError(t, err)
			require.Equal(t, info, now)
			require.NoError(t, err)
		})
		t.Run("WithInvoice", func(t *testing.T) {
			node := cfg.LND
			_, err := connectLightning(nil, node)
			require.NoError(t, err)

			invoice, err := node.CreateInvoice(100000, nil, 3600, "test")
			require.NoError(t, err)

			swap, err = admin.CreateSwap(&boltzrpc.CreateSwapRequest{Invoice: &invoice.PaymentRequest})
			require.NoError(t, err)

			info, err := admin.GetSwapInfoByPaymentHash(invoice.PaymentHash[:])
			require.NoError(t, err)
			require.Equal(t, swap.Id, info.Swap.Id)
		})
	})
}

func TestSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)

	checkSwap := func(t *testing.T, swap *boltzrpc.SwapInfo) {
		invoice, err := zpay32.Decode(swap.Invoice, &chaincfg.RegressionNetParams)
		require.NoError(t, err)
		preimage, err := hex.DecodeString(swap.Preimage)
		require.NotEmpty(t, swap.Preimage)
		require.NoError(t, err)
		require.Equal(t, *invoice.PaymentHash, sha256.Sum256(preimage))

		excpectedFees := swap.ExpectedAmount - uint64(invoice.MilliSat.ToSatoshis())
		actualFees := int64(*swap.OnchainFee) + *swap.ServiceFee
		if swap.WalletId != nil {
			lockupFee := getTransactionFee(t, chain, parseCurrency(swap.Pair.From), swap.LockupTransactionId)
			require.NoError(t, err)

			excpectedFees += lockupFee
		}

		require.Equal(t, int64(excpectedFees), actualFees)
	}

	t.Run("Recovery", func(t *testing.T) {
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Amount: 100000,
			Pair:   pairBtc,
		})
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)
		stop()

		test.SendToAddress(test.BtcCli, swap.Address, swap.ExpectedAmount)
		test.MineBlock()

		client, _, stop = setup(t, setupOptions{cfg: cfg})
		defer stop()

		ticker := time.NewTicker(200 * time.Millisecond)
		timeout := time.After(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				info, err := client.GetSwapInfo(swap.Id)
				require.NoError(t, err)
				if info.Swap.State == boltzrpc.SwapState_SUCCESSFUL {
					checkSwap(t, info.Swap)
					return
				}
			case <-timeout:
				require.Fail(t, "timed out while waiting for swap")
			}
		}
	})

	t.Run("IgnoreMrh", func(t *testing.T) {
		client, _, stop := setup(t, setupOptions{node: "standalone"})
		defer stop()

		externalPay := true

		fundedWallet(t, client, boltzrpc.Currency_LBTC)

		reverseSwap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
			ExternalPay: &externalPay,
			Amount:      100000,
			Pair: &boltzrpc.Pair{
				From: boltzrpc.Currency_BTC,
				To:   boltzrpc.Currency_LBTC,
			},
			AcceptZeroConf: true,
		})
		require.NoError(t, err)

		ignoreMrh := true
		swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Invoice: reverseSwap.Invoice,
			Pair: &boltzrpc.Pair{
				From: boltzrpc.Currency_LBTC,
				To:   boltzrpc.Currency_BTC,
			},
			SendFromInternal: true,
			IgnoreMrh:        &ignoreMrh,
		})
		require.NoError(t, err)

		stream, _ := swapStream(t, client, swap.Id)
		stream(boltzrpc.SwapState_PENDING)
		test.MineBlock()
		stream(boltzrpc.SwapState_SUCCESSFUL)
	})

	t.Run("Invoice", func(t *testing.T) {
		cfg.Node = "Cln"
		client, _, stop := setup(t, setupOptions{cfg: cfg})
		defer stop()

		node := cfg.LND
		_, err := connectLightning(nil, node)
		require.NoError(t, err)

		t.Run("Offer", func(t *testing.T) {
			offer := test.GetBolt12Offer()
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice: &offer,
				Amount:  100000,
			})
			require.NoError(t, err)
			_, statusStream := swapStream(t, client, swap.Id)
			info := statusStream(boltzrpc.SwapState_PENDING, boltz.InvoiceSet)
			require.NotEmpty(t, info.Swap.Invoice)
		})

		t.Run("Invalid", func(t *testing.T) {
			invoice := "invalid"
			_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice: &invoice,
			})
			require.Error(t, err)
		})

		t.Run("ZeroAmount", func(t *testing.T) {
			invoice, err := node.CreateInvoice(0, nil, 0, "test")
			require.NoError(t, err)
			_, err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice: &invoice.PaymentRequest,
			})
			requireCode(t, err, codes.InvalidArgument)
			require.ErrorContains(t, err, "not supported")
		})

		t.Run("Valid", func(t *testing.T) {
			preimage, _, err := newPreimage()
			require.NoError(t, err)
			invoice, err := node.CreateInvoice(100000, preimage, 0, "test")
			require.NoError(t, err)
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice:          &invoice.PaymentRequest,
				SendFromInternal: true,
			})
			require.NoError(t, err)

			// cant  create multiple swaps with the same invoice
			_, err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice:          &invoice.PaymentRequest,
				SendFromInternal: true,
			})
			requireCode(t, err, codes.AlreadyExists)

			stream, _ := swapStream(t, client, swap.Id)
			info := stream(boltzrpc.SwapState_PENDING)
			require.Equal(t, invoice.PaymentRequest, info.Swap.Invoice)

			test.MineBlock()
			info = stream(boltzrpc.SwapState_SUCCESSFUL)
			require.Equal(t, hex.EncodeToString(preimage), info.Swap.Preimage)
		})

		t.Run("LNURL", func(t *testing.T) {
			// provided by clnurl plugin
			lnurl := "LNURL1DP68GUP69UHNZV3H9CCZUVPWXYARXVPSXQHKZURF9AKXUATJD3CQQKE2EU"
			amount := uint64(100000)

			t.Run("Invalid", func(t *testing.T) {
				lnurl := "invalid"
				_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
					Invoice: &lnurl,
					Amount:  amount,
				})
				requireCode(t, err, codes.InvalidArgument)
			})

			t.Run("AmountRequired", func(t *testing.T) {
				_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
					Invoice: &lnurl,
				})
				requireCode(t, err, codes.InvalidArgument)
			})

			t.Run("Valid", func(t *testing.T) {
				swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
					Invoice: &lnurl,
					Amount:  amount,
				})
				require.NoError(t, err)
				info, err := client.GetSwapInfo(swap.Id)
				require.NoError(t, err)
				invoice, err := zpay32.Decode(info.Swap.Invoice, &chaincfg.RegressionNetParams)
				require.NoError(t, err)
				require.Equal(t, amount, uint64(invoice.MilliSat.ToSatoshis()))
			})
		})
	})

	t.Run("Standalone", func(t *testing.T) {
		cfg := loadConfig(t)
		cfg.Standalone = true
		client, _, stop := setup(t, setupOptions{cfg: cfg})
		node := cfg.LND
		_, err := connectLightning(nil, node)
		require.NoError(t, err)
		defer stop()

		request := &boltzrpc.CreateSwapRequest{}
		_, err = client.CreateSwap(request)
		require.Error(t, err)

		invoice, err := node.CreateInvoice(100000, nil, 0, "test")
		require.NoError(t, err)
		request.Invoice = &invoice.PaymentRequest
		swap, err := client.CreateSwap(request)
		require.NoError(t, err)

		stream, _ := swapStream(t, client, swap.Id)
		test.SendToAddress(test.BtcCli, swap.Address, swap.ExpectedAmount)
		stream(boltzrpc.SwapState_PENDING)
		test.MineBlock()
		info := stream(boltzrpc.SwapState_SUCCESSFUL)
		require.Empty(t, info.Swap.WalletId)
	})

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {

			tests := []struct {
				desc string
				from boltzrpc.Currency
				cli  func(string) string
			}{
				{"Liquid", boltzrpc.Currency_LBTC, test.LiquidCli},
				{"BTC", boltzrpc.Currency_BTC, test.BtcCli},
			}

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					cfg := loadConfig(t)
					boltzApi := getBoltz(t, cfg)
					cfg.Node = "LND"
					pair := &boltzrpc.Pair{
						From: tc.from,
						To:   boltzrpc.Currency_BTC,
					}
					admin, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi})
					defer stop()
					fundedWallet(t, admin, tc.from)

					submarinePair, err := admin.GetPairInfo(boltzrpc.SwapType_SUBMARINE, pair)
					require.NoError(t, err)

					_, write, _ := createTenant(t, admin, "test")
					tenant := client.NewBoltzClient(write)

					t.Run("Minimal", func(t *testing.T) {
						swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
							Amount:           submarinePair.Limits.Minimal,
							Pair:             pair,
							SendFromInternal: true,
						})
						require.NoError(t, err)
						require.NotEmpty(t, swap.TxId)
						require.NotZero(t, swap.TimeoutHours)
						require.NotZero(t, swap.TimeoutBlockHeight)

						stream, _ := swapStream(t, admin, swap.Id)
						stream(boltzrpc.SwapState_PENDING)
						test.MineBlock()
						stream(boltzrpc.SwapState_SUCCESSFUL)
					})

					t.Run("NoBalance", func(t *testing.T) {
						emptyWallet := emptyWallet(t, admin, tc.from)
						_, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
							Amount:           100000,
							Pair:             pair,
							SendFromInternal: true,
							WalletId:         &emptyWallet.Id,
						})
						require.ErrorContains(t, err, "insufficient balance")
					})

					t.Run("AnyAmount", func(t *testing.T) {
						swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
							Pair: pair,
						})
						require.NoError(t, err)

						stream, statusStream := swapStream(t, admin, swap.Id)
						test.SendToAddress(tc.cli, swap.Address, 100000)
						statusStream(boltzrpc.SwapState_PENDING, boltz.InvoiceSet)

						test.MineBlock()
						info := stream(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

					if node == "CLN" {
						return
					}

					t.Run("Refund", func(t *testing.T) {
						cli := tc.cli

						createFailed := func(t *testing.T, refundAddress string) (streamFunc, streamStatusFunc) {
							amount := submarinePair.Limits.Minimal + 100
							swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:          pair,
								RefundAddress: &refundAddress,
								Amount:        amount + 100,
							})
							require.NoError(t, err)

							test.SendToAddress(cli, swap.Address, amount)
							return swapStream(t, admin, swap.Id)
						}

						t.Run("InvalidRefundAddress", func(t *testing.T) {
							invalidAddress := "invalid"
							_, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:          pair,
								RefundAddress: &invalidAddress,
							})
							requireCode(t, err, codes.InvalidArgument)
							require.ErrorContains(t, err, "invalid refund address")
						})

						t.Run("Script", func(t *testing.T) {
							boltzApi.DisablePartialSignatures = true
							t.Cleanup(func() {
								boltzApi.DisablePartialSignatures = false
							})

							refundAddress := cli("getnewaddress")
							withStream, _ := createFailed(t, refundAddress)

							swap := withStream(boltzrpc.SwapState_ERROR).Swap

							test.MineUntil(t, cli, int64(swap.TimeoutBlockHeight))

							swap = withStream(boltzrpc.SwapState_REFUNDED).Swap

							from := parseCurrency(pair.From)

							refundFee := getTransactionFee(t, chain, from, swap.RefundTransactionId)
							require.NoError(t, err)

							require.Equal(t, int(refundFee), int(*swap.OnchainFee))

							checkTxOutAddress(t, chain, from, swap.RefundTransactionId, refundAddress, false)
						})

						t.Run("Cooperative", func(t *testing.T) {
							refundAddress := cli("getnewaddress")
							stream, _ := createFailed(t, refundAddress)

							info := stream(boltzrpc.SwapState_REFUNDED).Swap
							require.Zero(t, info.ServiceFee)

							from := parseCurrency(pair.From)

							refundFee := getTransactionFee(t, chain, from, info.RefundTransactionId)
							require.NoError(t, err)
							require.Equal(t, int(refundFee), int(*info.OnchainFee))

							checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)
						})

						if tc.from == boltzrpc.Currency_BTC {
							t.Run("Manual", func(t *testing.T) {
								setup := func(t *testing.T) *boltzrpc.SwapInfo {
									_, statusStream := createFailed(t, "")
									info := statusStream(boltzrpc.SwapState_SERVER_ERROR, boltz.TransactionLockupFailed).Swap
									clientInfo, err := admin.GetInfo()
									require.NoError(t, err)
									require.Len(t, clientInfo.RefundableSwaps, 1)
									require.Equal(t, clientInfo.RefundableSwaps[0], info.Id)

									clientInfo, err = tenant.GetInfo()
									require.NoError(t, err)
									require.Empty(t, clientInfo.RefundableSwaps)
									return info
								}

								t.Run("Address", func(t *testing.T) {
									info := setup(t)
									refundAddress := cli("getnewaddress")
									destination := &boltzrpc.RefundSwapRequest_Address{}
									request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination}

									t.Run("Invalid", func(t *testing.T) {
										destination.Address = "invalid"
										_, err := admin.RefundSwap(request)
										requireCode(t, err, codes.InvalidArgument)

										_, err = admin.RefundSwap(&boltzrpc.RefundSwapRequest{Id: "invalid"})
										requireCode(t, err, codes.NotFound)
									})

									t.Run("Valid", func(t *testing.T) {
										destination.Address = refundAddress
										response, err := admin.RefundSwap(request)
										require.NoError(t, err)
										info := response.Swap

										from := parseCurrency(pair.From)
										refundFee := getTransactionFee(t, chain, from, info.RefundTransactionId)
										require.NoError(t, err)
										assert.Equal(t, int(refundFee), int(*info.OnchainFee))

										checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)

										test.MineBlock()

										_, err = admin.RefundSwap(request)
										require.Error(t, err)
									})
								})

								t.Run("Wallet", func(t *testing.T) {
									info := setup(t)

									destination := &boltzrpc.RefundSwapRequest_WalletId{}
									request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination}

									t.Run("Invalid", func(t *testing.T) {
										destination.WalletId = 123
										_, err := admin.RefundSwap(request)
										requireCode(t, err, codes.NotFound)
									})

									t.Run("Valid", func(t *testing.T) {
										destination.WalletId = walletId(t, admin, pair.From)

										_, err := tenant.RefundSwap(request)
										requireCode(t, err, codes.NotFound)

										response, err := admin.RefundSwap(request)
										require.NoError(t, err)
										info := response.Swap
										require.Zero(t, info.ServiceFee)

										fromWallet, err := admin.GetWalletById(destination.WalletId)
										require.NoError(t, err)
										require.NotZero(t, fromWallet.Balance.Unconfirmed)

										_, err = admin.RefundSwap(request)
										require.Error(t, err)
									})

									t.Run("MultiLockup", func(t *testing.T) {
										destination.WalletId = walletId(t, admin, pair.From)

										info := setup(t)
										nTxs := 3
										var lockups []string
										for i := 0; i < nTxs; i++ {
											lockups = append(lockups, test.SendToAddress(tc.cli, info.LockupAddress, info.ExpectedAmount))
										}
										test.MineBlock()
										time.Sleep(1 * time.Second)
										request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination}
										response, err := admin.RefundSwap(request)
										require.NoError(t, err)
										require.Equal(t, response.Swap.State, boltzrpc.SwapState_REFUNDED)

										for i := 0; i < nTxs; i++ {
											request := &boltzrpc.RefundSwapRequest{Id: info.Id, Destination: destination, LockupTransactionId: &lockups[i]}
											response, err := admin.RefundSwap(request)
											require.NoError(t, err)
											require.Equal(t, response.Swap.State, boltzrpc.SwapState_REFUNDED)
										}
									})
								})
							})
						}
					})
				})
			}

		})
	}
}

func TestSwapMnemonic(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	// random mnemonic is generated on startup
	mnemonicResponse, err := client.GetSwapMnemonic()
	require.NoError(t, err)
	require.NotEmpty(t, mnemonicResponse.Mnemonic)

	mnemonic := "invalid"
	_, err = client.SetSwapMnemonic(&boltzrpc.SetSwapMnemonicRequest{
		Mnemonic: &boltzrpc.SetSwapMnemonicRequest_Existing{
			Existing: mnemonic,
		},
	})
	require.Error(t, err)

	createSwap := func(t *testing.T, expectedKeyIndex uint32) {
		swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Amount: swapAmount,
		})
		require.NoError(t, err)

		swapInfo, err := client.GetSwapInfo(swap.Id)
		require.NoError(t, err)

		mnemonic, err := client.GetSwapMnemonic()
		require.NoError(t, err)

		privateKey, err := boltz.DeriveKey(mnemonic.Mnemonic, expectedKeyIndex)
		require.NoError(t, err)
		require.Equal(t, hex.EncodeToString(privateKey.Serialize()), swapInfo.Swap.PrivateKey)
	}

	mnemonic = test.WalletMnemonic
	_, err = client.SetSwapMnemonic(&boltzrpc.SetSwapMnemonicRequest{
		Mnemonic: &boltzrpc.SetSwapMnemonicRequest_Existing{
			Existing: mnemonic,
		},
	})
	require.NoError(t, err)

	createSwap(t, 0)
	createSwap(t, 1)

	response, err := client.SetSwapMnemonic(&boltzrpc.SetSwapMnemonicRequest{
		Mnemonic: &boltzrpc.SetSwapMnemonicRequest_Generate{
			Generate: true,
		},
	})
	require.NoError(t, err)
	require.NotEqual(t, mnemonic, response.Mnemonic)

	createSwap(t, 0)

	t.Run("Missing", func(t *testing.T) {
		_, err := cfg.Database.Exec("DELETE FROM swapMnemonic")
		require.NoError(t, err)

		_, err = client.GetSwapMnemonic()
		require.Error(t, err)

		_, err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Amount: swapAmount,
		})
		requireCode(t, err, codes.FailedPrecondition)
	})
}
