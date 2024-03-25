//go:build !unit

package main

import (
	"bytes"
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

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/config"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func requireCode(t *testing.T, err error, code codes.Code) {
	assert.Equal(t, code, status.Code(err))
}

func loadConfig(t *testing.T) *config.Config {
	dataDir := "test"
	cfg, err := config.LoadConfig(dataDir)
	require.NoError(t, err)
	cfg.Database.Path = t.TempDir() + "/boltz.db"
	cfg.Node = "cln"
	cfg.Node = "lnd"
	return cfg
}

var walletName = "regtest"
var password = "password"
var walletInfo = &boltzrpc.WalletInfo{Currency: boltzrpc.Currency_LBTC, Name: walletName}
var credentials *wallet.Credentials

func setup(t *testing.T, cfg *config.Config, password string) (client.Boltz, client.AutoSwap, func()) {
	if cfg == nil {
		cfg = loadConfig(t)
	}

	logger.Init("", cfg.LogLevel)

	cfg.RPC.NoTls = true
	cfg.RPC.NoMacaroons = true

	var err error

	if credentials == nil {
		var wallet *wallet.Wallet
		wallet, credentials, err = test.InitTestWallet(parseCurrency(walletInfo.Currency), false)
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

func getCli(pair boltz.Currency) test.Cli {
	if pair == boltz.CurrencyLiquid {
		return test.LiquidCli
	} else {
		return test.BtcCli
	}
}

type nextFunc func(state boltzrpc.SwapState) *boltzrpc.GetSwapInfoResponse

func swapStream(t *testing.T, client client.Boltz, swapId string) nextFunc {
	stream, err := client.GetSwapInfoStream(swapId)
	require.NoError(t, err)

	updates := make(chan *boltzrpc.GetSwapInfoResponse, 3)

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
					// ignore initial swap.created message
					created := boltz.SwapCreated.String()
					if status.Swap.GetStatus() == created || status.ReverseSwap.GetStatus() == created {
						continue
					}
					if state == status.Swap.GetState() || state == status.ReverseSwap.GetState() {
						return status
					}
				} else {
					require.Fail(t, fmt.Sprintf("update stream for swap %s stopped before state %s", swapId, state))
				}
			case <-time.After(15 * time.Second):
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
		Currency: boltzrpc.Currency_LBTC,
	}, credentials, "")
	require.NoError(t, err)
}

func TestGetInfo(t *testing.T) {
	nodes := []string{"CLN"}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			cfg := loadConfig(t)
			cfg.Node = node
			client, _, stop := setup(t, cfg, "")
			defer stop()

			info, err := client.GetInfo()

			require.NoError(t, err)
			require.Equal(t, "regtest", info.Network)
		})
	}
}

func TestGetSwapInfoStream(t *testing.T) {
	client, _, stop := setup(t, nil, "")
	defer stop()

	stream, err := client.GetSwapInfoStream("")
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

	swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{})
	require.NoError(t, err)

	select {
	case info := <-updates:
		require.Equal(t, swap.Id, info.Swap.Id)
	case <-time.After(2 * time.Second):
		require.Fail(t, "no swap update received")
	}

	reverseSwap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
	require.NoError(t, err)

	select {
	case info := <-updates:
		require.Equal(t, reverseSwap.Id, info.ReverseSwap.Id)
	case <-time.After(2 * time.Second):
		require.Fail(t, "no reverse swap update received")
	}
}

func TestGetPairs(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, cfg, "")
	defer stop()

	info, err := client.GetPairs()

	require.NoError(t, err)
	require.Len(t, info.Submarine, 2)
	require.Len(t, info.Reverse, 2)
}

func checkTxOutAddress(t *testing.T, chain onchain.Onchain, currency boltz.Currency, txId string, outAddress string, cooperative bool) {
	transaction, err := chain.GetTransaction(currency, txId, nil)
	require.NoError(t, err)

	if tx, ok := transaction.(*boltz.BtcTransaction); ok {

		for _, input := range tx.MsgTx().TxIn {
			if cooperative {
				require.Len(t, input.Witness, 1)
			} else {
				require.Greater(t, len(input.Witness), 1)
			}
		}

		if outAddress != "" {
			decoded, err := btcutil.DecodeAddress(outAddress, &chaincfg.RegressionNetParams)
			require.NoError(t, err)
			script, err := txscript.PayToAddrScript(decoded)
			require.NoError(t, err)
			for _, output := range tx.MsgTx().TxOut {
				if bytes.Equal(output.PkScript, script) {
					return
				}
			}
			require.Fail(t, "could not find output address in transaction")
		}
	} else if tx, ok := transaction.(*boltz.LiquidTransaction); ok {
		for _, input := range tx.Inputs {
			if cooperative {
				require.Len(t, input.Witness, 1)
			} else {
				require.Greater(t, len(input.Witness), 1)
			}
		}
		if outAddress != "" {
			script, err := address.ToOutputScript(outAddress)
			require.NoError(t, err)
			for _, output := range tx.Outputs {
				if len(output.Script) == 0 {
					continue
				}
				if bytes.Equal(output.Script, script) {
					return
				}
			}
			require.Fail(t, "could not find output address in transaction")
		}
	}
}

