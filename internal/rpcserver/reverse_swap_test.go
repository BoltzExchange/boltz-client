//go:build !unit

package rpcserver

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/stretchr/testify/mock"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestReverseSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	checkSwap := func(t *testing.T, client client.Boltz, id string, chain *onchain.Onchain) {
		info, err := client.GetSwapInfo(id)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.NotZero(t, info.ReverseSwap.OnchainFee)
		require.NotZero(t, info.ReverseSwap.ServiceFee)
		require.NotNil(t, info.ReverseSwap.PaidAt)
		require.LessOrEqual(t, *info.ReverseSwap.PaidAt, time.Now().Unix())
		require.GreaterOrEqual(t, *info.ReverseSwap.PaidAt, info.ReverseSwap.CreatedAt)

		currency := parseCurrency(info.ReverseSwap.Pair.To)

		claimFee := getTransactionFee(t, chain, currency, info.ReverseSwap.ClaimTransactionId)

		totalFees := info.ReverseSwap.InvoiceAmount - info.ReverseSwap.OnchainAmount
		require.Equal(t, int64(totalFees+claimFee), *info.ReverseSwap.ServiceFee+int64(*info.ReverseSwap.OnchainFee))
	}

	tests := []struct {
		desc            string
		to              boltzrpc.Currency
		zeroConf        bool
		external        bool
		recover         bool
		disablePartials bool
		waitForClaim    bool
	}{
		{desc: "BTC/Normal", to: boltzrpc.Currency_BTC, disablePartials: true},
		{desc: "BTC/ZeroConf", to: boltzrpc.Currency_BTC, zeroConf: true, external: true},
		{desc: "BTC/Recover", to: boltzrpc.Currency_BTC, zeroConf: true, recover: true},
		{desc: "Liquid/Normal", to: boltzrpc.Currency_LBTC, disablePartials: true},
		{desc: "Liquid/ZeroConf", to: boltzrpc.Currency_LBTC, zeroConf: true, external: true},
		{desc: "Liquid/Recover", to: boltzrpc.Currency_LBTC, zeroConf: true, recover: true},
		{desc: "Wait", to: boltzrpc.Currency_BTC, zeroConf: true, waitForClaim: true},
	}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					cfg := loadConfig(t)
					boltzApi := getBoltz(t, cfg)
					boltzApi.DisablePartialSignatures = tc.disablePartials
					cfg.Node = node
					chain := getOnchain(t, cfg)
					client, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi, chain: chain})
					fundedWallet(t, client, tc.to)

					pair := &boltzrpc.Pair{
						From: boltzrpc.Currency_BTC,
						To:   tc.to,
					}

					addr := ""
					if tc.external {
						addr = getCli(tc.to)("getnewaddress")
					}

					var info *boltzrpc.GetSwapInfoResponse

					returnImmediately := !tc.waitForClaim
					request := &boltzrpc.CreateReverseSwapRequest{
						Amount:            100000,
						Address:           addr,
						Pair:              pair,
						AcceptZeroConf:    tc.zeroConf,
						ReturnImmediately: &returnImmediately,
					}

					if tc.recover {
						request.AcceptZeroConf = false
						swap, err := client.CreateReverseSwap(request)
						require.NoError(t, err)
						_, statusStream := swapStream(t, client, swap.Id)
						statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
						stop()

						test.MineBlock()

						client, _, stop = setup(t, setupOptions{cfg: cfg})

						ticker := time.NewTicker(200 * time.Millisecond)
						timeout := time.After(5 * time.Second)
						for info.GetReverseSwap().GetState() != boltzrpc.SwapState_SUCCESSFUL {
							select {
							case <-ticker.C:
								info, err = client.GetSwapInfo(swap.Id)
								require.NoError(t, err)
							case <-timeout:
								require.Fail(t, "timed out while waiting for swap")
							}
						}
					} else {
						swap, err := client.CreateReverseSwap(request)
						require.NoError(t, err)

						if tc.waitForClaim && tc.zeroConf {
							require.NotZero(t, swap.ClaimTransactionId)
							info, err = client.GetSwapInfo(swap.Id)
							require.NoError(t, err)
						} else {
							status, statusStream := swapStream(t, client, swap.Id)

							if !tc.zeroConf {
								statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
								test.MineBlock()
							}

							info = status(boltzrpc.SwapState_SUCCESSFUL)
						}

					}

					swap := info.GetReverseSwap()
					checkSwap(t, client, swap.Id, chain)
					if tc.external {
						require.Equal(t, addr, swap.ClaimAddress)
					}
					toCurrency := parseCurrency(swap.Pair.To)
					checkTxOutAddress(t, chain, toCurrency, swap.ClaimTransactionId, swap.ClaimAddress, !tc.disablePartials)

					stop()
				})
			}
		})
	}

	t.Run("MaxRoutingFee", func(t *testing.T) {
		ppm := uint64(10)
		amount := uint64(1_000_000)
		cfg := loadConfig(t)
		client, _, stop := setup(t, setupOptions{cfg: cfg})
		defer stop()

		externalPay := false
		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:             amount,
			AcceptZeroConf:     true,
			RoutingFeeLimitPpm: &ppm,
			ExternalPay:        &externalPay,
		}

		response, err := client.CreateReverseSwap(request)
		require.NoError(t, err)

		dbSwap, err := cfg.Database.QueryReverseSwap(response.Id)
		require.NoError(t, err)
		require.Equal(t, ppm, *dbSwap.RoutingFeeLimitPpm)

		externalPay = true
		_, err = client.CreateReverseSwap(request)
		require.Error(t, err)
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("Invalid", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		originalChain := chain.Btc.Chain

		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		tests := []struct {
			desc        string
			chainMocker chainMocker
			error       string
		}{
			{"LessValue", lessValueChainProvider, "locked up less"},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				chain.Btc.Chain = tc.chainMocker(t, originalChain)
				swap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
					Amount:         100000,
					AcceptZeroConf: false,
				})
				require.NoError(t, err)

				_, statusStream := swapStream(t, client, swap.Id)
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
				test.MineBlock()
				info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionConfirmed)
				require.Contains(t, info.ReverseSwap.Error, tc.error)
			})
		}
	})

	t.Run("Retry", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		chain.Btc.Chain = flakyChainProvider(t, chain.Btc.Chain)

		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:         100000,
			AcceptZeroConf: false,
		}

		swap, err := client.CreateReverseSwap(request)
		require.NoError(t, err)

		_, statusStream := swapStream(t, client, swap.Id)
		statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
		test.MineBlock()
		// on first call, the broadcast will fail
		statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionConfirmed)
		test.MineBlock()
		// new block triggers a retry, on which the broadcast will succeed
		statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.InvoiceSettled)
		checkSwap(t, client, swap.Id, chain)
	})

	t.Run("Standalone", func(t *testing.T) {
		cfg := loadConfig(t)
		cfg.Standalone = true
		lnd := cfg.LND
		_, err := connectLightning(nil, lnd)
		require.NoError(t, err)

		client, _, stop := setup(t, setupOptions{cfg: cfg})
		defer stop()

		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:         100000,
			AcceptZeroConf: true,
		}
		_, err = client.CreateReverseSwap(request)
		// theres no btc wallet
		require.Error(t, err)

		request.Address = test.BtcCli("getnewaddress")
		swap, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		require.NotEmpty(t, swap.Invoice)

		stream, _ := swapStream(t, client, swap.Id)

		_, err = lnd.PayInvoice(context.Background(), *swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)

		require.Equal(t, info.ReverseSwap.State, boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)
	})

	t.Run("ExternalPay", func(t *testing.T) {
		cfg := loadConfig(t)
		require.NoError(t, cfg.Cln.Connect())
		client, _, stop := setup(t, setupOptions{cfg: cfg, node: "lnd"})
		defer stop()

		externalPay := true
		description := "test"
		descriptionHash := sha256.Sum256([]byte(description))

		getRequest := func() *boltzrpc.CreateReverseSwapRequest {
			return &boltzrpc.CreateReverseSwapRequest{
				Amount:         100000,
				AcceptZeroConf: true,
				ExternalPay:    &externalPay,
			}
		}

		t.Run("DescriptionHash", func(t *testing.T) {
			request := getRequest()
			request.DescriptionHash = descriptionHash[:]

			swap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)

			decoded, err := zpay32.Decode(*swap.Invoice, &chaincfg.RegressionNetParams)
			require.NoError(t, err)

			require.Equal(t, descriptionHash[:], decoded.DescriptionHash[:])
		})

		t.Run("Description", func(t *testing.T) {
			request := getRequest()
			request.Description = &description
			swap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)

			decoded, err := zpay32.Decode(*swap.Invoice, &chaincfg.RegressionNetParams)
			require.NoError(t, err)

			require.Equal(t, btcutil.Amount(request.Amount), decoded.MilliSat.ToSatoshis())
			require.Equal(t, description, *decoded.Description)
		})

		t.Run("ReturnImmediately", func(t *testing.T) {
			request := getRequest()
			returnImmediately := false
			request.ReturnImmediately = &returnImmediately

			// cant wait for claim transaction if paid externally
			_, err := client.CreateReverseSwap(request)
			require.Error(t, err)
		})

		t.Run("Pay", func(t *testing.T) {
			request := getRequest()

			swap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)
			require.NotEmpty(t, swap.Invoice)

			stream, _ := swapStream(t, client, swap.Id)

			_, err = cfg.Cln.PayInvoice(context.Background(), *swap.Invoice, 10000, 30, nil)
			require.NoError(t, err)

			stream(boltzrpc.SwapState_PENDING)
			info := stream(boltzrpc.SwapState_SUCCESSFUL)
			require.True(t, info.ReverseSwap.ExternalPay)
			require.Nil(t, info.ReverseSwap.RoutingFeeMsat)

			test.MineBlock()
		})

		t.Run("InvoiceExpiry", func(t *testing.T) {
			request := getRequest()
			expiry := uint64(15 * 60)
			request.InvoiceExpiry = &expiry

			swap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)

			decoded, err := zpay32.Decode(*swap.Invoice, &chaincfg.RegressionNetParams)
			require.NoError(t, err)

			require.Equal(t, decoded.Expiry(), time.Duration(expiry)*time.Second)

			expiry = 24 * 60 * 60
			_, err = client.CreateReverseSwap(request)
			require.Error(t, err)
		})

	})

	t.Run("ManualClaim", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		createClaimable := func(t *testing.T) (*boltzrpc.ReverseSwapInfo, streamFunc, streamStatusFunc) {
			response, err := client.CreateWallet(&boltzrpc.WalletParams{
				Currency: boltzrpc.Currency_BTC,
				Name:     "temp",
			})
			require.NoError(t, err)
			swap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
				Amount:   100000,
				WalletId: &response.Wallet.Id,
			})
			require.NoError(t, err)

			stream, statusStream := swapStream(t, client, swap.Id)
			statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)

			_, err = client.RemoveWallet(response.Wallet.Id)
			require.NoError(t, err)

			// we dont accept zero conf, so the swap is not claimable yet
			info, err := client.GetInfo()
			require.NoError(t, err)
			require.NotContains(t, info.ClaimableSwaps, swap.Id)

			test.MineBlock()

			update := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionConfirmed)

			// we add a small sleep here to avoid the race where boltz says confirmed but the chain provider hasn't synced
			time.Sleep(100 * time.Millisecond)

			info, err = client.GetInfo()
			require.NoError(t, err)
			require.Contains(t, info.ClaimableSwaps, swap.Id)

			return update.ReverseSwap, stream, statusStream
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
				destination.Address = test.BtcCli("getnewaddress")
				response, err := client.ClaimSwaps(request)
				require.NoError(t, err)

				checkTxOutAddress(t, chain, boltz.CurrencyBtc, response.TransactionId, destination.Address, true)

				_, err = client.ClaimSwaps(request)
				requireCode(t, err, codes.NotFound)
			})
		})
		t.Run("Wallet", func(t *testing.T) {
			info, _, _ := createClaimable(t)

			destination := &boltzrpc.ClaimSwapsRequest_WalletId{}
			request := &boltzrpc.ClaimSwapsRequest{SwapIds: []string{info.Id}, Destination: destination}

			t.Run("Invalid", func(t *testing.T) {
				destination.WalletId = 234213412341234
				_, err := client.ClaimSwaps(request)
				requireCode(t, err, codes.NotFound)
			})

			t.Run("Valid", func(t *testing.T) {
				destinationWallet := emptyWallet(t, client, boltzrpc.Currency_BTC)
				destination.WalletId = destinationWallet.Id
				_, err := client.ClaimSwaps(request)
				require.NoError(t, err)

				_, err = client.ClaimSwaps(request)
				requireCode(t, err, codes.NotFound)
			})
		})
	})
}

