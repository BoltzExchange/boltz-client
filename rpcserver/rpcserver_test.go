//go:build !unit

package rpcserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
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
	boltzApi := getBoltz(t, cfg)
	chain, err := initOnchain(cfg, boltzApi, boltz.Regtest)
	require.NoError(t, err)
	return chain
}

var walletName = "regtest"
var password = "password"
var walletParams = &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: walletName}
var walletCredentials *wallet.Credentials
var testWallet *database.Wallet

type setupOptions struct {
	cfg      *config.Config
	password string
	chain    *onchain.Onchain
	boltzApi *boltz.Api
	node     string
}

func waitForSync(t *testing.T, client client.Boltz) {
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.After(1 * time.Second)
	for {
		var err error
		select {
		case <-ticker.C:
			_, err = client.GetInfo()
		case <-timeout:
			require.Fail(t, "timed out while waiting for daemon to sync")
		}
		if err == nil || strings.Contains(err.Error(), "locked") {
			break
		}
	}

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
	if walletCredentials == nil {
		var wallet *wallet.Wallet
		wallet, walletCredentials, err = test.InitTestWallet(parseCurrency(walletParams.Currency), false)
		require.NoError(t, err)
		walletCredentials.Name = walletName
		walletCredentials.TenantId = database.DefaultTenantId
		require.NoError(t, wallet.Disconnect())
	}

	encrytpedCredentials := walletCredentials
	if options.password != "" {
		encrytpedCredentials, err = walletCredentials.Encrypt(password)
		require.NoError(t, err)
	}
	testWallet = &database.Wallet{Credentials: encrytpedCredentials}

	require.NoError(t, cfg.Database.Connect())
	_, err = cfg.Database.GetWallet(encrytpedCredentials.Id)
	if err != nil {
		require.NoError(t, cfg.Database.CreateWallet(testWallet))
	} else {
		require.NoError(t, cfg.Database.UpdateWalletCredentials(encrytpedCredentials))
	}

	rpc := NewRpcServer(cfg)
	require.NoError(t, rpc.Init())
	rpc.boltzServer.boltz = options.boltzApi
	rpc.boltzServer.onchain = options.chain
	rpc.boltzServer.lightning = options.lightning
	go func() {
		require.NoError(t, rpc.boltzServer.start(cfg))
	}()

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
		if err := rpc.grpc.Serve(lis); err != nil {
			logger.Error("error connecting serving server: " + err.Error())
		}
	}()

	go func() {
		<-rpc.boltzServer.stop
		require.NoError(t, lis.Close())
		rpc.grpc.GracefulStop()
	}()

	clientConn := client.Connection{ClientConn: conn}
	macaroonFile, err := os.ReadFile("./test/macaroons/admin.macaroon")
	require.NoError(t, err)
	clientConn.SetMacaroon(hex.EncodeToString(macaroonFile))

	boltzClient := client.NewBoltzClient(clientConn)
	autoSwapClient := client.NewAutoSwapClient(clientConn)

	waitForSync(t, boltzClient)

	return boltzClient, autoSwapClient, func() {
		_, err = autoSwapClient.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)
		_, err = autoSwapClient.ResetConfig(client.ChainAutoSwap)
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

func createTenant(t *testing.T, admin client.Boltz, name string) (*boltzrpc.Tenant, client.Connection, client.Connection) {
	tenantInfo, err := admin.CreateTenant(name)
	require.NoError(t, err)
	require.NotZero(t, tenantInfo.Id)

	write, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
		TenantId:    &tenantInfo.Id,
		Permissions: client.FullPermissions,
	})
	require.NoError(t, err)

	read, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
		TenantId:    &tenantInfo.Id,
		Permissions: client.ReadPermissions,
	})
	require.NoError(t, err)

	readConn := admin.Connection
	readConn.SetMacaroon(read.Macaroon)

	writeConn := admin.Connection
	writeConn.SetMacaroon(write.Macaroon)

	return tenantInfo, writeConn, readConn
}

