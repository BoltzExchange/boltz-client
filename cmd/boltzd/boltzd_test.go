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
	"github.com/BoltzExchange/boltz-client/mempool"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/liquid"
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

var liquidWallet *liquid.Wallet

func loadConfig() *boltzlnd.Config {
	dataDir := "test"
	cfg := boltzlnd.LoadConfig(dataDir)
	cfg.Database.Path = "file:test.db?cache=shared&mode=memory"
	cfg.Node = "cln"
	cfg.LiquidWallet = liquidWallet
	cfg.Node = "lnd"
	return cfg
}

func setup(t *testing.T, cfg *boltzlnd.Config) (client.Boltz, client.AutoSwap, func()) {
	if cfg == nil {
		cfg = loadConfig()
	}

	logger.Init("", "debug")

	cfg.RPC.NoTls = true
	cfg.RPC.NoMacaroons = true

	if liquidWallet.Exists() {
		require.NoError(t, liquidWallet.Login())
	} else {
		_, err := liquidWallet.Register()
		require.NoError(t, err)
	}

	balance, err := liquidWallet.GetBalance()
	if err != nil {
		require.NoError(t, err)
	}
	if balance.Total == 0 {
		addr, err := liquidWallet.NewAddress()
		if err != nil {
			require.NoError(t, err)
		}
		test.LiquidCli("sendtoaddress " + addr + " 1")
		test.MineBlock()
		ticker := time.NewTicker(1 * time.Second)
		timeout := time.After(15 * time.Second)
		for balance.Total == 0 {
			select {
			case <-ticker.C:
				balance, err = liquidWallet.GetBalance()
				require.NoError(t, err)
			case <-timeout:
				t.Error("timeout")
			}
		}
	}

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

	return boltzClient, autoSwapClient, func() { require.NoError(t, boltzClient.Stop()) }
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
					require.Fail(t, "update stream stopped before state", state.String())
				}
			case <-time.After(10 * time.Second):
				require.Fail(t, "timed out while waiting for state", state.String())
			}
		}
	}
}

func withoutWallet(t *testing.T, client client.Boltz, run func()) {
	mnemonic, err := client.GetLiquidWalletMnemonic()
	require.NoError(t, err)

	info, err := client.GetLiquidWalletInfo()
	require.NoError(t, err)

	_, err = client.RemoveLiquidWallet()
	require.NoError(t, err)

	run()

	_, err = client.ImportLiquidWallet(mnemonic.Mnemonic)
	require.NoError(t, err)

	_, err = client.GetLiquidSubaccounts()
	require.NoError(t, err)

	_, err = client.SetLiquidSubaccount(&info.Subaccount.Pointer)
	require.NoError(t, err)
}

func TestMain(m *testing.M) {
	var err error
	liquidWallet, err = liquid.InitWallet("./data", boltz.Regtest, false)
	if err != nil {
		fmt.Println("Could not init liquid wallet: " + err.Error())
		return
	}
	m.Run()
}

func TestGetInfo(t *testing.T) {
	nodes := []string{"CLN"}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			cfg := loadConfig()
			cfg.Node = node
			client, _, stop := setup(t, cfg)
			defer stop()

			info, err := client.GetInfo()

			require.NoError(t, err)
			require.Equal(t, "regtest", info.Network)
		})
	}
}