func fundedWallet(t *testing.T, client client.Boltz, currency boltzrpc.Currency) *boltzrpc.Wallet {
	params := &boltzrpc.WalletParams{Currency: currency, Name: walletName(currency)}
	wallet, err := client.GetWallet(params.Name)
	if err != nil {
		mnemonic := test.WalletMnemonic
		subaccount := uint64(test.WalletSubaccount)
		creds := &boltzrpc.WalletCredentials{Mnemonic: &mnemonic, Subaccount: &subaccount}
		wallet, err = client.ImportWallet(params, creds)
		require.NoError(t, err)
	}
	if wallet.Balance.Total == 0 {
		for i := 0; i < 5; i++ {
			receive, err := client.WalletReceive(wallet.Id)
			require.NoError(t, err)
			test.SendToAddress(getCli(currency), receive.Address, 10_000_000)
			time.Sleep(200 * time.Millisecond)
		}
		test.MineBlock()
		require.Eventually(t, func() bool {
			wallet, err = client.GetWalletById(wallet.Id)
			require.NoError(t, err)
			return wallet.Balance.Total > 0
		}, 10*time.Second, 250*time.Millisecond)
	}
	return wallet
}

func walletId(t *testing.T, client client.Boltz, currency boltzrpc.Currency) uint64 {
	wallets, err := client.GetWallets(&currency, false)
	require.NoError(t, err)
	require.NotEmpty(t, wallets.Wallets)
	return wallets.Wallets[0].Id
}

