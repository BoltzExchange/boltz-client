//go:build !unit

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/database"

	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/vulpemventures/go-elements/address"

	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltzrpc/client"
	onchainmock "github.com/BoltzExchange/boltz-client/mocks/github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
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
	cfg.LogLevel = "debug"
	cfg.Database.Path = t.TempDir() + "/boltz.db"
	cfg.Node = "lnd"
	return cfg
}

func getBoltz(t *testing.T, cfg *config.Config) *boltz.Api {
	boltzApi, err := initBoltz(cfg, boltz.Regtest)
	require.NoError(t, err)
	return boltzApi
}

type txMocker func(t *testing.T, original onchain.TxProvider) *onchainmock.MockTxProvider

func lessValueTxProvider(t *testing.T, original onchain.TxProvider) *onchainmock.MockTxProvider {
	txMock := onchainmock.NewMockTxProvider(t)
	txMock.EXPECT().GetRawTransaction(mock.Anything).RunAndReturn(func(txId string) (string, error) {
		raw, err := original.GetRawTransaction(txId)
		require.NoError(t, err)
		transaction, err := boltz.NewBtcTxFromHex(raw)
		require.NoError(t, err)
		for _, out := range transaction.MsgTx().TxOut {
			out.Value -= 1
		}
		return transaction.Serialize()
	})
	return txMock
}

func unconfirmedTxProvider(t *testing.T, original onchain.TxProvider) *onchainmock.MockTxProvider {
	txMock := onchainmock.NewMockTxProvider(t)
	txMock.EXPECT().IsTransactionConfirmed(mock.Anything).Return(false, nil)
	txMock.EXPECT().GetRawTransaction(mock.Anything).RunAndReturn(original.GetRawTransaction)
	return txMock
}

func getOnchain(t *testing.T, cfg *config.Config) *onchain.Onchain {
	chain, err := initOnchain(cfg, boltz.Regtest)
	require.NoError(t, err)

	boltzApi := getBoltz(t, cfg)

	chain.Btc.Tx = onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyBtc)
	chain.Liquid.Tx = onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyLiquid)

	return chain
}

var walletName = "regtest"
var password = "password"
var walletParams = &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: walletName}
var credentials *wallet.Credentials
var testWallet *database.Wallet

type setupOptions struct {
	cfg      *config.Config
	password string
	chain    *onchain.Onchain
	boltzApi *boltz.Api
	node     string
}

func setup(t *testing.T, options setupOptions) (client.Boltz, client.AutoSwap, func()) {
	cfg := options.cfg
	if cfg == nil {
		cfg = loadConfig(t)
	}
	if options.node != "" {
		cfg.Node = options.node
	}
	cfg.RPC.NoTls = true

	if cfg.Node == "" || cfg.Node == "Standalone" {
		cfg.Standalone = true
	}

	logger.Init("", cfg.LogLevel)

	var err error
	if credentials == nil {
		var wallet *wallet.Wallet
		wallet, credentials, err = test.InitTestWallet(parseCurrency(walletParams.Currency), false)
		require.NoError(t, err)
		credentials.Name = walletName
		credentials.EntityId = database.DefaultEntityId
		require.NoError(t, wallet.Remove())
	}

	require.NoError(t, cfg.Database.Connect())

	encrytpedCredentials := credentials
	if options.password != "" {
		encrytpedCredentials, err = credentials.Encrypt(password)
		require.NoError(t, err)
	}
	_, err = cfg.Database.Exec("DELETE FROM wallets")
	require.NoError(t, err)
	testWallet = &database.Wallet{
		Credentials: encrytpedCredentials,
	}
	require.NoError(t, cfg.Database.CreateWallet(testWallet))

	lightningNode, err := initLightning(cfg)
	require.NoError(t, err)

	if options.chain == nil {
		options.chain = getOnchain(t, cfg)
	}
	if options.boltzApi == nil {
		options.boltzApi = getBoltz(t, cfg)
	}

	autoSwapConfPath := path.Join(t.TempDir(), "autoswap.toml")

	err = cfg.RPC.Init(boltz.Regtest, lightningNode, options.boltzApi, cfg.Database, options.chain, autoSwapConfPath)
	require.NoError(t, err)
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
		_, err = boltzClient.RemoveWallet(testWallet.Id)
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

type streamFunc func(state boltzrpc.SwapState) *boltzrpc.GetSwapInfoResponse

type streamStatusFunc func(state boltzrpc.SwapState, status boltz.SwapUpdateEvent) *boltzrpc.GetSwapInfoResponse

func swapStream(t *testing.T, client client.Boltz, swapId string) (streamFunc, streamStatusFunc) {
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

	streamFunc := func(state boltzrpc.SwapState, status boltz.SwapUpdateEvent) *boltzrpc.GetSwapInfoResponse {
		for {
			select {
			case update, ok := <-updates:
				if ok {
					currentStatus := update.Swap.GetStatus() + update.ReverseSwap.GetStatus() + update.ChainSwap.GetStatus()

					if status != boltz.SwapCreated && status.String() != currentStatus {
						continue
					}
					if state == update.Swap.GetState() || state == update.ReverseSwap.GetState() || state == update.ChainSwap.GetState() {
						return update
					}
				} else {
					require.Fail(t, fmt.Sprintf("update stream for swap %s stopped before state %s", swapId, state))
				}
			case <-time.After(15 * time.Second):
				require.Fail(t, fmt.Sprintf("timed out while waiting for swap %s to reach state %s", swapId, state))
			}
		}
	}

	return func(state boltzrpc.SwapState) *boltzrpc.GetSwapInfoResponse {
		return streamFunc(state, boltz.SwapCreated)
	}, streamFunc
}

func TestGetInfo(t *testing.T) {
	nodes := []string{"CLN", "LND", "Standalone"}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			client, _, stop := setup(t, setupOptions{node: node})
			defer stop()

			info, err := client.GetInfo()

			require.NoError(t, err)
			require.Equal(t, "regtest", info.Network)
		})
	}
}