func TestDeposit(t *testing.T) {
	nodes := []string{"CLN", "LND"}
	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			cfg := loadConfig()
			cfg.Node = node
			client, _, stop := setup(t, cfg)
			chain := onchain.Onchain{
				Btc:    &onchain.Currency{Tx: mempool.InitClient(cfg.MempoolApi)},
				Liquid: &onchain.Currency{Tx: mempool.InitClient(cfg.MempoolLiquidApi)},
			}
			defer stop()

			tests := []struct {
				desc   string
				pairId boltz.Pair
				cli    func(string) string
			}{
				{"BTC", boltz.PairBtc, test.BtcCli},
				{"Liquid", boltz.PairLiquid, test.LiquidCli},
			}

			t.Run("RefundAddressRequired", func(t *testing.T) {
				withoutWallet(t, client, func() {
					_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
						PairId: string(boltz.PairLiquid),
					})
					assert.Error(t, err)
				})
			})

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {

					checkSwap := func(t *testing.T, swap *boltzrpc.SwapInfo) {
						invoice, err := zpay32.Decode(swap.Invoice, &chaincfg.RegressionNetParams)
						require.NoError(t, err)

						excpectedFees := swap.ExpectedAmount - int64(invoice.MilliSat.ToSatoshis())
						actualFees := *swap.OnchainFee + *swap.ServiceFee
						if swap.AutoSend {
							lockupFee, err := chain.GetTransactionFee(tc.pairId, swap.LockupTransactionId)
							require.NoError(t, err)

							excpectedFees += int64(lockupFee)
						}

						require.Equal(t, excpectedFees, int64(actualFees))
					}

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

					t.Run("Recovery", func(t *testing.T) {
						client, _, stop := setup(t, cfg)
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							Amount: 100000,
							PairId: string(tc.pairId),
						})
						time.Sleep(500 * time.Millisecond)
						require.NoError(t, err)
						stop()

						test.SendToAddress(tc.cli, swap.Address, swap.ExpectedAmount)
						test.MineBlock()

						client, _, stop = setup(t, cfg)
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

					t.Run("Refund", func(t *testing.T) {
						serviceInfo, err := client.GetServiceInfo(string(tc.pairId))
						require.NoError(t, err)
						amount := serviceInfo.Limits.Minimal - 100
						swaps := make([]*boltzrpc.CreateSwapResponse, 3)

						refundAddress := tc.cli("getnewaddress")
						swaps[0], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
							PairId:        string(tc.pairId),
							RefundAddress: refundAddress,
						})
						require.NoError(t, err)

						swaps[1], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
							PairId: string(tc.pairId),
						})
						require.NoError(t, err)

						swaps[2], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
							PairId: string(tc.pairId),
						})
						require.NoError(t, err)

						var streams []nextFunc
						for _, swap := range swaps {
							stream := swapStream(t, client, swap.Id)
							test.SendToAddress(tc.cli, swap.Address, amount)
							stream(boltzrpc.SwapState_ERROR)
							streams = append(streams, stream)
						}

						test.MineUntil(t, tc.cli, int64(swaps[0].TimeoutBlockHeight))

						var infos []*boltzrpc.SwapInfo
						for _, stream := range streams {
							info := stream(boltzrpc.SwapState_REFUNDED)
							require.Zero(t, info.Swap.ServiceFee)
							infos = append(infos, info.Swap)
							test.MineBlock()
						}

						require.NotEqual(t, infos[0].RefundTransactionId, infos[1].RefundTransactionId)
						require.Equal(t, infos[1].RefundTransactionId, infos[2].RefundTransactionId)

						refundFee, err := chain.GetTransactionFee(tc.pairId, infos[1].RefundTransactionId)
						require.NoError(t, err)

						require.Equal(t, int(refundFee), int(*infos[1].OnchainFee)+int(*infos[2].OnchainFee))

						currency, err := chain.GetCurrency(tc.pairId)
						require.NoError(t, err)
						txHex, err := currency.Tx.GetTxHex(infos[0].RefundTransactionId)
						require.NoError(t, err)

						if tc.pairId == boltz.PairBtc {
							tx, err := boltz.NewBtcTxFromHex(txHex)
							require.NoError(t, err)

							decoded, err := btcutil.DecodeAddress(refundAddress, &chaincfg.RegressionNetParams)
							require.NoError(t, err)
							script, err := txscript.PayToAddrScript(decoded)
							require.NoError(t, err)
							require.Equal(t, tx.MsgTx().TxOut[0].PkScript, script)
						} else if tc.pairId == boltz.PairLiquid {
							tx, err := boltz.NewLiquidTxFromHex(txHex, nil)
							require.NoError(t, err)

							script, err := address.ToOutputScript(refundAddress)
							require.NoError(t, err)
							for _, output := range tx.Outputs {
								if len(output.Script) == 0 {
									continue
								}
								require.Equal(t, output.Script, script)
							}
						}

						refundFee, err = chain.GetTransactionFee(tc.pairId, infos[0].RefundTransactionId)
						require.NoError(t, err)
						require.Equal(t, int(refundFee), int(*infos[0].OnchainFee))
					})

				})
			}

		})
	}

}

func TestReverseSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	tests := []struct {
		desc   string
		pairId boltz.Pair
		cli    func(string) string
	}{
		//{"BTC", "BTC/BTC", false, btcCli},
		{"BTC", boltz.PairBtc, test.BtcCli},
		{"Liquid", boltz.PairLiquid, test.LiquidCli},
	}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			cfg := loadConfig()
			cfg.Node = node
			client, _, stop := setup(t, cfg)
			chain := onchain.Onchain{
				Btc:    &onchain.Currency{Tx: mempool.InitClient(cfg.MempoolApi)},
				Liquid: &onchain.Currency{Tx: mempool.InitClient(cfg.MempoolLiquidApi)},
			}
			defer stop()

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					swap, err := client.CreateReverseSwap(100000, "", true, string(tc.pairId))
					require.NoError(t, err)

					next := swapStream(t, client, swap.Id)
					next(boltzrpc.SwapState_PENDING)

					info := next(boltzrpc.SwapState_SUCCESSFUL)
					require.NotZero(t, info.ReverseSwap.OnchainFee)
					require.NotZero(t, info.ReverseSwap.ServiceFee)
					test.MineBlock()

					invoice, err := zpay32.Decode(info.ReverseSwap.Invoice, &chaincfg.RegressionNetParams)
					require.NoError(t, err)

					claimFee, err := chain.GetTransactionFee(tc.pairId, info.ReverseSwap.ClaimTransactionId)
					require.NoError(t, err)

					totalFees := int64(invoice.MilliSat.ToSatoshis()) - info.ReverseSwap.OnchainAmount
					require.Equal(t, totalFees+int64(claimFee), int64(*info.ReverseSwap.ServiceFee+*info.ReverseSwap.OnchainFee))
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

	client, autoSwap, stop := setup(t, cfg)
	defer stop()

	tests := []struct {
		desc string
		cli  func(string) string
		pair boltz.Pair
	}{
		{"BTC", test.BtcCli, boltz.PairBtc},
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

		_, err = autoSwap.SetConfigValue("pair", "L-BTC/BTC")
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
				_, err = from.PayInvoice(response.PaymentRequest, 10000, 30, channel.Id)
				require.NoError(t, err)

				time.Sleep(1000 * time.Millisecond)
			}

			swapCfg := autoswap.DefaultConfig
			swapCfg.AcceptZeroConf = true
			swapCfg.MaxFeePercent = 10
			swapCfg.AutoBudget = 1000000
			swapCfg.Pair = tc.pair
			swapCfg.Type = ""

			t.Run("LocalBalance", func(t *testing.T) {
				channels, err := us.ListChannels()
				require.NoError(t, err)
				var localBalance uint64
				for _, channel := range channels {
					localBalance += channel.LocalSat
				}

				swapCfg.LocalBalanceThreshold = localBalance + 100
				swapCfg.PerChannel = false
				swapCfg.Enabled = false
				swapCfg.AutoBudget = 1000000
				swapCfg.AcceptZeroConf = true

				swapCfgFile := cfg.DataDir + "/autoswap.toml"
				require.NoError(t, swapCfg.Write(swapCfgFile))
				_, err = autoSwap.ReloadConfig()
				require.NoError(t, err)

				recommendations, err := autoSwap.GetSwapRecommendations(true)
				require.NoError(t, err)
				require.Zero(t, recommendations.Swaps)

				swapCfg.LocalBalanceThreshold = localBalance - 100
				require.NoError(t, swapCfg.Write(swapCfgFile))

				_, err = autoSwap.ReloadConfig()
				require.NoError(t, err)

				t.Run("Recommendations", func(t *testing.T) {
					recommendations, err := autoSwap.GetSwapRecommendations(true)
					require.NoError(t, err)
					require.Len(t, recommendations.Swaps, 1)
					require.Equal(t, string(boltz.ReverseSwap), recommendations.Swaps[0].Type)
				})

				pay(them, us, 1_000_000)

				t.Run("Auto", func(t *testing.T) {
					_, err := autoSwap.SetConfigValue("enabled", "true")
					require.NoError(t, err)

					time.Sleep(200 * time.Millisecond)
					swaps, err := client.ListSwaps()
					require.NoError(t, err)

					require.NotEmpty(t, swaps.ReverseSwaps)
					next := swapStream(t, client, swaps.ReverseSwaps[len(swaps.ReverseSwaps)-1].Id)
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
						_, autoSwap, stop := setup(t, cfg)
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
						client, _, stop := setup(t, cfg)
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

func TestLiquidWallet(t *testing.T) {
	cfg := loadConfig()
	client, _, stop := setup(t, cfg)
	defer stop()

	// the main setup function already created a wallet
	_, err := client.RemoveLiquidWallet()
	require.NoError(t, err)

	mnemonic, err := client.CreateLiquidWallet()
	require.NoError(t, err)

	info, err := client.GetLiquidWalletInfo()
	require.NoError(t, err)
	require.Equal(t, "p2wpkh", info.Subaccount.Type)

	_, err = client.RemoveLiquidWallet()
	require.NoError(t, err)

	_, err = client.ImportLiquidWallet("invalid")
	require.Error(t, err)

	_, err = client.GetLiquidWalletInfo()
	require.Error(t, err)

	_, err = client.ImportLiquidWallet(mnemonic.Mnemonic)
	require.NoError(t, err)

	_, err = client.GetLiquidWalletInfo()
	require.Error(t, err)

	_, err = client.SetLiquidSubaccount(nil)
	require.NoError(t, err)

	_, err = client.GetLiquidWalletInfo()
	require.NoError(t, err)
}
