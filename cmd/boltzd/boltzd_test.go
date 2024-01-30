package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/vulpemventures/go-elements/address"

	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltzrpc/client"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/test"

	boltzlnd "github.com/BoltzExchange/boltz-client"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func loadConfig() *boltzlnd.Config {
	dataDir := "test"
	cfg := boltzlnd.LoadConfig(dataDir)
	cfg.Database.Path = "file:test.db?cache=shared&mode=memory"
	cfg.Node = "cln"
	cfg.Node = "lnd"
	return cfg
}

var walletName = "regtest"
var password = "password"
var walletInfo = &boltzrpc.WalletInfo{Currency: "L-BTC", Name: walletName}
var credentials *wallet.Credentials

func setup(t *testing.T, cfg *boltzlnd.Config, password string) (client.Boltz, client.AutoSwap, func()) {
	if cfg == nil {
		cfg = loadConfig()
	}

	logger.Init("", "debug")

	cfg.RPC.NoTls = true
	cfg.RPC.NoMacaroons = true

	var err error

	if credentials == nil {
		var wallet *wallet.Wallet
		wallet, credentials, err = test.InitTestWallet(boltz.Currency(walletInfo.Currency), false)
		require.NoError(t, err)
		credentials.Name = walletInfo.Name
		require.NoError(t, wallet.Remove())
	}

	require.NoError(t, cfg.Database.Connect())
	encrytpedCredentials := credentials
	if password != "" {
		encrytpedCredentials, err = credentials.Encrypt(password)
		require.NoError(t, err)
	}
	_, err = cfg.Database.Exec("DELETE FROM wallets")
	require.NoError(t, err)
	require.NoError(t, cfg.Database.InsertWalletCredentials(encrytpedCredentials))

	Init(cfg)

	server := cfg.RPC.Grpc

	lis := bufconn.Listen(1024 * 1024)

	conn, err := grpc.DialContext(
		context.Background(), "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		logger.Fatal("error connecting to server: " + err.Error())
	}

	go func() {
		if err := server.Serve(lis); err != nil {
			logger.Error("error connecting serving server: " + err.Error())
		}
	}()

	go func() {
		<-cfg.RPC.Stop
		lis.Close()
		server.GracefulStop()
	}()

	clientConn := client.Connection{
		ClientConn: conn,
		Ctx:        context.Background(),
	}
	boltzClient := client.NewBoltzClient(clientConn)
	autoSwapClient := client.NewAutoSwapClient(clientConn)
	// the liquid wallet needs a bit to sync its subaccounts
	time.Sleep(200 * time.Millisecond)

	return boltzClient, autoSwapClient, func() {
		_, err = boltzClient.RemoveWallet(credentials.Name)
		require.NoError(t, err)
		require.NoError(t, boltzClient.Stop())
	}
}

func getCli(pair boltz.Pair) test.Cli {
	if pair == boltz.PairLiquid {
		return test.LiquidCli
	} else {
		return test.BtcCli
	}
}

type nextFunc func(state boltzrpc.SwapState) *boltzrpc.GetSwapInfoResponse

func swapStream(t *testing.T, client client.Boltz, swapId string) nextFunc {
	stream, err := client.GetSwapInfoStream(swapId)
	require.NoError(t, err)

	updates := make(chan *boltzrpc.GetSwapInfoResponse)

	go func() {
		for {
			status, err := stream.Recv()
			if err != nil {
				close(updates)
				return
			}
			updates <- status
		}
	}()

	return func(state boltzrpc.SwapState) *boltzrpc.GetSwapInfoResponse {
		for {
			select {
			case status, ok := <-updates:
				if ok {
					if state == status.Swap.GetState() || state == status.ReverseSwap.GetState() {
						return status
					}
				} else {
					require.Fail(t, fmt.Sprintf("update stream for swap %s stopped before state %s", swapId, state))
				}
			case <-time.After(10 * time.Second):
				require.Fail(t, fmt.Sprintf("timed out while waiting for swap %s to reach state %s", swapId, state))
			}
		}
	}
}