func TestMacaroons(t *testing.T) {
	fullPermissions := []*boltzrpc.MacaroonPermissions{
		{Action: boltzrpc.MacaroonAction_READ},
		{Action: boltzrpc.MacaroonAction_WRITE},
	}

	readPermissions := []*boltzrpc.MacaroonPermissions{
		{Action: boltzrpc.MacaroonAction_READ},
	}

	admin, _, stop := setup(t, setupOptions{})
	defer stop()
	conn := admin.Connection

	entityName := "test"

	entityInfo, err := admin.CreateEntity(entityName)
	require.NoError(t, err)
	require.NotZero(t, entityInfo.Id)

	list, err := admin.ListEntities()
	require.NoError(t, err)
	require.Len(t, list.Entities, 2)

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

	t.Run("Bake", func(t *testing.T) {
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
	})

	t.Run("Info", func(t *testing.T) {
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.NotNil(t, info.Entity)

		info, err = readEntity.GetInfo()
		require.NoError(t, err)
		require.Empty(t, info.NodePubkey)
		require.Equal(t, entityInfo.Id, info.Entity.Id)
	})

	t.Run("AutoSwap", func(t *testing.T) {
		_, err := entityAuto.GetConfig()
		require.Error(t, err)
	})

	t.Run("Wallet", func(t *testing.T) {
		hasWallets := func(t *testing.T, client client.Boltz, amount int) {
			wallets, err := client.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, amount)
		}
		hasWallets(t, entity, 0)
		hasWallets(t, admin, 2)

		_, err = entity.GetWallet(walletName)
		requireCode(t, err, codes.NotFound)

		_, err = readEntity.CreateWallet(walletParams)
		requireCode(t, err, codes.PermissionDenied)

		_, err = entity.CreateWallet(walletParams)
		requireCode(t, err, codes.OK)

		hasWallets(t, entity, 1)
		hasWallets(t, admin, 2)
	})

	t.Run("Swaps", func(t *testing.T) {
		hasSwaps := func(t *testing.T, client client.Boltz, length int) {
			swaps, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Len(t, swaps.Swaps, length)
			require.Len(t, swaps.ReverseSwaps, length)
			//require.Len(t, swaps.ChainSwaps, length)
		}

		t.Run("Admin", func(t *testing.T) {
			hasSwaps(t, admin, 0)
			_, err = admin.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.NoError(t, err)
			externalPay := false
			_, err = admin.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
			require.NoError(t, err)
			hasSwaps(t, admin, 1)
		})

		t.Run("Entity", func(t *testing.T) {
			hasSwaps(t, entity, 0)
			_, err = entity.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.ErrorContains(t, err, "invoice is required in standalone mode")
			externalPay := false
			_, err = entity.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
			require.ErrorContains(t, err, "can not create reverse swap without external pay in standalone mode")
			hasSwaps(t, entity, 0)
		})

		t.Run("Read", func(t *testing.T) {
			hasSwaps(t, readEntity, 0)
			_, err = readEntity.CreateSwap(&boltzrpc.CreateSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readEntity.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readEntity.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			hasSwaps(t, readEntity, 0)
		})
	})
}