func parseCurrency(grpcCurrency boltzrpc.Currency) boltz.Currency {
	if grpcCurrency == boltzrpc.Currency_BTC {
		return boltz.CurrencyBtc
	} else {
		return boltz.CurrencyLiquid
	}
}

var pairBtc = &boltzrpc.Pair{
	From: boltzrpc.Currency_BTC,
	To:   boltzrpc.Currency_BTC,
}

func TestSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

	cfg := loadConfig(t)
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
		if swap.Wallet != nil {
			lockupFee, err := chain.GetTransactionFee(parseCurrency(swap.Pair.From), swap.LockupTransactionId)
			require.NoError(t, err)

			excpectedFees += int64(lockupFee)
		}

		require.Equal(t, excpectedFees, int64(actualFees))
	}

	t.Run("Recovery", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")
		swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
			Amount: 100000,
			Pair:   pairBtc,
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

	t.Run("Invoice", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")
		defer stop()

		t.Run("Invalid", func(t *testing.T) {
			invoice := "invalid"
			_, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice: &invoice,
			})
			require.Error(t, err)
		})

		t.Run("Valid", func(t *testing.T) {
			node := cfg.LND
			invoice, err := node.CreateInvoice(100000, nil, 0, "test")
			require.NoError(t, err)
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice:          &invoice.PaymentRequest,
				SendFromInternal: true,
			})
			require.NoError(t, err)

			stream := swapStream(t, client, swap.Id)
			info := stream(boltzrpc.SwapState_PENDING)
			require.Equal(t, invoice.PaymentRequest, info.Swap.Invoice)

			test.MineBlock()
			stream(boltzrpc.SwapState_SUCCESSFUL)

			paid, err := node.CheckInvoicePaid(invoice.PaymentHash)
			require.NoError(t, err)
			require.True(t, paid)
		})
	})

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {

			tests := []struct {
				desc string
				from boltzrpc.Currency
				cli  func(string) string
			}{
				{"BTC", boltzrpc.Currency_BTC, test.BtcCli},
				{"Liquid", boltzrpc.Currency_LBTC, test.LiquidCli},
			}

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					pair := &boltzrpc.Pair{
						From: tc.from,
						To:   boltzrpc.Currency_BTC,
					}
					client, _, stop := setup(t, cfg, "")
					defer stop()

					t.Run("Normal", func(t *testing.T) {
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							Amount:           100000,
							Pair:             pair,
							SendFromInternal: true,
						})
						require.NoError(t, err)
						require.NotEmpty(t, swap.TxId)
						require.NotZero(t, swap.TimeoutHours)
						require.NotZero(t, swap.TimeoutBlockHeight)

						next := swapStream(t, client, swap.Id)
						test.MineBlock()

						info := next(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})
					t.Run("Deposit", func(t *testing.T) {
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							Pair: pair,
						})
						require.NoError(t, err)

						test.SendToAddress(tc.cli, swap.Address, 100000)
						test.MineBlock()

						next := swapStream(t, client, swap.Id)
						info := next(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

					if node == "CLN" {
						return
					}

					t.Run("Refund", func(t *testing.T) {
						cli := tc.cli

						submarinePair, err := client.GetSubmarinePair(pair)

						require.NoError(t, err)
						amount := submarinePair.Limits.Minimal

						t.Run("Script", func(t *testing.T) {
							cfg.Boltz.DisablePartialSignatures = true
							swaps := make([]*boltzrpc.CreateSwapResponse, 3)

							refundAddress := cli("getnewaddress")
							swaps[0], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:          pair,
								RefundAddress: &refundAddress,
								Amount:        int64(amount + 100),
							})
							require.NoError(t, err)

							swaps[1], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:   pair,
								Amount: int64(amount + 100),
							})
							require.NoError(t, err)

							swaps[2], err = client.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:   pair,
								Amount: int64(amount + 100),
							})
							require.NoError(t, err)

							var streams []nextFunc
							for _, swap := range swaps {
								stream := swapStream(t, client, swap.Id)
								test.SendToAddress(cli, swap.Address, int64(amount))
								stream(boltzrpc.SwapState_ERROR)
								streams = append(streams, stream)
							}

							test.MineUntil(t, cli, int64(swaps[0].TimeoutBlockHeight))

							var infos []*boltzrpc.SwapInfo
							for _, stream := range streams {
								info := stream(boltzrpc.SwapState_REFUNDED)
								require.Zero(t, info.Swap.ServiceFee)
								infos = append(infos, info.Swap)
								test.MineBlock()
							}

							from := parseCurrency(pair.From)

							require.NotEqual(t, infos[0].RefundTransactionId, infos[1].RefundTransactionId)
							require.Equal(t, infos[1].RefundTransactionId, infos[2].RefundTransactionId)

							refundFee, err := chain.GetTransactionFee(from, infos[1].RefundTransactionId)
							require.NoError(t, err)

							require.Equal(t, int(refundFee), int(*infos[1].OnchainFee)+int(*infos[2].OnchainFee))

							checkTxOutAddress(t, chain, from, infos[0].RefundTransactionId, refundAddress, false)

							refundFee, err = chain.GetTransactionFee(from, infos[0].RefundTransactionId)
							require.NoError(t, err)
							require.Equal(t, int(refundFee), int(*infos[0].OnchainFee))
						})

						createFailedSwap := func(t *testing.T, refundAddress string) nextFunc {
							cfg.Boltz.DisablePartialSignatures = false
							amount := submarinePair.Limits.Minimal + 100
							swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:          pair,
								RefundAddress: &refundAddress,
								Amount:        int64(amount + 100),
							})
							require.NoError(t, err)

							stream := swapStream(t, client, swap.Id)
							test.SendToAddress(cli, swap.Address, int64(amount))
							return stream
						}

						t.Run("Cooperative", func(t *testing.T) {
							refundAddress := cli("getnewaddress")
							stream := createFailedSwap(t, refundAddress)

							info := stream(boltzrpc.SwapState_REFUNDED).Swap
							require.Zero(t, info.ServiceFee)

							from := parseCurrency(pair.From)

							refundFee, err := chain.GetTransactionFee(from, info.RefundTransactionId)
							require.NoError(t, err)
							require.Equal(t, int(refundFee), int(*info.OnchainFee))

							checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)
						})

						t.Run("Manual", func(t *testing.T) {
							if tc.from == boltzrpc.Currency_LBTC {
								withoutWallet(t, client, func() {
									stream := createFailedSwap(t, "")

									info := stream(boltzrpc.SwapState_ERROR).Swap
									refundAddress := cli("getnewaddress")

									t.Run("Invalid", func(t *testing.T) {
										_, err := client.RefundSwap(info.Id, "invalid")
										requireCode(t, err, codes.InvalidArgument)

										_, err = client.RefundSwap("invalid", refundAddress)
										requireCode(t, err, codes.NotFound)
									})

									t.Run("Valid", func(t *testing.T) {
										_, err := client.RefundSwap(info.Id, refundAddress)
										require.NoError(t, err)

										info = stream(boltzrpc.SwapState_REFUNDED).Swap
										require.Zero(t, info.ServiceFee)

										from := parseCurrency(pair.From)

										refundFee, err := chain.GetTransactionFee(from, info.RefundTransactionId)
										require.NoError(t, err)
										assert.Equal(t, int(refundFee), int(*info.OnchainFee))

										checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)

										_, err = client.RefundSwap(info.Id, refundAddress)
										requireCode(t, err, codes.FailedPrecondition)
									})
								})
							}
						})
					})
				})
			}

		})
	}

}