func withoutWallet(t *testing.T, client client.Boltz, run func()) {
	name := "regtest"
	credentials, err := client.GetWalletCredentials(name, "")
	require.NoError(t, err)

	_, err = client.RemoveWallet(name)
	require.NoError(t, err)

	run()

	_, err = client.ImportWallet(&boltzrpc.WalletInfo{
		Name:     name,
		Currency: "L-BTC",
	}, credentials, "")
	require.NoError(t, err)
}

func TestGetInfo(t *testing.T) {
	nodes := []string{"CLN"}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			cfg := loadConfig()
			cfg.Node = node
			client, _, stop := setup(t, cfg, "")
			defer stop()

			info, err := client.GetInfo()

			require.NoError(t, err)
			require.Equal(t, "regtest", info.Network)
		})
	}
}

func checkTxOutAddress(t *testing.T, chain onchain.Onchain, pair boltz.Pair, txId string, outAddress string) {
	currency, err := chain.GetCurrency(pair)
	require.NoError(t, err)
	txHex, err := currency.Tx.GetTxHex(txId)
	require.NoError(t, err)

	if pair == boltz.PairBtc {
		tx, err := boltz.NewBtcTxFromHex(txHex)
		require.NoError(t, err)

		decoded, err := btcutil.DecodeAddress(outAddress, &chaincfg.RegressionNetParams)
		require.NoError(t, err)
		script, err := txscript.PayToAddrScript(decoded)
		require.NoError(t, err)
		require.Equal(t, tx.MsgTx().TxOut[0].PkScript, script)
	} else if pair == boltz.PairLiquid {
		tx, err := boltz.NewLiquidTxFromHex(txHex, nil)
		require.NoError(t, err)

		script, err := address.ToOutputScript(outAddress)
		require.NoError(t, err)
		for _, output := range tx.Outputs {
			if len(output.Script) == 0 {
				continue
			}
			require.Equal(t, output.Script, script)
		}
	}
}

func TestSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	cfg := loadConfig()
	setBoltzEndpoint(cfg.Boltz, boltz.Regtest)
	cfg.Node = "LND"

	boltzClient := &boltz.Boltz{URL: cfg.Boltz.URL}
	chain := onchain.Onchain{
		Btc:    &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyBtc)},
		Liquid: &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyLiquid)},
	}

	checkSwap := func(t *testing.T, swap *boltzrpc.SwapInfo) {
		invoice, err := zpay32.Decode(swap.Invoice, &chaincfg.RegressionNetParams)
		require.NoError(t, err)

		excpectedFees := swap.ExpectedAmount - int64(invoice.MilliSat.ToSatoshis())
		actualFees := *swap.OnchainFee + *swap.ServiceFee
		if swap.AutoSend {
			lockupFee, err := chain.GetTransactionFee(boltz.Pair(swap.PairId), swap.LockupTransactionId)
			require.NoError(t, err)

			excpectedFees += int64(lockupFee)
		}

		require.Equal(t, excpectedFees, int64(actualFees))
	}

	t.Run("RefundAddressRequired", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")
		defer stop()
		withoutWallet(t, client, func() {
			_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				PairId: string(boltz.PairLiquid),
			})
			assert.Error(t, err)
		})
	})

	t.Run("Refund", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")
		defer stop()
		pair := boltz.PairBtc
		serviceInfo, err := client.GetServiceInfo(string(pair))
		require.NoError(t, err)
		amount := serviceInfo.Limits.Minimal - 100
		swaps := make([]*boltzrpc.CreateSwapResponse, 3)

		refundAddress := test.BtcCli("getnewaddress")
		swaps[0], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
			PairId:        string(pair),
			RefundAddress: refundAddress,
		})
		require.NoError(t, err)

		swaps[1], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
			PairId: string(pair),
		})
		require.NoError(t, err)

		swaps[2], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
			PairId: string(pair),
		})
		require.NoError(t, err)

		var streams []nextFunc
		for _, swap := range swaps {
			stream := swapStream(t, client, swap.Id)
			test.SendToAddress(test.BtcCli, swap.Address, amount)
			stream(boltzrpc.SwapState_ERROR)
			streams = append(streams, stream)
		}

		test.MineUntil(t, test.BtcCli, int64(swaps[0].TimeoutBlockHeight))

		var infos []*boltzrpc.SwapInfo
		for _, stream := range streams {
			info := stream(boltzrpc.SwapState_REFUNDED)
			require.Zero(t, info.Swap.ServiceFee)
			infos = append(infos, info.Swap)
			test.MineBlock()
		}

		require.NotEqual(t, infos[0].RefundTransactionId, infos[1].RefundTransactionId)
		require.Equal(t, infos[1].RefundTransactionId, infos[2].RefundTransactionId)

		refundFee, err := chain.GetTransactionFee(pair, infos[1].RefundTransactionId)
		require.NoError(t, err)

		require.Equal(t, int(refundFee), int(*infos[1].OnchainFee)+int(*infos[2].OnchainFee))

		checkTxOutAddress(t, chain, pair, infos[0].RefundTransactionId, refundAddress)

		refundFee, err = chain.GetTransactionFee(pair, infos[0].RefundTransactionId)
		require.NoError(t, err)
		require.Equal(t, int(refundFee), int(*infos[0].OnchainFee))
	})

	t.Run("Recovery", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")
		pair := boltz.PairBtc
		swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Amount: 100000,
			PairId: string(pair),
		})
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)
		stop()

		test.SendToAddress(test.BtcCli, swap.Address, swap.ExpectedAmount)
		test.MineBlock()

		client, _, stop = setup(t, cfg, "")
		defer stop()

		ticker := time.NewTicker(100 * time.Millisecond)
		timeout := time.After(1 * time.Second)
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

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {

			tests := []struct {
				desc   string
				pairId boltz.Pair
				cli    func(string) string
			}{
				{"BTC", boltz.PairBtc, test.BtcCli},
				{"Liquid", boltz.PairLiquid, test.LiquidCli},
			}

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					client, _, stop := setup(t, cfg, "")
					defer stop()

					t.Run("Normal", func(t *testing.T) {
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							Amount:   100000,
							PairId:   string(tc.pairId),
							AutoSend: true,
						})
						require.NoError(t, err)
						require.NotEmpty(t, swap.TxId)

						next := swapStream(t, client, swap.Id)
						next(boltzrpc.SwapState_PENDING)

						test.MineBlock()

						info := next(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

					t.Run("Deposit", func(t *testing.T) {
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							PairId: string(tc.pairId),
						})
						require.NoError(t, err)

						next := swapStream(t, client, swap.Id)
						next(boltzrpc.SwapState_PENDING)

						test.SendToAddress(tc.cli, swap.Address, 100000)
						test.MineBlock()

						info := next(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

				})
			}

		})
	}

}

func TestReverseSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	tests := []struct {
		desc     string
		pairId   boltz.Pair
		zeroConf bool
		external bool
		recover  bool
	}{
		{desc: "BTC/Normal", pairId: boltz.PairBtc},
		{desc: "BTC/ZeroConf", pairId: boltz.PairBtc, zeroConf: true, external: true},
		{desc: "BTC/Recover", pairId: boltz.PairBtc, zeroConf: true, recover: true},
		{desc: "Liquid/Normal", pairId: boltz.PairLiquid},
		{desc: "Liquid/ZeroConf", pairId: boltz.PairLiquid, zeroConf: true, external: true},
		{desc: "Liquid/Recover", pairId: boltz.PairLiquid, zeroConf: true, recover: true},
	}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					cfg := loadConfig()
					client, _, stop := setup(t, cfg, "")
					cfg.Node = node
					boltzClient := &boltz.Boltz{URL: cfg.Boltz.URL}
					chain := onchain.Onchain{
						Btc:    &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyBtc)},
						Liquid: &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyLiquid)},
					}

					addr := ""
					if tc.external {
						addr = getCli(tc.pairId)("getnewaddress")
					}

					var info *boltzrpc.GetSwapInfoResponse

					request := &boltzrpc.CreateReverseSwapRequest{
						Amount:         100000,
						Address:        addr,
						PairId:         string(tc.pairId),
						AcceptZeroConf: tc.zeroConf,
					}

					if tc.recover {
						swap, err := client.CreateReverseSwap(request)
						require.NoError(t, err)
						stop()

						test.MineBlock()

						client, _, stop = setup(t, cfg, "")

						ticker := time.NewTicker(100 * time.Millisecond)
						timeout := time.After(3 * time.Second)
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

						next := swapStream(t, client, swap.Id)
						next(boltzrpc.SwapState_PENDING)

						if !tc.zeroConf {
							test.MineBlock()
						}

						info = next(boltzrpc.SwapState_SUCCESSFUL)
					}

					require.NotZero(t, info.ReverseSwap.OnchainFee)
					require.NotZero(t, info.ReverseSwap.ServiceFee)

					invoice, err := zpay32.Decode(info.ReverseSwap.Invoice, &chaincfg.RegressionNetParams)
					require.NoError(t, err)

					claimFee, err := chain.GetTransactionFee(tc.pairId, info.ReverseSwap.ClaimTransactionId)
					require.NoError(t, err)

					totalFees := int64(invoice.MilliSat.ToSatoshis()) - info.ReverseSwap.OnchainAmount
					require.Equal(t, totalFees+int64(claimFee), int64(*info.ReverseSwap.ServiceFee+*info.ReverseSwap.OnchainFee))

					if tc.external {
						require.Equal(t, addr, info.ReverseSwap.ClaimAddress)
					}
					checkTxOutAddress(t, chain, tc.pairId, info.ReverseSwap.ClaimTransactionId, info.ReverseSwap.ClaimAddress)

					stop()
				})
			}
		})
	}
}