func TestGetSwapInfoStream(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
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
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	info, err := client.GetPairs()

	require.NoError(t, err)
	require.Len(t, info.Submarine, 2)
	require.Len(t, info.Reverse, 2)
}

func checkTxOutAddress(t *testing.T, chain *onchain.Onchain, currency boltz.Currency, txId string, outAddress string, cooperative bool) {
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
	chain := getOnchain(t, cfg)

	checkSwap := func(t *testing.T, swap *boltzrpc.SwapInfo) {
		invoice, err := zpay32.Decode(swap.Invoice, &chaincfg.RegressionNetParams)
		require.NoError(t, err)

		excpectedFees := swap.ExpectedAmount - uint64(invoice.MilliSat.ToSatoshis())
		actualFees := *swap.OnchainFee + *swap.ServiceFee
		if swap.WalletId != nil {
			lockupFee, err := chain.GetTransactionFee(parseCurrency(swap.Pair.From), swap.LockupTransactionId)
			require.NoError(t, err)

			excpectedFees += lockupFee
		}

		require.Equal(t, excpectedFees, actualFees)
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
		client, _, stop := setup(t, setupOptions{cfg: cfg})
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
			_, err := connectLightning(node)
			require.NoError(t, err)
			invoice, err := node.CreateInvoice(100000, nil, 0, "test")
			require.NoError(t, err)
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Invoice:          &invoice.PaymentRequest,
				SendFromInternal: true,
			})
			require.NoError(t, err)

			stream, _ := swapStream(t, client, swap.Id)
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
		client, _, stop := setup(t, setupOptions{cfg: cfg})
		node := cfg.LND
		_, err := connectLightning(node)
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
				{"BTC", boltzrpc.Currency_BTC, test.BtcCli},
				{"Liquid", boltzrpc.Currency_LBTC, test.LiquidCli},
			}

			for _, tc := range tests {
				t.Run(tc.desc, func(t *testing.T) {
					boltzApi := getBoltz(t, cfg)
					cfg.Node = "LND"
					pair := &boltzrpc.Pair{
						From: tc.from,
						To:   boltzrpc.Currency_BTC,
					}
					client, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi})
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

						stream, _ := swapStream(t, client, swap.Id)
						test.MineBlock()

						info := stream(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})
					t.Run("Deposit", func(t *testing.T) {
						swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
							Pair: pair,
						})
						require.NoError(t, err)

						test.SendToAddress(tc.cli, swap.Address, 100000)
						test.MineBlock()

						stream, _ := swapStream(t, client, swap.Id)
						info := stream(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

					if node == "CLN" {
						return
					}

					t.Run("Refund", func(t *testing.T) {
						cli := tc.cli

						submarinePair, err := client.GetPairInfo(boltzrpc.SwapType_SUBMARINE, pair)

						require.NoError(t, err)

						createFailedSwap := func(t *testing.T, refundAddress string) streamFunc {
							amount := submarinePair.Limits.Minimal + 100
							swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
								Pair:          pair,
								RefundAddress: &refundAddress,
								Amount:        amount + 100,
							})
							require.NoError(t, err)

							stream, _ := swapStream(t, client, swap.Id)
							test.SendToAddress(cli, swap.Address, amount)
							return stream
						}

						t.Run("Script", func(t *testing.T) {
							boltzApi.DisablePartialSignatures = true
							t.Cleanup(func() {
								boltzApi.DisablePartialSignatures = false
							})

							refundAddress := cli("getnewaddress")
							withStream := createFailedSwap(t, refundAddress)

							swap := withStream(boltzrpc.SwapState_ERROR).Swap

							test.MineUntil(t, cli, int64(swap.TimeoutBlockHeight))

							swap = withStream(boltzrpc.SwapState_REFUNDED).Swap

							from := parseCurrency(pair.From)

							refundFee, err := chain.GetTransactionFee(from, swap.RefundTransactionId)
							require.NoError(t, err)

							require.Equal(t, int(refundFee), int(*swap.OnchainFee))

							checkTxOutAddress(t, chain, from, swap.RefundTransactionId, refundAddress, false)
						})

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

						if tc.from == boltzrpc.Currency_LBTC {
							t.Run("Manual", func(t *testing.T) {
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
					boltzApi := getBoltz(t, cfg)
					boltzApi.DisablePartialSignatures = tc.disablePartials
					cfg.Node = node
					chain := getOnchain(t, cfg)
					client, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi, chain: chain})

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
							_, statusStream := swapStream(t, client, swap.Id)
							statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)

							if !tc.zeroConf {
								test.MineBlock()
							}

							info = statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.InvoiceSettled)
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

	t.Run("Invalid", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		originalTx := chain.Btc.Tx

		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		tests := []struct {
			desc     string
			txMocker txMocker
			error    string
		}{
			{"LessValue", lessValueTxProvider, "locked up less"},
			{"Unconfirmed", unconfirmedTxProvider, "not confirmed"},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				chain.Btc.Tx = tc.txMocker(t, originalTx)
				swap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
					Amount:         100000,
					AcceptZeroConf: false,
				})
				require.NoError(t, err)

				_, statusStream := swapStream(t, client, swap.Id)
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
				test.MineBlock()
				info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionMempool)
				require.Contains(t, info.ReverseSwap.Error, tc.error)
			})
		}
	})

	t.Run("Standalone", func(t *testing.T) {
		cfg := loadConfig(t)
		cfg.Standalone = true
		lnd := cfg.LND
		_, err := connectLightning(lnd)
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

		_, err = lnd.PayInvoice(context.Background(), *swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream, _ := swapStream(t, client, swap.Id)
		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)

		require.Equal(t, info.ReverseSwap.State, boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)
	})

	t.Run("ExternalPay", func(t *testing.T) {
		cfg := loadConfig(t)
		client, _, stop := setup(t, setupOptions{cfg: cfg, node: "lnd"})
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

		stream, _ := swapStream(t, client, swap.Id)

		_, err = cfg.LND.PayInvoice(context.Background(), *swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)

		test.MineBlock()
	})

}
func walletId(t *testing.T, client client.Boltz, currency boltzrpc.Currency) uint64 {
	wallets, err := client.GetWallets(&currency, false)
	require.NoError(t, err)
	require.NotEmpty(t, wallets.Wallets)
	return wallets.Wallets[0].Id
}