func emptyWallet(t *testing.T, client client.Boltz, currency boltzrpc.Currency) *boltzrpc.Wallet {
	name := "empty" + currency.String()
	response, err := client.CreateWallet(&boltzrpc.WalletParams{
		Currency: currency,
		Name:     name,
	})
	if err != nil {
		existing, err := client.GetWallet(name)
		require.NoError(t, err)
		return existing
	}
	return response.Wallet
}

func TestDirectReverseSwapPayments(t *testing.T) {
	cfg := loadConfig(t)
	maxZeroConfAmount := uint64(100000)
	cfg.MaxZeroConfAmount = &maxZeroConfAmount
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
	fundedWallet(t, client, boltzrpc.Currency_LBTC)
	defer stop()

	t.Run("AddMagicRoutingHint", func(t *testing.T) {
		addMrh := true
		request := &boltzrpc.CreateReverseSwapRequest{
			Amount: 100000,
			Pair: &boltzrpc.Pair{
				From: boltzrpc.Currency_BTC,
				To:   boltzrpc.Currency_LBTC,
			},
			AddMagicRoutingHint: &addMrh,
		}
		_, err := client.CreateReverseSwap(request)
		requireCode(t, err, codes.InvalidArgument)

		wallet := emptyWallet(t, client, boltzrpc.Currency_LBTC)
		externalPay := true
		request.ExternalPay = &externalPay
		request.WalletId = &wallet.Id

		createAndCheckMrh := func() (*boltzrpc.ReverseSwapInfo, *btcec.PublicKey) {
			swap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)

			swapInfo, err := client.GetSwapInfo(swap.Id)
			require.NoError(t, err)

			decoded, err := zpay32.Decode(*swap.Invoice, &chaincfg.RegressionNetParams)
			require.NoError(t, err)
			return swapInfo.ReverseSwap, boltz.FindMagicRoutingHint(decoded)
		}

		swapInfo, key := createAndCheckMrh()
		require.NotNil(t, key)
		require.NotEmpty(t, swapInfo.ClaimAddress)

		addMrh = false
		swapInfo, key = createAndCheckMrh()
		require.Nil(t, key)
		require.Empty(t, swapInfo.ClaimAddress)
	})

	tt := []struct {
		desc     string
		zeroconf bool
		currency boltzrpc.Currency
	}{
		{"Btc/Normal", false, boltzrpc.Currency_BTC},
		{"Liquid/Normal", false, boltzrpc.Currency_LBTC},
		{"Liquid/ZeroConf", true, boltzrpc.Currency_LBTC},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {

			confirmed := false
			if !tc.zeroconf || tc.currency == boltzrpc.Currency_BTC {
				currency, _ := chain.GetCurrency(serializers.ParseCurrency(&tc.currency))
				mockTx := onchainmock.NewMockChainProvider(t)
				mockTx.EXPECT().IsTransactionConfirmed(mock.Anything).RunAndReturn(func(string) (bool, error) {
					return confirmed, nil
				})
				mockTx.EXPECT().BroadcastTransaction(mock.Anything).RunAndReturn(currency.Chain.BroadcastTransaction).Maybe()
				coverChainProvider(t, mockTx, currency.Chain)
				currency.Chain = mockTx
			}

			externalPay := true
			addMrh := true
			request := &boltzrpc.CreateReverseSwapRequest{
				Pair: &boltzrpc.Pair{
					From: boltzrpc.Currency_BTC,
					To:   tc.currency,
				},
				ExternalPay:         &externalPay,
				AddMagicRoutingHint: &addMrh,
			}
			if tc.zeroconf {
				request.Amount = maxZeroConfAmount / 2
			} else {
				request.Amount = maxZeroConfAmount * 2
			}
			reverseSwap, err := client.CreateReverseSwap(request)
			require.NoError(t, err)

			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Pair: &boltzrpc.Pair{
					From: tc.currency,
					To:   boltzrpc.Currency_BTC,
				},
				Invoice:          reverseSwap.Invoice,
				SendFromInternal: true,
			})
			require.NoError(t, err)
			require.NotEmpty(t, swap.Bip21)
			require.NotEmpty(t, swap.TxId)
			require.NotZero(t, swap.ExpectedAmount)
			require.Empty(t, swap.Id)

			_, statusStream := swapStream(t, client, reverseSwap.Id)
			if !tc.zeroconf {
				if tc.currency == boltzrpc.Currency_BTC {
					// for btc, we only check on new blocks, so we mine one here,
					// but the mocked tx provider says that the tx isn't confirmed yet
					test.MineBlock()
				}
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionDirectMempool)
			}

			confirmed = true
			test.MineBlock()
			info := statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.TransactionDirect)
			require.Equal(t, info.ReverseSwap.ClaimAddress, swap.Address)
			time.Sleep(1 * time.Second)
		})
	}
}