func TestMacaroons(t *testing.T) {
	admin, adminAuto, stop := setup(t, setupOptions{})
	defer stop()
	conn := admin.Connection

	tenantName := "test"

	tenantInfo, write, read := createTenant(t, admin, tenantName)

	list, err := admin.ListTenants()
	require.NoError(t, err)
	require.Len(t, list.Tenants, 2)

	tenant := client.NewBoltzClient(write)
	tenantAuto := client.NewAutoSwapClient(write)
	readTenant := client.NewBoltzClient(read)

	t.Run("Bake", func(t *testing.T) {
		response, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.FullPermissions,
		})
		require.NoError(t, err)

		anotherAdmin := client.NewBoltzClient(conn)

		anotherAdmin.SetMacaroon(response.Macaroon)

		response, err = admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.ReadPermissions,
		})
		require.NoError(t, err)

		anotherAdmin.SetMacaroon(response.Macaroon)

		// write actions are not allowed now
		_, err = anotherAdmin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.ReadPermissions,
		})
		require.Error(t, err)

		err = anotherAdmin.Stop()
		require.Error(t, err)
	})

	t.Run("Admin", func(t *testing.T) {
		_, err = tenant.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			TenantId:    &tenantInfo.Id,
			Permissions: client.ReadPermissions,
		})
		require.Error(t, err)

		_, err = tenant.GetTenant(tenantInfo.Name)
		require.Error(t, err)

		_, err = tenant.ListTenants()
		require.Error(t, err)

		_, err = admin.GetTenant(tenantInfo.Name)
		require.NoError(t, err)
	})

	t.Run("Info", func(t *testing.T) {
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.NotNil(t, info.Tenant)

		info, err = readTenant.GetInfo()
		require.NoError(t, err)
		require.Empty(t, info.NodePubkey)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)
	})

	t.Run("AutoSwap", func(t *testing.T) {
		_, err := adminAuto.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)
		cfg, err := adminAuto.GetLightningConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		_, err = tenantAuto.GetLightningConfig()
		require.Error(t, err)
	})

	t.Run("Wallet", func(t *testing.T) {
		hasWallets := func(t *testing.T, client client.Boltz, amount int) {
			wallets, err := client.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, amount)
		}
		hasWallets(t, tenant, 0)
		hasWallets(t, admin, 2)

		_, err = tenant.GetWallet(walletName)
		requireCode(t, err, codes.NotFound)

		_, err = readTenant.CreateWallet(walletParams)
		requireCode(t, err, codes.PermissionDenied)

		_, err = tenant.CreateWallet(walletParams)
		requireCode(t, err, codes.OK)

		hasWallets(t, tenant, 1)
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
			hasSwaps(t, tenant, 0)
		})

		t.Run("Tenant", func(t *testing.T) {
			hasSwaps(t, tenant, 0)
			_, err = tenant.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.ErrorContains(t, err, "invoice is required in standalone mode")
			externalPay := false
			_, err = tenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
			require.ErrorContains(t, err, "can not create reverse swap without external pay in standalone mode")
			hasSwaps(t, tenant, 0)
		})

		t.Run("Read", func(t *testing.T) {
			hasSwaps(t, readTenant, 0)
			_, err = readTenant.CreateSwap(&boltzrpc.CreateSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			hasSwaps(t, readTenant, 0)
		})
	})
}