func TestChainSwap(t *testing.T) {

	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)

	tests := []struct {
		desc string
		from boltzrpc.Currency
		to   boltzrpc.Currency
	}{
		{"BTC", boltzrpc.Currency_BTC, boltzrpc.Currency_LBTC},
		{"Liquid", boltzrpc.Currency_LBTC, boltzrpc.Currency_BTC},
	}

	t.Run("Recovery", func(t *testing.T) {
		cfg := loadConfig(t)
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})

		externalPay := true
		to := test.LiquidCli("getnewaddress")
		swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
			Amount:      100000,
			Pair:        &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC},
			ExternalPay: &externalPay,
			ToAddress:   &to,
		})
		require.NoError(t, err)
		stop()

		test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, swap.FromData.Amount)
		test.MineBlock()

		client, _, stop = setup(t, setupOptions{cfg: cfg})
		defer stop()

		stream, _ := swapStream(t, client, "")
		update := stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
		require.Equal(t, swap.Id, update.Id)
		checkTxOutAddress(t, chain, boltz.CurrencyLiquid, update.ToData.GetTransactionId(), update.ToData.GetAddress(), true)
	})

	t.Run("Invalid", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		originalTx := chain.Btc.Tx
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		tests := []struct {
			desc     string
			txMocker txMocker
			error    string
		}{
			{"LessValue", lessValueTxProvider, "locked up less"},
			{"Unconfirmed", unconfirmedTxProvider, "not confirmed"},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				chain.Btc.Tx = tc.txMocker(t, originalTx)

				externalPay := true
				toWalletId := walletId(t, client, boltzrpc.Currency_BTC)
				swap, err := client.CreateChainSwap(
					&boltzrpc.CreateChainSwapRequest{
						Amount:      100000,
						ExternalPay: &externalPay,
						ToWalletId:  &toWalletId,
						Pair: &boltzrpc.Pair{
							From: boltzrpc.Currency_LBTC,
							To:   boltzrpc.Currency_BTC,
						},
					},
				)
				require.NoError(t, err)

				test.SendToAddress(test.LiquidCli, swap.FromData.LockupAddress, swap.FromData.Amount)
				test.MineBlock()

				_, statusStream := swapStream(t, client, swap.Id)
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)
				test.MineBlock()
				info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionServerMempoool)
				require.Contains(t, info.ChainSwap.Error, tc.error)
			})
		}
	})

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			pair := &boltzrpc.Pair{
				From: tc.from,
				To:   tc.to,
			}
			cfg := loadConfig(t)
			boltzApi := getBoltz(t, cfg)
			client, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi})
			defer stop()

			fromCli := getCli(tc.from)
			toCli := getCli(tc.to)

			refundAddress := fromCli("getnewaddress")
			toAddress := toCli("getnewaddress")

			fromWalletId := walletId(t, client, tc.from)
			toWalletId := walletId(t, client, tc.to)

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

				stream, _ := swapStream(t, client, swap.Id)
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

				_, streamStatus := swapStream(t, client, swap.Id)
				test.SendToAddress(fromCli, swap.FromData.LockupAddress, swap.FromData.Amount)
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
				test.MineBlock()
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionConfirmed)
				streamStatus(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)
				test.MineBlock()
				info := streamStatus(boltzrpc.SwapState_SUCCESSFUL, boltz.TransactionClaimed).ChainSwap

				to := parseCurrency(tc.to)
				checkTxOutAddress(t, chain, to, info.ToData.GetTransactionId(), info.ToData.GetAddress(), true)

				checkSwap(t, swap.Id)
			})

			t.Run("Refund", func(t *testing.T) {
				chainPair, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, pair)

				require.NoError(t, err)

				createFailed := func(t *testing.T, refundAddress string) (streamFunc, streamStatusFunc) {
					amount := chainPair.Limits.Minimal + 100
					externalPay := true
					swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Pair:          pair,
						RefundAddress: &refundAddress,
						ToAddress:     &toAddress,
						Amount:        amount + 100,
						ExternalPay:   &externalPay,
					})
					require.NoError(t, err)

					test.SendToAddress(fromCli, swap.FromData.LockupAddress, amount)
					return swapStream(t, client, swap.Id)
				}

				t.Run("Script", func(t *testing.T) {
					boltzApi.DisablePartialSignatures = true

					stream, statusStream := createFailed(t, refundAddress)
					info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).ChainSwap
					test.MineUntil(t, fromCli, int64(info.FromData.TimeoutBlockHeight))
					info = stream(boltzrpc.SwapState_REFUNDED).ChainSwap

					from := parseCurrency(pair.From)
					refundFee, err := chain.GetTransactionFee(from, info.FromData.GetTransactionId())
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

					refundFee, err := chain.GetTransactionFee(from, info.FromData.GetTransactionId())
					require.NoError(t, err)
					require.Equal(t, refundFee, *info.OnchainFee)

					checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)
				})

				if tc.from == boltzrpc.Currency_LBTC {
					t.Run("Manual", func(t *testing.T) {
						_, statusStream := createFailed(t, "")
						info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).ChainSwap
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

							info = statusStream(boltzrpc.SwapState_REFUNDED, boltz.TransactionLockupFailed).ChainSwap
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
				}
			})
		})
	}
}