func TestAutoSwap(t *testing.T) {

	cfg := loadConfig()
	cfg.Node = "cln"

	require.NoError(t, cfg.Cln.Connect())
	require.NoError(t, cfg.LND.Connect())

	var us, them lightning.LightningNode
	if strings.EqualFold(cfg.Node, "lnd") {
		us = cfg.LND
		them = cfg.Cln
	} else {
		us = cfg.Cln
		them = cfg.LND
	}

	client, autoSwap, stop := setup(t, cfg, "")
	defer stop()

	tests := []struct {
		desc string
		cli  func(string) string
		pair boltz.Currency
	}{
		{"BTC", test.BtcCli, boltz.CurrencyBtc},
		//{"Liquid", liquidCli, boltz.PairLiquid},
	}

	t.Run("Setup", func(t *testing.T) {
		running := func(value bool) *autoswaprpc.GetStatusResponse {
			status, err := autoSwap.GetStatus()
			require.NoError(t, err)
			require.Equal(t, value, status.Running)
			require.NotZero(t, status.Strategy)
			return status
		}

		_, err := autoSwap.ResetConfig()
		require.NoError(t, err)

		running(false)

		_, err = autoSwap.SetConfigValue("currency", "L-BTC")
		require.NoError(t, err)

		_, err = autoSwap.Enable()
		require.NoError(t, err)

		status := running(true)
		require.Empty(t, status.Error)

		withoutWallet(t, client, func() {
			status = running(false)
			require.NotZero(t, status.Error)
		})

		status = running(true)
		require.Empty(t, status.Error)

		_, err = autoSwap.Disable()
		require.NoError(t, err)

		running(false)
	})

	t.Run("CantRemoveWallet", func(t *testing.T) {
		_, err := autoSwap.SetConfigValue("wallet", walletName)
		require.NoError(t, err)
		_, err = client.RemoveWallet(walletName)
		require.Error(t, err)
	})

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			getChannel := func(from lightning.LightningNode, peer string) *lightning.LightningChannel {
				channels, err := from.ListChannels()
				require.NoError(t, err)
				for _, channel := range channels {
					if channel.PeerId == peer {
						return channel
					}
				}
				return nil
			}

			pay := func(from lightning.LightningNode, to lightning.LightningNode, amount uint64) {
				info, err := to.GetInfo()
				require.NoError(t, err)

				channel := getChannel(from, info.Pubkey)
				require.NotNil(t, channel)

				if channel.RemoteSat < channel.Capacity/2 {
					amount = amount + (channel.Capacity/2-channel.RemoteSat)/1000
				}
				if channel.LocalSat < amount {
					return
				}

				response, err := to.CreateInvoice(int64(amount), nil, 100000, "Testt")
				require.NoError(t, err)
				_, err = from.PayInvoice(response.PaymentRequest, 10000, 30, []lightning.ChanId{channel.Id})
				require.NoError(t, err)

				time.Sleep(1000 * time.Millisecond)
			}

			swapCfg := autoswap.DefaultConfig
			swapCfg.AcceptZeroConf = true
			swapCfg.MaxFeePercent = 10
			swapCfg.AutoBudget = 1000000
			swapCfg.Currency = tc.pair
			swapCfg.Type = ""
			swapCfg.Wallet = ""

			writeConfig := func(t *testing.T) {
				swapCfgFile := cfg.DataDir + "/autoswap.toml"
				require.NoError(t, swapCfg.Write(swapCfgFile))
				_, err := autoSwap.ReloadConfig()
				require.NoError(t, err)
			}

			writeConfig(t)

			t.Run("LocalBalance", func(t *testing.T) {
				channels, err := us.ListChannels()
				require.NoError(t, err)
				var localBalance uint64
				for _, channel := range channels {
					localBalance += channel.LocalSat
				}

				swapCfg.MaxBalance = localBalance + 100
				swapCfg.Type = boltz.ReverseSwap
				swapCfg.PerChannel = false
				swapCfg.Enabled = false
				swapCfg.AutoBudget = 1000000
				swapCfg.AcceptZeroConf = true

				writeConfig(t)

				recommendations, err := autoSwap.GetSwapRecommendations(true)
				require.NoError(t, err)
				require.Zero(t, recommendations.Swaps)

				swapCfg.MaxBalance = localBalance - 100
				swapCfg.MinBalance = localBalance / 2

				writeConfig(t)

				t.Run("Recommendations", func(t *testing.T) {
					recommendations, err := autoSwap.GetSwapRecommendations(true)
					require.NoError(t, err)
					require.Len(t, recommendations.Swaps, 1)
					require.Equal(t, string(boltz.ReverseSwap), recommendations.Swaps[0].Type)
				})

				t.Run("Auto", func(t *testing.T) {
					pay(them, us, 1_000_000)
					_, err := autoSwap.SetConfigValue("enabled", "true")
					require.NoError(t, err)

					time.Sleep(200 * time.Millisecond)
					isAuto := true
					swaps, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{IsAuto: &isAuto})
					require.NoError(t, err)

					require.NotEmpty(t, swaps.ReverseSwaps)
					next := swapStream(t, client, swaps.ReverseSwaps[0].Id)
					next(boltzrpc.SwapState_SUCCESSFUL)

					stats, err := autoSwap.GetStatus()
					require.NoError(t, err)
					require.Equal(t, 1, int(stats.Stats.Count))
					require.Less(t, stats.Budget.Remaining, int64(stats.Budget.Total))
					require.NotZero(t, stats.Stats.TotalFees)
					require.NotZero(t, stats.Stats.TotalAmount)
				})
			})

			_, err := autoSwap.ResetConfig()
			require.NoError(t, err)

			/*
				t.Run("PerChannel", func(t *testing.T) {
					cfg.Swap.ChannelImbalanceThreshhold = 1
					cfg.Swap.PerChannel = true

					pay(them, us, 2000000)

					boltzPubkey := test.BoltzLnCli("getinfo | jq -r .identity_pubkey")
					boltzChannel := getChannel(us, boltzPubkey)
					if boltzChannel.LocalSat > 11_000_000 {
						invoice := test.BoltzLnCli("addinvoice 2000000 | jq -r .payment_request")
						_, err := us.PayInvoice(invoice, 10000, 30, boltzChannel.Id)
						require.NoError(t, err)
					}

					time.Sleep(1000 * time.Millisecond)

					t.Run("Recommendations", func(t *testing.T) {
						_, autoSwap, stop := setup(t, cfg, "")
						defer stop()
						recommendations, err := autoSwap.GetSwapRecommendations()
						require.NoError(t, err)
						require.Len(t, recommendations.Swaps, 2)
						require.Equal(t, boltz.ReverseSwap, recommendations.Swaps[0].Type)
						require.Empty(t, recommendations.Swaps[0].DismissedReasons)
						require.Equal(t, boltz.ReverseSwap, recommendations.Swaps[1].Type)
						require.Empty(t, recommendations.Swaps[1].DismissedReasons)
					})

					t.Run("Auto", func(t *testing.T) {
						client, _, stop := setup(t, cfg, "")
						defer stop()

						time.Sleep(500 * time.Millisecond)
						test.MineBlock()
						time.Sleep(500 * time.Millisecond)

						swaps, err := client.ListSwaps()
						require.NoError(t, err)

						info, err := them.GetInfo()
						require.NoError(t, err)

						expected := getChannel(us, info.Pubkey)

						require.NotNil(t, expected)
						require.Len(t, swaps.ReverseSwaps, 1)
						require.Len(t, swaps.Swaps, 1)

						require.Equal(t, boltzrpc.SwapState_SUCCESSFUL, swaps.ReverseSwaps[0].State)
						require.Equal(t, expected.Id, swaps.ReverseSwaps[0].ChanId)

						require.Equal(t, boltzrpc.SwapState_SUCCESSFUL, swaps.Swaps[0].State)
						require.Equal(t, boltzChannel.Id, swaps.Swaps[0].ChanId)
					})

				})
			*/
		})
	}
}