func TestReverseSwap(t *testing.T) {
	nodes := []string{"CLN", "LND"}

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
					cfg.Boltz.DisablePartialSignatures = tc.disablePartials
					client, _, stop := setup(t, cfg, "")
					cfg.Node = node
					chain := onchain.Onchain{
						Btc:    &onchain.Currency{Tx: onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyBtc)},
						Liquid: &onchain.Currency{Tx: onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyLiquid)},
					}

					pair := &boltzrpc.Pair{
						From: boltzrpc.Currency_BTC,
						To:   tc.to,
					}

					addr := ""
					if tc.external {
						addr = getCli(parseCurrency(tc.to))("getnewaddress")
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

						if tc.waitForClaim && tc.zeroConf {
							require.NotZero(t, swap.ClaimTransactionId)
							info, err = client.GetSwapInfo(swap.Id)
							require.NoError(t, err)
						} else {
							next := swapStream(t, client, swap.Id)
							next(boltzrpc.SwapState_PENDING)

							if !tc.zeroConf {
								test.MineBlock()
							}

							info = next(boltzrpc.SwapState_SUCCESSFUL)
						}

					}

					require.NotZero(t, info.ReverseSwap.OnchainFee)
					require.NotZero(t, info.ReverseSwap.ServiceFee)

					invoice, err := zpay32.Decode(info.ReverseSwap.Invoice, &chaincfg.RegressionNetParams)
					require.NoError(t, err)

					currency := parseCurrency(tc.to)

					claimFee, err := chain.GetTransactionFee(currency, info.ReverseSwap.ClaimTransactionId)
					require.NoError(t, err)

					totalFees := int64(invoice.MilliSat.ToSatoshis()) - info.ReverseSwap.OnchainAmount
					require.Equal(t, totalFees+int64(claimFee), int64(*info.ReverseSwap.ServiceFee+*info.ReverseSwap.OnchainFee))

					if tc.external {
						require.Equal(t, addr, info.ReverseSwap.ClaimAddress)
					}
					checkTxOutAddress(t, chain, currency, info.ReverseSwap.ClaimTransactionId, info.ReverseSwap.ClaimAddress, !tc.disablePartials)

					stop()
				})
			}
		})
	}

	t.Run("ExternalPay", func(t *testing.T) {
		cfg := loadConfig(t)
		client, _, stop := setup(t, cfg, "")
		defer stop()

		externalPay := true
		returnImmediately := false

		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:            100000,
			AcceptZeroConf:    true,
			ExternalPay:       &externalPay,
			ReturnImmediately: &returnImmediately,
		}

		// cant wait for claim transaction if paid externally
		_, err := client.CreateReverseSwap(request)
		require.Error(t, err)

		request.ReturnImmediately = nil

		swap, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		require.NotEmpty(t, swap.Invoice)

		stream := swapStream(t, client, swap.Id)

		_, err = cfg.Lightning.PayInvoice(*swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)

		test.MineBlock()
	})

}