func TestAutoSwap(t *testing.T) {

	cfg := loadConfig(t)
	cfg.Node = "cln"

	_, err := connectLightning(cfg.Cln)
	require.NoError(t, err)
	_, err = connectLightning(cfg.LND)
	require.NoError(t, err)

	var us, them lightning.LightningNode
	if strings.EqualFold(cfg.Node, "lnd") {
		us = cfg.LND
		them = cfg.Cln
	} else {
		us = cfg.Cln
		them = cfg.LND
	}

	client, autoSwap, stop := setup(t, setupOptions{cfg: cfg})
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

		status := running(false)
		require.NotEmpty(t, status.Error)

		_, err = autoSwap.SetConfigValue("wallet", walletName)
		require.NoError(t, err)

		status = running(true)
		require.Empty(t, status.Error)

		_, err = autoSwap.Disable()
		require.NoError(t, err)

		running(false)
	})

	t.Run("CantRemoveWallet", func(t *testing.T) {
		_, err := autoSwap.SetConfigValue("wallet", walletName)
		require.NoError(t, err)
		_, err = client.RemoveWallet(testWallet.Id)
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

				response, err := to.CreateInvoice(amount, nil, 100000, "Testt")
				require.NoError(t, err)
				_, err = from.PayInvoice(context.Background(), response.PaymentRequest, 10000, 30, []lightning.ChanId{channel.Id})
				require.NoError(t, err)

				time.Sleep(1000 * time.Millisecond)
			}

			swapCfg := autoswap.DefaultConfig()
			swapCfg.AcceptZeroConf = true
			swapCfg.MaxFeePercent = 10
			swapCfg.Budget = 1000000
			swapCfg.Currency = tc.currency
			swapCfg.SwapType = ""
			swapCfg.Wallet = strings.ToUpper(cfg.Node)

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
					stream, _ := swapStream(t, client, swaps.ReverseSwaps[0].Id)
					stream(boltzrpc.SwapState_SUCCESSFUL)

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
						_, autoSwap, stop := setup(t, setupOptions{cfg: cfg})
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
						client, _, stop := setup(t, setupOptions{cfg: cfg})
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
	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	// the main setup function already created a wallet
	_, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	walletParams := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "test", Password: &password}

	credentials, err := client.CreateWallet(walletParams)
	require.NoError(t, err)

	_, err = client.GetWallet(walletParams.Name)
	require.NoError(t, err)

	_, err = client.RemoveWallet(credentials.Wallet.Id)
	require.NoError(t, err)

	mnemonic := "invalid"
	_, err = client.ImportWallet(walletParams, &boltzrpc.WalletCredentials{Mnemonic: &mnemonic})
	require.Error(t, err)

	_, err = client.GetWallet(walletParams.Name)
	require.Error(t, err)

	_, err = client.ImportWallet(walletParams, &boltzrpc.WalletCredentials{Mnemonic: &credentials.Mnemonic})
	require.NoError(t, err)

	/*
		_, err = client.GetWallet(info)
		require.Error(t, err)

	*/

	_, err = client.SetSubaccount(testWallet.Id, nil)
	require.NoError(t, err)

	_, err = client.GetWallet(walletName)
	require.NoError(t, err)
}