func TestWallet(t *testing.T) {
	cfg := loadConfig()
	client, _, stop := setup(t, cfg, "")
	defer stop()

	// the main setup function already created a wallet
	_, err := client.RemoveWallet(walletName)
	require.NoError(t, err)

	credentials, err := client.CreateWallet(walletInfo, password)
	require.NoError(t, err)

	_, err = client.GetWallet(walletName)
	require.NoError(t, err)

	_, err = client.RemoveWallet(walletName)
	require.NoError(t, err)

	mnemonic := "invalid"
	_, err = client.ImportWallet(walletInfo, &boltzrpc.WalletCredentials{Mnemonic: &mnemonic}, password)
	require.Error(t, err)

	_, err = client.GetWallet(walletName)
	require.Error(t, err)

	credentials.Subaccount = nil
	_, err = client.ImportWallet(walletInfo, credentials, password)
	require.NoError(t, err)

	/*
		_, err = client.GetWallet(info)
		require.Error(t, err)

	*/

	_, err = client.SetSubaccount(walletName, nil)
	require.NoError(t, err)

	_, err = client.GetWallet(walletName)
	require.NoError(t, err)
}

func TestUnlock(t *testing.T) {
	cfg := loadConfig()
	password := "password"
	client, _, stop := setup(t, cfg, password)
	defer stop()

	_, err := client.GetInfo()
	require.Error(t, err)

	require.Error(t, client.Unlock("wrong"))
	require.NoError(t, client.Unlock(password))

	test.MineBlock()
	_, err = client.GetInfo()
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(walletName, "")
	require.Error(t, err)

	c, err := client.GetWalletCredentials(walletName, password)
	require.NoError(t, err)
	require.Equal(t, credentials.Mnemonic, *c.Mnemonic)

	second := &boltzrpc.WalletInfo{Currency: "L-BTC", Name: "new"}
	_, err = client.CreateWallet(second, "wrong")
	require.Error(t, err)

	_, err = client.CreateWallet(second, password)
	require.NoError(t, err)
}

