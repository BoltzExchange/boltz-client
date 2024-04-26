//go:build !unit

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/database"

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

	if cfg.Node == "" || cfg.Node == "Standalone" {
		cfg.Standalone = true
	}

	logger.Init("", cfg.LogLevel)

	cfg.RPC.NoTls = true
	cfg.RPC.NoMacaroons = false

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
	require.NoError(t, cfg.Database.CreateWallet(&database.Wallet{
		Credentials: encrytpedCredentials,
	}))

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

	clientConn := client.Connection{ClientConn: conn}
	macaroonFile, err := os.ReadFile("./test/macaroons/admin.macaroon")
	require.NoError(t, err)
	clientConn.SetMacaroon(hex.EncodeToString(macaroonFile))

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

func getCli(pair boltzrpc.Currency) test.Cli {
	if pair == boltzrpc.Currency_LBTC {
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
					if state == status.Swap.GetState() || state == status.ReverseSwap.GetState() || state == status.ChainSwap.GetState() {
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
	nodes := []string{"CLN", "LND", "Standalone"}

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

func TestMacaroons(t *testing.T) {
	admin, _, stop := setup(t, nil, "")
	conn := admin.Connection
	defer stop()

	fullPermissions := []*boltzrpc.MacaroonPermissions{
		{Action: boltzrpc.MacaroonAction_READ},
		{Action: boltzrpc.MacaroonAction_WRITE},
	}

	readPermissions := []*boltzrpc.MacaroonPermissions{
		{Action: boltzrpc.MacaroonAction_READ},
	}

	t.Run("Admin", func(t *testing.T) {
		response, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: fullPermissions,
		})
		require.NoError(t, err)

		anotherAdmin := client.NewBoltzClient(conn)

		anotherAdmin.SetMacaroon(response.Macaroon)

		response, err = admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: readPermissions,
		})
		require.NoError(t, err)

		anotherAdmin.SetMacaroon(response.Macaroon)

		// write actions are not allowed now
		_, err = anotherAdmin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: readPermissions,
		})
		require.Error(t, err)

		err = anotherAdmin.Stop()
		require.Error(t, err)

		_, err = anotherAdmin.ListEntities()
		require.NoError(t, err)
	})

	t.Run("Entity", func(t *testing.T) {

		entityName := "test"

		entityInfo, err := admin.CreateEntity(entityName)
		require.NoError(t, err)
		require.NotZero(t, entityInfo.Id)

		write, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			EntityId:    &entityInfo.Id,
			Permissions: fullPermissions,
		})
		require.NoError(t, err)

		readonly, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			EntityId:    &entityInfo.Id,
			Permissions: readPermissions,
		})
		require.NoError(t, err)

		entity := client.NewBoltzClient(conn)
		entity.SetMacaroon(write.Macaroon)

		entityAuto := client.NewAutoSwapClient(conn)
		entityAuto.SetMacaroon(write.Macaroon)

		readEntity := client.NewBoltzClient(conn)
		readEntity.SetMacaroon(readonly.Macaroon)

		t.Run("Parameter", func(t *testing.T) {
			withEntityParam := client.NewBoltzClient(conn)
			withEntityParam.SetEntity(entityInfo.Id)

			info, err := withEntityParam.GetInfo()
			require.NoError(t, err)
			require.Equal(t, entityInfo.Id, *info.EntityId)
		})

		t.Run("Admin", func(t *testing.T) {
			_, err = entity.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
				EntityId:    &entityInfo.Id,
				Permissions: readPermissions,
			})
			require.Error(t, err)

			_, err = entity.GetEntity(entityInfo.Name)
			require.Error(t, err)

			_, err = entity.ListEntities()
			require.Error(t, err)

			_, err = admin.GetEntity(entityInfo.Name)
			require.NoError(t, err)

			list, err := admin.ListEntities()
			require.NoError(t, err)
			require.Len(t, list.Entities, 1)
		})

		t.Run("Info", func(t *testing.T) {
			info, err := readEntity.GetInfo()
			require.NoError(t, err)
			require.Empty(t, info.NodePubkey)
			require.Equal(t, entityInfo.Id, *info.EntityId)
		})

		t.Run("AutoSwap", func(t *testing.T) {
			_, err := entityAuto.GetConfig()
			require.Error(t, err)
		})

		t.Run("Wallet", func(t *testing.T) {
			_, err = entity.GetWallet(walletName)
			require.Error(t, err)

			_, err = readEntity.CreateWallet(walletInfo, "")
			require.Error(t, err)

			_, err = entity.CreateWallet(walletInfo, "")
			require.NoError(t, err)

			wallets, err := entity.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, 1)

			wallets, err = admin.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, 2)
		})

		t.Run("Swaps", func(t *testing.T) {
			_, err = admin.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.NoError(t, err)
			_, err = admin.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
			require.NoError(t, err)

			swaps, err := readEntity.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Empty(t, swaps.Swaps)
			require.Empty(t, swaps.ReverseSwaps)

			_, err = readEntity.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
				Amount: 100000,
			})
			require.Error(t, err)

			externalPay := false
			_, err = entity.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
				Amount:      100000,
				ExternalPay: &externalPay,
			})
			require.Errorf(t, err, "no lightning node available, external pay required")

			_, err = entity.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.Error(t, err, "no lightning node available, invoice required")

			swaps, err = admin.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Len(t, swaps.Swaps, 1)
			require.Len(t, swaps.ReverseSwaps, 1)
		})
	})

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
		if swap.WalletId != nil {
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

	t.Run("Standalone", func(t *testing.T) {
		cfg := loadConfig(t)
		cfg.Standalone = true
		client, _, stop := setup(t, cfg, "")
		node := cfg.LND
		require.NoError(t, node.Connect())
		_, err := node.GetInfo()
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

		stream := swapStream(t, client, swap.Id)
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

						createFailedSwap := func(t *testing.T, refundAddress string) nextFunc {
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

						t.Run("Script", func(t *testing.T) {
							cfg.Boltz.DisablePartialSignatures = true

							refundAddress := cli("getnewaddress")
							withStream := createFailedSwap(t, refundAddress)
							withoutStream := createFailedSwap(t, refundAddress)

							withInfo := withStream(boltzrpc.SwapState_ERROR).Swap
							withoutStream(boltzrpc.SwapState_ERROR)

							test.MineUntil(t, cli, int64(withInfo.TimeoutBlockHeight))

							withInfo = withStream(boltzrpc.SwapState_REFUNDED).Swap
							withoutInfo := withoutStream(boltzrpc.SwapState_REFUNDED).Swap

							from := parseCurrency(pair.From)

							require.Equal(t, withInfo.RefundTransactionId, withoutInfo.RefundTransactionId)
							refundFee, err := chain.GetTransactionFee(from, withInfo.RefundTransactionId)
							require.NoError(t, err)

							require.Equal(t, int(refundFee), int(*withInfo.OnchainFee)+int(*withoutInfo.OnchainFee))

							checkTxOutAddress(t, chain, from, withInfo.RefundTransactionId, refundAddress, false)
						})

						t.Run("Cooperative", func(t *testing.T) {
							cfg.Boltz.DisablePartialSignatures = false

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

						if tc.from == boltzrpc.Currency_LBTC {
							t.Run("Manual", func(t *testing.T) {
								withoutWallet(t, client, func() {
									stream := createFailedSwap(t, "")

									info := stream(boltzrpc.SwapState_ERROR).Swap
									refundAddress := cli("getnewaddress")

									clientInfo, err := client.GetInfo()
									require.NoError(t, err)
									require.Equal(t, clientInfo.RefundableSwaps[0], info.Id)

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
										requireCode(t, err, codes.NotFound)
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
						next := swapStream(t, client, swap.Id)
						next(boltzrpc.SwapState_PENDING)
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

	t.Run("Standalone", func(t *testing.T) {
		cfg := loadConfig(t)
		cfg.Standalone = true
		lnd := cfg.LND
		connectLightning(lnd)

		client, _, stop := setup(t, cfg, "")
		defer stop()

		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:         100000,
			AcceptZeroConf: true,
		}
		_, err := client.CreateReverseSwap(request)
		// theres no btc wallet
		require.Error(t, err)

		request.Address = test.BtcCli("getnewaddress")
		swap, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		require.NotEmpty(t, swap.Invoice)

		_, err = lnd.PayInvoice(*swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream := swapStream(t, client, swap.Id)
		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)

		require.Equal(t, info.ReverseSwap.State, boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)
	})

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

func TestChainSwap(t *testing.T) {
	cfg := loadConfig(t)
	setBoltzEndpoint(cfg.Boltz, boltz.Regtest)
	boltzClient := &boltz.Boltz{URL: cfg.Boltz.URL}

	chain := onchain.Onchain{
		Btc:    &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyBtc)},
		Liquid: &onchain.Currency{Tx: onchain.NewBoltzTxProvider(boltzClient, boltz.CurrencyLiquid)},
	}

	tests := []struct {
		desc string
		from boltzrpc.Currency
		to   boltzrpc.Currency
	}{
		{"BTC", boltzrpc.Currency_BTC, boltzrpc.Currency_LBTC},
		{"Liquid", boltzrpc.Currency_LBTC, boltzrpc.Currency_BTC},
	}

	t.Run("Recovery", func(t *testing.T) {
		client, _, stop := setup(t, cfg, "")

		externalPay := true
		to := test.LiquidCli("getnewaddress")
		swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
			Amount:      100000,
			Pair:        &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC},
			ExternalPay: &externalPay,
			ToAddress:   &to,
		})
		require.NoError(t, err)
		stream := swapStream(t, client, swap.Id)
		stream(boltzrpc.SwapState_PENDING)
		stop()

		test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, swap.FromData.Amount)
		test.MineBlock()

		client, _, stop = setup(t, cfg, "")
		defer stop()

		stream = swapStream(t, client, "")
		update := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
		require.Equal(t, swap.Id, update.Id)
		checkTxOutAddress(t, chain, boltz.CurrencyLiquid, update.ToData.GetTransactionId(), update.ToData.GetAddress(), true)
	})

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			pair := &boltzrpc.Pair{
				From: tc.from,
				To:   tc.to,
			}
			client, _, stop := setup(t, cfg, "")
			defer stop()

			fromCli := getCli(tc.from)
			toCli := getCli(tc.to)

			refundAddress := fromCli("getnewaddress")
			toAddress := toCli("getnewaddress")

			wallets, err := client.GetWallets(&tc.from, false)
			require.NoError(t, err)
			require.NotEmpty(t, wallets.Wallets)
			fromWalletId := wallets.Wallets[0].Id

			wallets, err = client.GetWallets(&tc.to, false)
			require.NoError(t, err)
			require.NotEmpty(t, wallets.Wallets)
			toWalletId := wallets.Wallets[0].Id

			checkSwap := func(t *testing.T, id string) {
				response, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{})
				require.NoError(t, err)
				require.NotEmpty(t, response.ChainSwaps)
				for _, swap := range response.ChainSwaps {
					if swap.Id == id {
						fromFee, err := chain.GetTransactionFee(parseCurrency(tc.from), swap.FromData.GetLockupTransactionId())
						require.NoError(t, err)
						if swap.FromData.WalletId == nil {
							fromFee = 0
						}
						toFee, err := chain.GetTransactionFee(parseCurrency(tc.to), swap.ToData.GetLockupTransactionId())
						require.NoError(t, err)
						claimFee, err := chain.GetTransactionFee(parseCurrency(tc.to), swap.ToData.GetTransactionId())
						require.NoError(t, err)

						require.Equal(t, int(fromFee+toFee+claimFee), int(*swap.OnchainFee))
						return
					}
				}
				require.Fail(t, "swap not returned by listswaps", id)
			}

			t.Run("InternalWallets", func(t *testing.T) {
				toWallet, err := client.GetWalletById(toWalletId)
				require.NoError(t, err)
				prev := toWallet.Balance.Total

				zeroConf := true
				swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
					Amount:         100000,
					Pair:           pair,
					ToWalletId:     &toWalletId,
					FromWalletId:   &fromWalletId,
					AcceptZeroConf: &zeroConf,
				})
				require.NoError(t, err)
				require.NotEmpty(t, swap.Id)

				stream := swapStream(t, client, swap.Id)
				test.MineBlock()
				stream(boltzrpc.SwapState_SUCCESSFUL)

				// gdk takes too long to sync
				if tc.to == boltzrpc.Currency_BTC {
					toWallet, err = client.GetWalletById(toWalletId)
					require.NoError(t, err)
					require.Greater(t, toWallet.Balance.Total, prev)
				}

				checkSwap(t, swap.Id)
			})

			t.Run("External", func(t *testing.T) {
				externalPay := true
				swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
					Amount:      100000,
					Pair:        pair,
					ExternalPay: &externalPay,
					ToAddress:   &toAddress,
				})
				require.NoError(t, err)
				require.NotEmpty(t, swap.Id)
				require.NotEmpty(t, swap.ToData.Address)

				stream := swapStream(t, client, swap.Id)
				test.SendToAddress(fromCli, swap.FromData.LockupAddress, swap.FromData.Amount)
				test.MineBlock()
				stream(boltzrpc.SwapState_PENDING)
				test.MineBlock()
				info := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap

				to := parseCurrency(tc.to)
				checkTxOutAddress(t, chain, to, info.ToData.GetTransactionId(), info.ToData.GetAddress(), true)

				checkSwap(t, swap.Id)
			})

			t.Run("Refund", func(t *testing.T) {
				submarinePair, err := client.GetChainPair(pair)

				require.NoError(t, err)

				createFailed := func(t *testing.T, refundAddress string) nextFunc {
					amount := submarinePair.Limits.Minimal + 100
					externalPay := true
					swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Pair:          pair,
						RefundAddress: &refundAddress,
						ToAddress:     &toAddress,
						Amount:        amount + 100,
						ExternalPay:   &externalPay,
					})
					require.NoError(t, err)

					stream := swapStream(t, client, swap.Id)
					test.SendToAddress(fromCli, swap.FromData.LockupAddress, int64(amount))
					return stream
				}

				t.Run("Script", func(t *testing.T) {
					cfg.Boltz.DisablePartialSignatures = true

					withStream := createFailed(t, refundAddress)
					withoutStream := createFailed(t, refundAddress)

					withInfo := withStream(boltzrpc.SwapState_ERROR).ChainSwap
					withoutStream(boltzrpc.SwapState_ERROR)

					test.MineUntil(t, fromCli, int64(withInfo.FromData.TimeoutBlockHeight))

					withInfo = withStream(boltzrpc.SwapState_REFUNDED).ChainSwap
					withoutInfo := withoutStream(boltzrpc.SwapState_REFUNDED).ChainSwap

					from := parseCurrency(pair.From)

					require.Equal(t, withInfo.FromData.GetTransactionId(), withoutInfo.FromData.GetTransactionId())
					refundFee, err := chain.GetTransactionFee(from, withInfo.FromData.GetTransactionId())
					require.NoError(t, err)

					require.Equal(t, refundFee, *withInfo.OnchainFee+*withoutInfo.OnchainFee)

					checkTxOutAddress(t, chain, from, withInfo.FromData.GetTransactionId(), refundAddress, false)
				})

				t.Run("Cooperative", func(t *testing.T) {
					cfg.Boltz.DisablePartialSignatures = false

					stream := createFailed(t, refundAddress)

					info := stream(boltzrpc.SwapState_REFUNDED).ChainSwap
					require.Zero(t, info.ServiceFee)

					from := parseCurrency(pair.From)

					refundFee, err := chain.GetTransactionFee(from, info.FromData.GetTransactionId())
					require.NoError(t, err)
					require.Equal(t, refundFee, *info.OnchainFee)

					checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)
				})

				if tc.from == boltzrpc.Currency_LBTC {
					t.Run("Manual", func(t *testing.T) {
						stream := createFailed(t, "")
						withoutWallet(t, client, func() {

							info := stream(boltzrpc.SwapState_ERROR).ChainSwap

							clientInfo, err := client.GetInfo()
							require.NoError(t, err)
							require.Len(t, clientInfo.RefundableSwaps, 1)
							require.Equal(t, clientInfo.RefundableSwaps[0], info.Id)

							t.Run("Invalid", func(t *testing.T) {
								_, err := client.RefundSwap(info.Id, "invalid")
								requireCode(t, err, codes.InvalidArgument)

								_, err = client.RefundSwap("invalid", refundAddress)
								requireCode(t, err, codes.NotFound)
							})

							t.Run("Valid", func(t *testing.T) {
								_, err := client.RefundSwap(info.Id, refundAddress)
								require.NoError(t, err)

								info = stream(boltzrpc.SwapState_REFUNDED).ChainSwap
								require.Zero(t, info.ServiceFee)

								from := parseCurrency(pair.From)

								refundFee, err := chain.GetTransactionFee(from, info.FromData.GetTransactionId())
								require.NoError(t, err)
								assert.Equal(t, int(refundFee), int(*info.OnchainFee))

								checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)

								_, err = client.RefundSwap(info.Id, refundAddress)
								requireCode(t, err, codes.NotFound)
							})
						})
					})
				}
			})
		})
	}
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
	_, err := client.GetWalletCredentials(walletName, "")
	require.NoError(t, err)

	// the main setup function already created a wallet
	_, err = client.RemoveWallet(walletName)
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