func TestAutoSwap(t *testing.T) {

	cfg := loadConfig(t)
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
		desc     string
		cli      func(string) string
		currency boltzrpc.Currency
	}{
		{"BTC", test.BtcCli, boltzrpc.Currency_BTC},
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

		_, err = autoSwap.SetConfigValue("currency", "LBTC")
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

			swapCfg := autoswap.DefaultConfig()
			swapCfg.AcceptZeroConf = true
			swapCfg.MaxFeePercent = 10
			swapCfg.Budget = 1000000
			swapCfg.Currency = tc.currency
			swapCfg.SwapType = ""
			swapCfg.Wallet = ""

			writeConfig := func(t *testing.T) {
				_, err := autoSwap.SetConfig(swapCfg)
				require.NoError(t, err)
				_, err = autoSwap.ReloadConfig()
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
				swapCfg.SwapType = "reverse"
				swapCfg.PerChannel = false
				swapCfg.Enabled = false
				swapCfg.Budget = 1000000
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
	cfg := loadConfig(t)
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
	cfg := loadConfig(t)
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

	second := &boltzrpc.WalletInfo{Currency: boltzrpc.Currency_LBTC, Name: "new"}
	_, err = client.CreateWallet(second, "wrong")
	require.Error(t, err)

	_, err = client.CreateWallet(second, password)
	require.NoError(t, err)
}

func TestCreateWalletWithPassword(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, cfg, "")
	defer stop()

	_, err := client.GetWalletCredentials(walletName, "")
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	second := &boltzrpc.WalletInfo{Name: "another", Currency: boltzrpc.Currency_BTC}
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
	cfg := loadConfig(t)
	client, _, stop := setup(t, cfg, "")
	defer stop()

	credentials, err := client.GetWalletCredentials(walletName, "")
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	second := &boltzrpc.WalletInfo{Name: "another", Currency: boltzrpc.Currency_BTC}
	_, err = client.ImportWallet(second, credentials, "")
	require.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	cfg := loadConfig(t)
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