func TestCreateWalletWithPassword(t *testing.T) {
	cfg := loadConfig()
	client, _, stop := setup(t, cfg, "")
	defer stop()

	_, err := client.GetWalletCredentials(walletName, "")
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	second := &boltzrpc.WalletInfo{Name: "another", Currency: "BTC"}
	_, err = client.CreateWallet(second, password)
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(walletInfo.Name, "")
	require.Error(t, err)

	_, err = client.GetWalletCredentials(walletInfo.Name, password)
	require.NoError(t, err)

	_, err = client.RemoveWallet(second.Name)
	require.NoError(t, err)

}

func TestImportDuplicateCredentials(t *testing.T) {
	cfg := loadConfig()
	client, _, stop := setup(t, cfg, "")
	defer stop()

	credentials, err := client.GetWalletCredentials(walletName, "")
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	second := &boltzrpc.WalletInfo{Name: "another", Currency: "BTC"}
	_, err = client.ImportWallet(second, credentials, "")
	require.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	cfg := loadConfig()
	client, _, stop := setup(t, cfg, "")
	defer stop()

	_, err := client.GetWalletCredentials(walletName, "")
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

	_, err = client.GetWalletCredentials(walletName, "")
	require.Error(t, err)

	_, err = client.GetWalletCredentials(walletName, password)
	require.NoError(t, err)
}