func TestGetSwapInfoStream(t *testing.T) {
	admin, _, stop := setup(t, setupOptions{})
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

	t.Run("Normal", func(t *testing.T) {
		swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{})
		require.NoError(t, err)

		info := stream(boltzrpc.SwapState_PENDING)
		require.Equal(t, swap.Id, info.Swap.Id)

		check(t, swap.Id)
	})

	t.Run("Reverse", func(t *testing.T) {
		reverseSwap, err := admin.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
		require.NoError(t, err)

		info := stream(boltzrpc.SwapState_PENDING)
		require.Equal(t, reverseSwap.Id, info.ReverseSwap.Id)

		check(t, reverseSwap.Id)
	})

	t.Run("Chain", func(t *testing.T) {
		externalPay := true
		chainSwap, err := admin.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
			Amount:      100000,
			Pair:        &boltzrpc.Pair{From: boltzrpc.Currency_BTC, To: boltzrpc.Currency_LBTC},
			ExternalPay: &externalPay,
			ToWalletId:  &testWallet.Id,
		})
		require.NoError(t, err)

		info := stream(boltzrpc.SwapState_PENDING)
		require.Equal(t, chainSwap.Id, info.ChainSwap.Id)

		check(t, chainSwap.Id)
	})
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
		cfg.Node = "Cln"
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
					cfg := loadConfig(t)
					boltzApi := getBoltz(t, cfg)
					cfg.Node = "LND"
					pair := &boltzrpc.Pair{
						From: tc.from,
						To:   boltzrpc.Currency_BTC,
					}
					admin, _, stop := setup(t, setupOptions{cfg: cfg, boltzApi: boltzApi})
					defer stop()

					_, write, _ := createTenant(t, admin, "test")
					tenant := client.NewBoltzClient(write)

					t.Run("Normal", func(t *testing.T) {
						t.Run("EnoughBalance", func(t *testing.T) {

							swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
								Amount:           100000,
								Pair:             pair,
								SendFromInternal: true,
							})
							require.NoError(t, err)
							require.NotEmpty(t, swap.TxId)
							require.NotZero(t, swap.TimeoutHours)
							require.NotZero(t, swap.TimeoutBlockHeight)

							stream, _ := swapStream(t, admin, swap.Id)
							test.MineBlock()

							info := stream(boltzrpc.SwapState_SUCCESSFUL)
							checkSwap(t, info.Swap)
						})

						t.Run("NoBalance", func(t *testing.T) {
							emptyWalletId := emptyWallet(t, admin, tc.from)
							_, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
								Amount:           100000,
								Pair:             pair,
								SendFromInternal: true,
								WalletId:         &emptyWalletId,
							})
							require.Error(t, err)
						})
					})
					t.Run("Deposit", func(t *testing.T) {
						swap, err := admin.CreateSwap(&boltzrpc.CreateSwapRequest{
							Pair: pair,
						})
						require.NoError(t, err)

						test.SendToAddress(tc.cli, swap.Address, 100000)
						test.MineBlock()

						stream, _ := swapStream(t, admin, swap.Id)
						info := stream(boltzrpc.SwapState_SUCCESSFUL)
						checkSwap(t, info.Swap)
					})

					if node == "CLN" {
						return
					}

					t.Run("Refund", func(t *testing.T) {
						cli := tc.cli

						submarinePair, err := admin.GetPairInfo(boltzrpc.SwapType_SUBMARINE, pair)

						require.NoError(t, err)

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

							refundFee, err := chain.GetTransactionFee(from, swap.RefundTransactionId)
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

							refundFee, err := chain.GetTransactionFee(from, info.RefundTransactionId)
							require.NoError(t, err)
							require.Equal(t, int(refundFee), int(*info.OnchainFee))

							checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)
						})

						if tc.from == boltzrpc.Currency_BTC {
							t.Run("Manual", func(t *testing.T) {
								setup := func(t *testing.T) *boltzrpc.SwapInfo {
									_, statusStream := createFailed(t, "")
									info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).Swap
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
										refundFee, err := chain.GetTransactionFee(from, info.RefundTransactionId)
										require.NoError(t, err)
										assert.Equal(t, int(refundFee), int(*info.OnchainFee))

										checkTxOutAddress(t, chain, from, info.RefundTransactionId, refundAddress, true)

										_, err = admin.RefundSwap(request)
										requireCode(t, err, codes.NotFound)
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
					require.NotNil(t, info.ReverseSwap.PaidAt)
					require.LessOrEqual(t, *info.ReverseSwap.PaidAt, time.Now().Unix())
					require.GreaterOrEqual(t, *info.ReverseSwap.PaidAt, info.ReverseSwap.CreatedAt)

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
		require.NoError(t, cfg.Cln.Connect())
		client, _, stop := setup(t, setupOptions{cfg: cfg, node: "lnd"})
		defer stop()

		externalPay := true
		returnImmediately := false
		description := "test"

		request := &boltzrpc.CreateReverseSwapRequest{
			Amount:            100000,
			AcceptZeroConf:    true,
			ExternalPay:       &externalPay,
			ReturnImmediately: &returnImmediately,
			Description:       &description,
		}

		// cant wait for claim transaction if paid externally
		_, err := client.CreateReverseSwap(request)
		require.Error(t, err)

		request.ReturnImmediately = nil

		swap, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		require.NotEmpty(t, swap.Invoice)

		decoded, err := zpay32.Decode(*swap.Invoice, &chaincfg.RegressionNetParams)
		require.NoError(t, err)

		require.Equal(t, btcutil.Amount(request.Amount), decoded.MilliSat.ToSatoshis())
		require.Equal(t, description, *decoded.Description)

		stream, _ := swapStream(t, client, swap.Id)

		_, err = cfg.Cln.PayInvoice(context.Background(), *swap.Invoice, 10000, 30, nil)
		require.NoError(t, err)

		stream(boltzrpc.SwapState_PENDING)
		info := stream(boltzrpc.SwapState_SUCCESSFUL)
		require.True(t, info.ReverseSwap.ExternalPay)
		require.Nil(t, info.ReverseSwap.RoutingFeeMsat)

		test.MineBlock()
	})

	t.Run("ManualClaim", func(t *testing.T) {
		cfg := loadConfig(t)
		chain := getOnchain(t, cfg)
		client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
		defer stop()

		createClaimable := func(t *testing.T) (*boltzrpc.ReverseSwapInfo, streamFunc, streamStatusFunc) {
			returnImmediately := false

			response, err := client.CreateWallet(&boltzrpc.WalletParams{
				Currency: boltzrpc.Currency_BTC,
				Name:     "temp",
			})
			require.NoError(t, err)
			swap, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
				Amount:            100000,
				ReturnImmediately: &returnImmediately,
				WalletId:          &response.Wallet.Id,
			})
			require.NoError(t, err)

			stream, statusStream := swapStream(t, client, swap.Id)
			statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)

			_, err = client.RemoveWallet(response.Wallet.Id)
			require.NoError(t, err)

			test.MineBlock()

			info, err := client.GetInfo()
			require.NoError(t, err)
			require.Len(t, info.ClaimableSwaps, 1)
			require.Equal(t, info.ClaimableSwaps[0], swap.Id)

			return stream(boltzrpc.SwapState_ERROR).ReverseSwap, stream, statusStream
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
				destination.WalletId = walletId(t, client, boltzrpc.Currency_BTC)
				_, err := client.ClaimSwaps(request)
				require.NoError(t, err)

				fromWallet, err := client.GetWalletById(destination.WalletId)
				require.NoError(t, err)
				require.NotZero(t, fromWallet.Balance.Unconfirmed)

				_, err = client.ClaimSwaps(request)
				requireCode(t, err, codes.NotFound)
			})
		})
	})

}