func TestUnlock(t *testing.T) {
	password := "password"
	client, _, stop := setup(t, setupOptions{password: password})
	defer stop()

	_, err := client.GetInfo()
	require.Error(t, err)

	require.Error(t, client.Unlock("wrong"))
	require.NoError(t, client.Unlock(password))

	test.MineBlock()
	_, err = client.GetInfo()
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(testWallet.Id, nil)
	require.Error(t, err)

	c, err := client.GetWalletCredentials(testWallet.Id, &password)
	require.NoError(t, err)
	require.Equal(t, credentials.Mnemonic, *c.Mnemonic)

	wrongPassword := "wrong"
	second := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "new", Password: &wrongPassword}
	_, err = client.CreateWallet(second)
	require.Error(t, err)

	second.Password = &password
	_, err = client.CreateWallet(second)
	require.NoError(t, err)
}

func TestMagicRoutingHints(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	addr := test.BtcCli("getnewaddress")
	pair := &boltzrpc.Pair{
		From: boltzrpc.Currency_BTC,
		To:   boltzrpc.Currency_BTC,
	}
	externalPay := true
	var amount uint64 = 100000
	reverseSwap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
		Amount:      amount,
		Address:     addr,
		Pair:        pair,
		ExternalPay: &externalPay,
	})
	require.NoError(t, err)

	swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
		Pair:             pair,
		Invoice:          reverseSwap.Invoice,
		SendFromInternal: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, swap.Bip21)
	require.NotEmpty(t, swap.TxId)
	require.NotZero(t, swap.ExpectedAmount)
	require.Equal(t, addr, swap.Address)
	require.Empty(t, swap.Id)
}

func TestCreateWalletWithPassword(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	_, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	secondParams := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_BTC, Password: &password}
	secondWallet, err := client.CreateWallet(secondParams)
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(testWallet.Id, nil)
	require.Error(t, err)

	_, err = client.GetWalletCredentials(testWallet.Id, &password)
	require.NoError(t, err)

	_, err = client.RemoveWallet(secondWallet.Wallet.Id)
	require.NoError(t, err)

}

func TestImportDuplicateCredentials(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	credentials, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted aswell
	second := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_BTC}
	_, err = client.ImportWallet(second, credentials)
	require.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	cfg := loadConfig(t)
	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

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