func walletId(t *testing.T, client client.Boltz, currency boltzrpc.Currency) uint64 {
	wallets, err := client.GetWallets(&currency, false)
	require.NoError(t, err)
	require.NotEmpty(t, wallets.Wallets)
	return wallets.Wallets[0].Id
}

func emptyWallet(t *testing.T, client client.Boltz, currency boltzrpc.Currency) uint64 {
	response, err := client.CreateWallet(&boltzrpc.WalletParams{
		Currency: currency,
		Name:     "empty",
	})
	if err != nil {
		existing, err := client.GetWallet("empty")
		require.NoError(t, err)
		return existing.Id
	}
	return response.Wallet.Id
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
				t.Run("EnoughBalance", func(t *testing.T) {
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

				t.Run("NoBalance", func(t *testing.T) {
					emptyWalletId := emptyWallet(t, client, tc.from)
					_, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
						Amount:       100000,
						Pair:         pair,
						ToWalletId:   &toWalletId,
						FromWalletId: &emptyWalletId,
					})
					require.Error(t, err)
				})
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

				if tc.from == boltzrpc.Currency_BTC {

					t.Run("Manual", func(t *testing.T) {
						setup := func(t *testing.T) (*boltzrpc.ChainSwapInfo, streamStatusFunc) {
							_, statusStream := createFailed(t, "")
							info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).ChainSwap
							clientInfo, err := client.GetInfo()
							require.NoError(t, err)
							require.Len(t, clientInfo.RefundableSwaps, 1)
							require.Equal(t, clientInfo.RefundableSwaps[0], info.Id)
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

								refundFee, err := chain.GetTransactionFee(from, info.FromData.GetTransactionId())
								require.NoError(t, err)
								assert.Equal(t, int(refundFee), int(*info.OnchainFee))

								checkTxOutAddress(t, chain, from, info.FromData.GetTransactionId(), refundAddress, true)

								_, err = client.RefundSwap(request)
								requireCode(t, err, codes.NotFound)
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
								destination.WalletId = fromWalletId
								_, err := client.RefundSwap(request)
								require.NoError(t, err)

								info = statusStream(boltzrpc.SwapState_REFUNDED, boltz.TransactionLockupFailed).ChainSwap
								require.Zero(t, info.ServiceFee)

								fromWallet, err := client.GetWalletById(fromWalletId)
								require.NoError(t, err)
								require.NotZero(t, fromWallet.Balance.Unconfirmed)

								_, err = client.RefundSwap(request)
								requireCode(t, err, codes.NotFound)
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
							Amount:      100000,
						})
						require.NoError(t, err)

						test.SendToAddress(fromCli, swap.FromData.LockupAddress, swap.FromData.Amount)
						test.MineBlock()

						_, err = client.RemoveWallet(response.Wallet.Id)
						require.NoError(t, err)

						stream, statusStream := swapStream(t, client, swap.Id)
						statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionServerMempoool)

						test.MineBlock()

						info, err := client.GetInfo()
						require.NoError(t, err)
						require.Len(t, info.ClaimableSwaps, 1)
						require.Equal(t, info.ClaimableSwaps[0], swap.Id)

						return stream(boltzrpc.SwapState_ERROR).ChainSwap, stream, statusStream
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
							checkSwap(t, info.Id)

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
							destination.WalletId = toWalletId
							_, err := client.ClaimSwaps(request)
							require.NoError(t, err)

							info = stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
							checkSwap(t, info.Id)

							fromWallet, err := client.GetWalletById(toWalletId)
							require.NoError(t, err)
							require.NotZero(t, fromWallet.Balance.Unconfirmed)

							_, err = client.ClaimSwaps(request)
							requireCode(t, err, codes.NotFound)
						})
					})
				})
			}
		})
	}
}

func TestAutoSwap(t *testing.T) {
	cfg := loadConfig(t)
	cfg.Node = "LND"

	_, err := connectLightning(cfg.Cln)
	require.NoError(t, err)
	_, err = connectLightning(cfg.LND)
	require.NoError(t, err)

	admin, autoSwap, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	_, err = autoSwap.ResetConfig(client.LnAutoSwap)
	require.NoError(t, err)

	_, err = autoSwap.ResetConfig(client.ChainAutoSwap)
	require.NoError(t, err)

	t.Run("Chain", func(t *testing.T) {
		cfg := &autoswaprpc.ChainConfig{
			FromWallet: walletName,
			ToWallet:   cfg.Node,
			Enabled:    true,
		}
		fromWallet, err := admin.GetWallet(cfg.FromWallet)
		require.NoError(t, err)
		cfg.MaxBalance = fromWallet.Balance.Confirmed - 1000

		_, err = autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: cfg})
		require.NoError(t, err)

		status, err := autoSwap.GetStatus()
		require.NoError(t, err)
		require.True(t, status.Chain.Running)

		recommendations, err := autoSwap.GetRecommendations()
		require.NoError(t, err)
		require.Len(t, recommendations.Chain, 1)

		test.MineBlock()
		stream, _ := swapStream(t, admin, "")
		info := stream(boltzrpc.SwapState_PENDING)
		require.NotNil(t, info.ChainSwap)
		id := info.ChainSwap.Id

		recommendations, err = autoSwap.GetRecommendations()
		require.NoError(t, err)
		require.NotEmpty(t, recommendations.Chain[0].DismissedReasons)

		isAuto := true
		response, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{IsAuto: &isAuto})
		require.Len(t, response.ChainSwaps, 1)
		require.Equal(t, id, response.ChainSwaps[0].Id)

		stream, _ = swapStream(t, admin, id)
		require.NoError(t, err)
		test.MineBlock()
		stream(boltzrpc.SwapState_PENDING)
		test.MineBlock()
		stream(boltzrpc.SwapState_SUCCESSFUL)

		recommendations, err = autoSwap.GetRecommendations()
		require.NoError(t, err)
		require.NotEmpty(t, recommendations.Chain[0].DismissedReasons)

		_, write, _ := createTenant(t, admin, "test")
		tenant := client.NewAutoSwapClient(write)

		_, err = tenant.GetConfig()
		require.Error(t, err)
	})

	t.Run("Lightning", func(t *testing.T) {
		_, err := autoSwap.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)

		var us, them lightning.LightningNode
		if strings.EqualFold(cfg.Node, "lnd") {
			us = cfg.LND
			them = cfg.Cln
		} else {
			us = cfg.Cln
			them = cfg.LND
		}

		t.Run("Setup", func(t *testing.T) {
			running := func(value bool) *autoswaprpc.GetStatusResponse {
				status, err := autoSwap.GetStatus()
				require.NoError(t, err)
				require.Equal(t, value, status.Lightning.Running)
				require.NotEmpty(t, status.Lightning.Description)
				return status
			}

			running(false)

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{
				Config: &autoswaprpc.LightningConfig{
					Currency: boltzrpc.Currency_LBTC,
					Wallet:   walletName,
				},
				FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"currency", "wallet"}},
			})
			require.NoError(t, err)

			_, err = autoSwap.Enable()
			require.NoError(t, err)

			status := running(true)
			require.Empty(t, status.Lightning.Error)

			_, err = autoSwap.SetLightningConfigValue("wallet", "invalid")
			require.Error(t, err)
		})

		t.Run("CantRemoveWallet", func(t *testing.T) {
			_, err := autoSwap.SetLightningConfigValue("wallet", walletName)
			require.NoError(t, err)
			_, err = autoSwap.SetLightningConfigValue("enabled", true)
			require.NoError(t, err)
			_, err = admin.RemoveWallet(testWallet.Id)
			require.Error(t, err)
		})

		t.Run("Start", func(t *testing.T) {
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

				if channel.InboundSat < channel.Capacity/2 {
					amount = amount + (channel.Capacity/2-channel.InboundSat)/1000
				}
				if channel.OutboundSat < amount {
					return
				}

				response, err := to.CreateInvoice(amount, nil, 100000, "Testt")
				require.NoError(t, err)
				_, err = from.PayInvoice(context.Background(), response.PaymentRequest, 10000, 30, []lightning.ChanId{channel.Id})
				require.NoError(t, err)

				time.Sleep(1000 * time.Millisecond)
			}

			channels, err := us.ListChannels()
			require.NoError(t, err)
			var inboundBalance, outboundBalance uint64
			for _, channel := range channels {
				inboundBalance += channel.InboundSat
				outboundBalance += channel.OutboundSat
			}

			swapCfg := autoswap.DefaultLightningConfig()
			swapCfg.AcceptZeroConf = true
			swapCfg.MaxFeePercent = 10
			swapCfg.Currency = boltzrpc.Currency_BTC
			swapCfg.InboundBalance = inboundBalance - 100
			swapCfg.OutboundBalance = outboundBalance - 100
			swapCfg.Wallet = strings.ToUpper(cfg.Node)

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
			require.NoError(t, err)

			recommendations, err := autoSwap.GetRecommendations()
			require.NoError(t, err)
			require.Zero(t, recommendations.Lightning)

			swapCfg.InboundBalance = inboundBalance + 100

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
			require.NoError(t, err)

			t.Run("Recommendations", func(t *testing.T) {
				recommendations, err := autoSwap.GetRecommendations()
				require.NoError(t, err)
				require.Len(t, recommendations.Lightning, 1)
				require.Equal(t, string(boltz.ReverseSwap), recommendations.Lightning[0].Type)
			})

			t.Run("Auto", func(t *testing.T) {
				pay(them, us, 1_000_000)
				_, err := autoSwap.Enable()
				require.NoError(t, err)

				time.Sleep(200 * time.Millisecond)
				isAuto := true
				swaps, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{IsAuto: &isAuto})
				require.NoError(t, err)

				require.NotEmpty(t, swaps.ReverseSwaps)
				stream, _ := swapStream(t, admin, swaps.ReverseSwaps[0].Id)
				stream(boltzrpc.SwapState_SUCCESSFUL)

				status, err := autoSwap.GetStatus()
				budget := status.Lightning.Budget
				require.NoError(t, err)
				require.Equal(t, 1, int(budget.Stats.Count))
				require.Less(t, budget.Remaining, int64(budget.Total))
				require.NotZero(t, budget.Stats.TotalFees)
				require.NotZero(t, budget.Stats.TotalAmount)
			})

		})

	})

}

func TestWallet(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
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

	waitForSync(t, client)

	_, err = client.GetInfo()
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(testWallet.Id, nil)
	require.Error(t, err)

	c, err := client.GetWalletCredentials(testWallet.Id, &password)
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
	client, _, stop := setup(t, setupOptions{})
	defer stop()

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

}

func TestImportDuplicateCredentials(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	credentials, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	// after creating one with a password, the first one will be encrypted as well
	second := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_BTC}
	_, err = client.ImportWallet(second, credentials)
	require.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
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
