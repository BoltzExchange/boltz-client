//go:build !unit

package rpcserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"

	"github.com/BoltzExchange/boltz-client/v2/internal/macaroons"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/vulpemventures/go-elements/address"

	"github.com/BoltzExchange/boltz-client/v2/internal/autoswap"
	lnmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/lightning"
	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/v2/internal/config"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestMain(m *testing.M) {
	_, err := test.InitTestWallet(false)
	if err != nil {
		logger.Fatal(err.Error())
	}
	os.Exit(m.Run())
}

func requireCode(t *testing.T, err error, code codes.Code) {
	assert.Equal(t, code, status.Code(err))
}

func loadConfig(t *testing.T) *config.Config {
	dataDir := "test"
	cfg, err := config.LoadConfig(dataDir)
	require.NoError(t, err)
	cfg.Log.Level = "debug"
	cfg.Database.Path = t.TempDir() + "/boltz.db"
	cfg.Node = "lnd"
	return cfg
}

func getBoltz(t *testing.T, cfg *config.Config) *boltz.Api {
	boltzApi, err := initBoltz(cfg, boltz.Regtest)
	require.NoError(t, err)
	return boltzApi
}

func newMockWallet(t *testing.T, chain *onchain.Onchain) (*onchainmock.MockWallet, *onchain.WalletInfo) {
	info := &onchain.WalletInfo{
		Name:     "mock",
		Id:       rand.Uint64(),
		TenantId: database.DefaultTenantId,
		Currency: boltz.CurrencyBtc,
	}
	mockWallet := onchainmock.NewMockWallet(t)
	mockWallet.EXPECT().Ready().Return(true).Maybe()
	mockWallet.EXPECT().GetWalletInfo().RunAndReturn(func() onchain.WalletInfo {
		return *info
	}).Maybe()
	mockWallet.EXPECT().Disconnect().Return(nil).Maybe()
	chain.AddWallet(mockWallet)
	t.Cleanup(func() {
		chain.RemoveWallet(info.Id)
	})
	return mockWallet, info
}

type mockWalletSetup func(mock *onchainmock.MockWallet)

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
	txMock.EXPECT().GetRawTransaction(mock.Anything).RunAndReturn(original.GetRawTransaction).Maybe()
	return txMock
}

func getOnchain(t *testing.T, cfg *config.Config) *onchain.Onchain {
	boltzApi := getBoltz(t, cfg)
	chain, err := initOnchain(cfg, boltzApi, boltz.Regtest)
	require.NoError(t, err)
	return chain
}

func getTransactionFee(t *testing.T, chain *onchain.Onchain, currency boltz.Currency, txId string) uint64 {
	tx, err := chain.GetTransaction(currency, txId, nil, false)
	require.NoError(t, err)
	fee, err := chain.GetTransactionFee(tx)
	require.NoError(t, err)
	return fee
}

var password = "password"
var swapAmount = uint64(100000)

func walletName(currency boltzrpc.Currency) string {
	return "regtest" + currency.String()
}

type setupOptions struct {
	cfg       *config.Config
	chain     *onchain.Onchain
	boltzApi  *boltz.Api
	lightning lightning.LightningNode
	node      string
	dontSync  bool
	wallets   []onchain.Wallet
}

func waitForSync(t *testing.T, client client.Boltz) {
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.After(2 * time.Second)
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

	if cfg.Node == "" || strings.ToLower(cfg.Node) == "standalone" {
		cfg.Standalone = true
	}

	logger.Init(cfg.Log)

	var err error

	rpc := NewRpcServer(cfg)
	require.NoError(t, rpc.Init())
	rpc.boltzServer.boltz = options.boltzApi
	rpc.boltzServer.onchain = options.chain
	rpc.boltzServer.lightning = options.lightning
	go func() {
		err := rpc.boltzServer.start(cfg)
		if err != nil {
			logger.Warn("error starting boltz server: " + err.Error())
		} else {
			for _, wallet := range options.wallets {
				rpc.boltzServer.onchain.AddWallet(wallet)
			}
		}
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
	if password := cfg.RPC.Password; password != "" {
		clientConn.SetPassword(password)
	} else {
		macaroonFile, err := os.ReadFile("./test/macaroons/admin.macaroon")
		require.NoError(t, err)
		clientConn.SetMacaroon(hex.EncodeToString(macaroonFile))
	}

	boltzClient := client.NewBoltzClient(clientConn)
	autoSwapClient := client.NewAutoSwapClient(clientConn)

	if !options.dontSync {

		waitForSync(t, boltzClient)
	}

	return boltzClient, autoSwapClient, func() {
		_, err = autoSwapClient.ResetConfig(client.LnAutoSwap)
		_, err = autoSwapClient.ResetConfig(client.ChainAutoSwap)
		require.NoError(t, boltzClient.Stop())
	}
}

func getCli(currency boltzrpc.Currency) test.Cli {
	return test.GetCli(parseCurrency(currency))
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
				test.PrintBackendLogs()
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

	t.Run("Syncing", func(t *testing.T) {
		node := lnmock.NewMockLightningNode(t)

		node.EXPECT().Connect().Return(nil)
		node.EXPECT().GetInfo().Return(&lightning.LightningInfo{Synced: false}, nil)

		client, _, stop := setup(t, setupOptions{lightning: node, dontSync: true})
		defer stop()

		_, err := client.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unavailable)
		require.ErrorContains(t, err, "lightning node")
	})
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
	global := admin
	global.SetTenant(macaroons.TenantAll)

	tenantName := "test"

	tenantInfo, write, read := createTenant(t, admin, tenantName)

	list, err := admin.ListTenants()
	require.NoError(t, err)
	require.Len(t, list.Tenants, 2)

	tenant := client.NewBoltzClient(write)
	tenantAuto := client.NewAutoSwapClient(write)
	readTenant := client.NewBoltzClient(read)

	t.Run("Reserved", func(t *testing.T) {
		_, err := admin.CreateTenant(macaroons.TenantAll)
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("SetTenant", func(t *testing.T) {
		admin := client.NewBoltzClient(conn)

		admin.SetTenant(tenantName)
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)

		admin.SetTenant(info.Tenant.Id)
		info, err = admin.GetInfo()
		require.NoError(t, err)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)

		admin.SetTenant("invalid")
		_, err = admin.GetInfo()
		require.Error(t, err)
	})

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

	t.Run("Infos", func(t *testing.T) {
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.NotNil(t, info.Tenant)

		info, err = global.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.Nil(t, info.Tenant)

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
		testWallet := fundedWallet(t, admin, boltzrpc.Currency_LBTC)
		hasWallets := func(t *testing.T, client client.Boltz, amount int) {
			wallets, err := client.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, amount)
		}
		hasWallets(t, tenant, 0)
		hasWallets(t, admin, 2)

		_, err = tenant.GetWallet(testWallet.Name)
		requireCode(t, err, codes.NotFound)

		walletParams := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "test"}
		_, err = readTenant.CreateWallet(walletParams)
		requireCode(t, err, codes.PermissionDenied)

		_, err = tenant.CreateWallet(walletParams)
		requireCode(t, err, codes.OK)

		hasWallets(t, tenant, 1)
		hasWallets(t, admin, 2)
		hasWallets(t, global, 3)
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
			externalPay = true
			_, err = tenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay, Address: test.BtcCli("getnewaddress")})
			require.NoError(t, err)
			swaps, err := tenant.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Len(t, swaps.ReverseSwaps, 1)
		})

		t.Run("Read", func(t *testing.T) {
			_, err = readTenant.CreateSwap(&boltzrpc.CreateSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
		})
	})
}

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

func TestGetPairs(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	info, err := client.GetPairs()

	require.NoError(t, err)
	require.Len(t, info.Submarine, 2)
	require.Len(t, info.Reverse, 2)
	require.Len(t, info.Chain, 2)
}

func checkTxOutAddress(t *testing.T, chain *onchain.Onchain, currency boltz.Currency, txId string, outAddress string, cooperative bool) {
	transaction, err := chain.GetTransaction(currency, txId, nil, false)
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

					currency := parseCurrency(tc.to)

					claimFee := getTransactionFee(t, chain, currency, info.ReverseSwap.ClaimTransactionId)

					totalFees := info.ReverseSwap.InvoiceAmount - info.ReverseSwap.OnchainAmount
					require.Equal(t, int64(totalFees+claimFee), *info.ReverseSwap.ServiceFee+int64(*info.ReverseSwap.OnchainFee))

					if tc.external {
						require.Equal(t, addr, info.ReverseSwap.ClaimAddress)
					}
					checkTxOutAddress(t, chain, currency, info.ReverseSwap.ClaimTransactionId, info.ReverseSwap.ClaimAddress, !tc.disablePartials)

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
	wallets, err := client.GetWallets(&currency, false)
	require.NoError(t, err)
	for _, existing := range wallets.Wallets {
		if existing.Balance.Confirmed > 0 {
			return existing
		}
	}
	params := &boltzrpc.WalletParams{Currency: currency, Name: walletName(currency)}
	mnemonic := test.WalletMnemonic
	subaccount := uint64(test.WalletSubaccount)
	creds := &boltzrpc.WalletCredentials{Mnemonic: &mnemonic, Subaccount: &subaccount}
	result, err := client.ImportWallet(params, creds)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)
	return result
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

func TestAutoSwap(t *testing.T) {
	cfg := loadConfig(t)
	cfg.Node = "CLN"

	_, err := connectLightning(nil, cfg.Cln)
	require.NoError(t, err)
	_, err = connectLightning(nil, cfg.LND)
	require.NoError(t, err)

	admin, autoSwap, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()
	fundedWallet(t, admin, boltzrpc.Currency_LBTC)

	reset := func(t *testing.T) {
		_, err = autoSwap.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)
		_, err = autoSwap.ResetConfig(client.ChainAutoSwap)
		require.NoError(t, err)
	}

	t.Run("Chain", func(t *testing.T) {
		reset(t)
		cfg := &autoswaprpc.ChainConfig{
			FromWallet: walletName(boltzrpc.Currency_LBTC),
			ToWallet:   cfg.Node,
			Budget:     1_000_000,
		}
		fromWallet, err := admin.GetWallet(cfg.FromWallet)
		require.NoError(t, err)
		cfg.MaxBalance = fromWallet.Balance.Confirmed - 1000
		cfg.ReserveBalance = cfg.MaxBalance - swapAmount

		_, err = autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: cfg})
		require.NoError(t, err)

		status, err := autoSwap.GetStatus()
		require.NoError(t, err)
		require.False(t, status.Chain.Running)

		recommendations, err := autoSwap.GetRecommendations()
		require.NoError(t, err)
		require.Len(t, recommendations.Chain, 1)

		stream, _ := swapStream(t, admin, "")

		_, err = autoSwap.ExecuteRecommendations(&autoswaprpc.ExecuteRecommendationsRequest{
			Chain: recommendations.Chain,
		})
		require.NoError(t, err)

		info := stream(boltzrpc.SwapState_PENDING)
		require.NotNil(t, info.ChainSwap)
		id := info.ChainSwap.Id

		recommendations, err = autoSwap.GetRecommendations()
		require.NoError(t, err)
		require.NotEmpty(t, recommendations.Chain[0].Swap.GetDismissedReasons())

		require.Eventually(t, func() bool {
			recommendations, err = autoSwap.GetRecommendations()
			require.NoError(t, err)
			return recommendations.Chain[0].Swap == nil
		}, 10*time.Second, 250*time.Millisecond)

		response, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{Include: boltzrpc.IncludeSwaps_AUTO})
		require.Len(t, response.ChainSwaps, 1)
		require.Equal(t, id, response.ChainSwaps[0].Id)
		require.True(t, response.ChainSwaps[0].IsAuto)

		stream, _ = swapStream(t, admin, id)
		require.NoError(t, err)
		test.MineBlock()
		stream(boltzrpc.SwapState_PENDING)
		test.MineBlock()
		stream(boltzrpc.SwapState_SUCCESSFUL)

		_, write, _ := createTenant(t, admin, "test")
		tenant := client.NewAutoSwapClient(write)

		_, err = tenant.GetChainConfig()
		require.Error(t, err)

		cfg.Enabled = true
		_, err = autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: cfg})
		require.NoError(t, err)

		status, err = autoSwap.GetStatus()
		require.NoError(t, err)
		require.True(t, status.Chain.Running)

	})

	t.Run("Lightning", func(t *testing.T) {
		reset(t)

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
					Wallet:   walletName(boltzrpc.Currency_LBTC),
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
			_, err := autoSwap.SetLightningConfigValue("wallet", walletName(boltzrpc.Currency_LBTC))
			require.NoError(t, err)
			_, err = autoSwap.SetLightningConfigValue("enabled", true)
			require.NoError(t, err)
			_, err = admin.RemoveWallet(walletId(t, admin, boltzrpc.Currency_LBTC))
			require.Error(t, err)
		})

		t.Run("Start", func(t *testing.T) {
			swapCfg := autoswap.DefaultLightningConfig()
			swapCfg.AcceptZeroConf = true
			swapCfg.Budget = 1000000
			swapCfg.MaxFeePercent = 10
			swapCfg.Currency = boltzrpc.Currency_BTC
			swapCfg.InboundBalance = 1
			swapCfg.OutboundBalance = 1
			swapCfg.Wallet = strings.ToUpper(cfg.Node)

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
			require.NoError(t, err)

			setupRecommendation := func(t *testing.T) {
				recommendations, err := autoSwap.GetRecommendations()
				require.NoError(t, err)
				recommendation := recommendations.Lightning[0]
				if recommendation.Swap == nil {
					offset := uint64(100000)
					swapCfg.InboundBalance = recommendation.Channel.InboundSat + offset
					swapCfg.OutboundBalance = recommendation.Channel.OutboundSat - offset

					_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
					require.NoError(t, err)
				}
			}

			t.Run("Recommendations", func(t *testing.T) {
				setupRecommendation(t)

				recommendations, err := autoSwap.GetRecommendations()
				require.NoError(t, err)
				require.Len(t, recommendations.Lightning, 1)
				require.Equal(t, boltzrpc.SwapType_REVERSE, recommendations.Lightning[0].Swap.Type)

				stream, _ := swapStream(t, admin, "")
				_, err = autoSwap.ExecuteRecommendations(&autoswaprpc.ExecuteRecommendationsRequest{
					Lightning: recommendations.Lightning,
				})
				require.NoError(t, err)
				info := stream(boltzrpc.SwapState_PENDING)
				require.NotNil(t, info.ReverseSwap)
				require.True(t, info.ReverseSwap.IsAuto)

				stream(boltzrpc.SwapState_SUCCESSFUL)
				test.MineBlock()
			})

			t.Run("Auto", func(t *testing.T) {
				setupRecommendation(t)

				stream, _ := swapStream(t, admin, "")

				_, err := autoSwap.Enable()
				require.NoError(t, err)

				test.MineBlock()
				info := stream(boltzrpc.SwapState_PENDING)
				require.NotNil(t, info.ReverseSwap)
				require.True(t, info.ReverseSwap.IsAuto)
				id := info.ReverseSwap.Id

				swaps, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{Include: boltzrpc.IncludeSwaps_AUTO})
				require.NoError(t, err)
				// it might be the first index since we create swaps above aswell
				require.True(t, slices.ContainsFunc(swaps.ReverseSwaps, func(s *boltzrpc.ReverseSwapInfo) bool {
					return s.Id == id
				}))
				stream, _ = swapStream(t, admin, id)
				stream(boltzrpc.SwapState_SUCCESSFUL)

				status, err := autoSwap.GetStatus()
				budget := status.Lightning.Budget
				require.NoError(t, err)
				require.NotZero(t, budget.Stats.Count)
				require.Less(t, budget.Remaining, budget.Total)
				require.NotZero(t, budget.Stats.TotalFees)
				require.NotZero(t, budget.Stats.TotalAmount)
			})

		})

	})
}

func TestWalletTransactions(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)

	client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
	t.Cleanup(stop)

	t.Run("Pagination", func(t *testing.T) {
		offset := uint64(1)
		limit := uint64(1)
		txId := "test"
		testWallet, walletInfo := newMockWallet(t, chain)
		testWallet.EXPECT().GetTransactions(limit, offset).Return([]*onchain.WalletTransaction{{Id: txId}}, nil)

		response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{
			Id:     walletInfo.Id,
			Offset: &offset,
			Limit:  &limit,
		})
		require.NoError(t, err)
		require.Len(t, response.Transactions, 1)
		require.Equal(t, txId, response.Transactions[0].Id)
	})

	t.Run("Readonly", func(t *testing.T) {
		testWallet, walletInfo := newMockWallet(t, chain)
		testWallet.EXPECT().GetTransactions(mock.Anything, mock.Anything).Return(nil, nil)
		walletInfo.Readonly = true

		response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{Id: walletInfo.Id})
		require.NoError(t, err)
		require.Empty(t, response.Transactions)
	})

	claimTx := "claim"
	refundTx := "refund"
	lockupTx := "lockup"
	fakeSwaps := test.FakeSwaps{
		ReverseSwaps: []database.ReverseSwap{
			{
				ClaimTransactionId: claimTx,
			},
		},
		Swaps: []database.Swap{
			{
				RefundTransactionId: refundTx,
			},
		},
		ChainSwaps: []database.ChainSwap{
			{
				FromData: &database.ChainSwapData{
					LockupTransactionId: lockupTx,
				},
			},
		},
	}
	fakeSwaps.Create(t, cfg.Database)

	tests := []struct {
		desc         string
		transactions []*onchain.WalletTransaction
		fakeSwaps    test.FakeSwaps
		txType       boltzrpc.TransactionType
	}{
		{
			desc: "Claim",
			transactions: []*onchain.WalletTransaction{
				{Id: claimTx},
			},
			txType: boltzrpc.TransactionType_CLAIM,
		},
		{
			desc: "Refund",
			transactions: []*onchain.WalletTransaction{
				{Id: refundTx},
			},
			txType: boltzrpc.TransactionType_REFUND,
		},
		{
			desc: "Lockup",
			transactions: []*onchain.WalletTransaction{
				{Id: lockupTx},
			},
			txType: boltzrpc.TransactionType_LOCKUP,
		},

		{
			desc: "Consolidate",
			transactions: []*onchain.WalletTransaction{
				{Id: "consolidation", IsConsolidation: true},
			},
			txType: boltzrpc.TransactionType_CONSOLIDATION,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testWallet, walletInfo := newMockWallet(t, chain)
			testWallet.EXPECT().GetTransactions(mock.Anything, mock.Anything).Return(test.transactions, nil)

			response, err := client.ListWalletTransactions(&boltzrpc.ListWalletTransactionsRequest{Id: walletInfo.Id})
			require.NoError(t, err)
			for _, tx := range response.Transactions {
				if tx.Infos[0].Type == test.txType {
					return
				}
			}
			require.Fail(t, "swap not found")
		})
	}
}

func TestBumpTransaction(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{chain: chain, cfg: cfg})
	defer stop()

	feeRate := func(value float64) *float64 { return &value }

	someTxId := test.SendToAddress(test.BtcCli, test.GetNewAddress(test.BtcCli), 5000)
	txIdRequest := &boltzrpc.BumpTransactionRequest{
		Previous: &boltzrpc.BumpTransactionRequest_TxId{TxId: someTxId},
	}
	newTxId := "newTransaction"

	tests := []struct {
		desc    string
		setup   func(t *testing.T)
		swaps   *test.FakeSwaps
		request *boltzrpc.BumpTransactionRequest
		wantErr string
	}{
		{
			desc:    "Success",
			request: txIdRequest,
			setup: func(t *testing.T) {
				original := chain.Btc.Blocks
				blockProvider := onchainmock.NewMockBlockProvider(t)
				rate := float64(5)
				blockProvider.EXPECT().EstimateFee().Return(rate, nil)
				chain.Btc.Blocks = blockProvider
				t.Cleanup(func() {
					chain.Btc.Blocks = original
				})

				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, rate).Return(newTxId, nil)
			},
		},
		{
			desc:    "AlreadyConfirmed",
			request: txIdRequest,
			setup: func(t *testing.T) {
				original := chain.Btc.Tx
				txProvider := onchainmock.NewMockTxProvider(t)
				txProvider.EXPECT().IsTransactionConfirmed(someTxId).Return(true, nil)
				txProvider.EXPECT().GetRawTransaction(someTxId).RunAndReturn(original.GetRawTransaction)
				chain.Btc.Tx = txProvider
				t.Cleanup(func() {
					chain.Btc.Tx = original
				})
			},
			wantErr: "already confirmed",
		},
		{
			desc:    "Error",
			request: txIdRequest,
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return("", errors.New("error"))
			},
			wantErr: "error",
		},
		{
			desc:    "NotFound",
			request: txIdRequest,
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return("", errors.New("not found"))
			},
			wantErr: "does not belong to any wallet",
		},
		{
			desc:    "Readonly",
			request: txIdRequest,
			setup: func(t *testing.T) {
				_, walletInfo := newMockWallet(t, chain)
				walletInfo.Readonly = true
			},
			wantErr: "does not belong to any wallet",
		},
		{
			desc: "FeeRate/Less",
			request: &boltzrpc.BumpTransactionRequest{
				Previous:    txIdRequest.Previous,
				SatPerVbyte: feeRate(1),
			},
			wantErr: "fee rate has to be higher",
		},
		{
			desc: "SwapId/Success",
			request: &boltzrpc.BumpTransactionRequest{Previous: &boltzrpc.BumpTransactionRequest_SwapId{
				SwapId: "normal-swap",
			}},
			swaps: &test.FakeSwaps{
				Swaps: []database.Swap{
					{
						Id:                  "normal-swap",
						LockupTransactionId: someTxId,
					},
				},
			},
			setup: func(t *testing.T) {
				mockWallet, _ := newMockWallet(t, chain)
				mockWallet.EXPECT().BumpTransactionFee(someTxId, mock.Anything).Return(newTxId, nil)
			},
		},
		{
			desc: "SwapId/NotImplemented",
			request: &boltzrpc.BumpTransactionRequest{Previous: &boltzrpc.BumpTransactionRequest_SwapId{
				SwapId: "refundable",
			}},
			swaps: &test.FakeSwaps{
				Swaps: []database.Swap{
					{
						Id:                  "refundable",
						RefundTransactionId: someTxId,
					},
				},
			},
			setup: func(t *testing.T) {
				newMockWallet(t, chain)
			},
			wantErr: "refund transactions cannot be bumped",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			if tc.swaps != nil {
				tc.swaps.Create(t, cfg.Database)
			}

			_, err := client.BumpTransaction(tc.request)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("Swaps", func(t *testing.T) {
		t.Run("Submarine", func(t *testing.T) {
			swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
				Amount: swapAmount,
			})
			require.NoError(t, err)

			_, statusStream := swapStream(t, client, swap.Id)

			lockupTx := test.SendToAddress(test.BtcCli, swap.Address, swap.ExpectedAmount)
			info := statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
			require.Equal(t, lockupTx, info.Swap.LockupTransactionId)

			newLockupTx := test.BumpFee(test.BtcCli, lockupTx)
			info = statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool)
			require.Equal(t, newLockupTx, info.Swap.LockupTransactionId)
		})

		t.Run("Chain", func(t *testing.T) {
			externalPay := true
			toAddress := test.GetNewAddress(test.LiquidCli)
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Amount: &swapAmount,
				Pair: &boltzrpc.Pair{
					From: boltzrpc.Currency_BTC,
					To:   boltzrpc.Currency_LBTC,
				},
				ToAddress:   &toAddress,
				ExternalPay: &externalPay,
			})
			require.NoError(t, err)

			_, statusStream := swapStream(t, client, swap.Id)
			lockupTx := test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, swap.FromData.Amount)
			info := statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool).GetChainSwap()
			require.Equal(t, lockupTx, info.FromData.GetLockupTransactionId())

			newLockupTx := test.BumpFee(test.BtcCli, lockupTx)
			info = statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionMempool).GetChainSwap()
			require.Equal(t, newLockupTx, info.FromData.GetLockupTransactionId())
		})
	})
}

func TestWallet(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	// the main setup function already created a wallet
	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
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

	_, err = client.GetWallet(walletName(boltzrpc.Currency_LBTC))
	require.NoError(t, err)
}

func TestUnlock(t *testing.T) {
	password := "password"
	cfg := loadConfig(t)
	require.NoError(t, cfg.Database.Connect())
	walletCredentials := test.WalletCredentials(boltz.CurrencyBtc)
	encryptedCredentials, err := walletCredentials.Encrypt(password)
	require.NoError(t, err)
	encryptedWallet := &database.Wallet{
		WalletCredentials: encryptedCredentials,
	}
	err = cfg.Database.CreateWallet(encryptedWallet)
	require.NoError(t, err)

	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	_, err = client.GetInfo()
	require.Error(t, err)

	require.Error(t, client.Unlock("wrong"))
	require.NoError(t, client.Unlock(password))

	test.MineBlock()

	waitForSync(t, client)

	_, err = client.GetInfo()
	require.NoError(t, err)

	_, err = client.GetWalletCredentials(encryptedWallet.Id, nil)
	require.Error(t, err)

	c, err := client.GetWalletCredentials(encryptedWallet.Id, &password)
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

func TestDirectReverseSwapPayments(t *testing.T) {
	cfg := loadConfig(t)
	maxZeroConfAmount := uint64(100000)
	cfg.MaxZeroConfAmount = &maxZeroConfAmount
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{cfg: cfg, chain: chain})
	fundedWallet(t, client, boltzrpc.Currency_LBTC)
	defer stop()

	t.Run("Multiple", func(t *testing.T) {
		externalPay := true
		request := &boltzrpc.CreateReverseSwapRequest{
			Pair: &boltzrpc.Pair{
				From: boltzrpc.Currency_BTC,
				To:   boltzrpc.Currency_LBTC,
			},
			ExternalPay: &externalPay,
			Amount:      maxZeroConfAmount,
		}
		firstResponse, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		_, statusStream := swapStream(t, client, firstResponse.Id)
		first := statusStream(boltzrpc.SwapState_PENDING, boltz.SwapCreated).ReverseSwap
		claimAddress := first.ClaimAddress

		request.Address = claimAddress
		request.AcceptZeroConf = true
		externalPay = false
		second, err := client.CreateReverseSwap(request)
		require.NoError(t, err)
		require.NotEmpty(t, second.ClaimTransactionId)
		test.MineBlock()

		// send a bunch of payments to the address.
		test.SendToAddress(test.LiquidCli, claimAddress, first.OnchainAmount/2)
		correct := test.SendToAddress(test.LiquidCli, claimAddress, first.OnchainAmount)
		first = statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.TransactionDirect).ReverseSwap
		claimTx := first.ClaimTransactionId
		require.NotEqualf(t, claimTx, second.ClaimTransactionId, "transactions are the same")
		require.Equal(t, correct, claimTx)
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
				mockTx := onchainmock.NewMockTxProvider(t)
				mockTx.EXPECT().IsTransactionConfirmed(mock.Anything).RunAndReturn(func(string) (bool, error) {
					return confirmed, nil
				})
				currency.Tx = mockTx
			}

			externalPay := true
			request := &boltzrpc.CreateReverseSwapRequest{
				Pair: &boltzrpc.Pair{
					From: boltzrpc.Currency_BTC,
					To:   tc.currency,
				},
				ExternalPay: &externalPay,
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
			// it only gets to mempool state on liquid since its using gdk - btc uses the node wallet
			if !tc.zeroconf && tc.currency == boltzrpc.Currency_LBTC {
				statusStream(boltzrpc.SwapState_PENDING, boltz.TransactionDirectMempool)
			}
			confirmed = true
			test.MineBlock()
			info := statusStream(boltzrpc.SwapState_SUCCESSFUL, boltz.TransactionDirect)
			require.Equal(t, info.ReverseSwap.ClaimAddress, swap.Address)
		})
	}
}

func TestCreateWallet(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	t.Run("WithPassword", func(t *testing.T) {
		testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
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
	})

	t.Run("DuplicateName", func(t *testing.T) {
		_, err := client.CreateWallet(&boltzrpc.WalletParams{Name: walletName(boltzrpc.Currency_BTC), Currency: boltzrpc.Currency_BTC})
		require.Error(t, err)
	})

}

func TestWalletSendReceive(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	client, _, stop := setup(t, setupOptions{chain: chain})
	defer stop()

	t.Run("Receive", func(t *testing.T) {
		mockWallet, walletInfo := newMockWallet(t, chain)
		errWallet, errInfo := newMockWallet(t, chain)

		address := "test"
		mockWallet.EXPECT().NewAddress().Return(address, nil)
		errWallet.EXPECT().NewAddress().Return("", errors.New("error"))

		response, err := client.WalletReceive(walletInfo.Id)
		require.NoError(t, err)
		require.Equal(t, address, response.Address)

		_, err = client.WalletReceive(errInfo.Id)
		require.Error(t, err)
	})

	t.Run("SendFee", func(t *testing.T) {
		nodeWallet, err := client.GetWallet(strings.ToUpper(cfg.Node))
		require.NoError(t, err)
		testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)

		swapRelated := true
		sendAll := true
		t.Run("NodeWallet", func(t *testing.T) {
			_, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            nodeWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.Error(t, err)
		})

		t.Run("AddressRequired", func(t *testing.T) {
			swapRelated := false
			_, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            nodeWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.Error(t, err)
		})

		t.Run("Normal", func(t *testing.T) {
			response, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            testWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
			})
			require.NoError(t, err)
			require.NotZero(t, response.Fee)
			require.NotZero(t, response.FeeRate)
		})

		t.Run("CustomFee", func(t *testing.T) {
			satPerVbyte := 10.0
			response, err := client.GetSendFee(&boltzrpc.WalletSendRequest{
				Id:            testWallet.Id,
				SendAll:       &sendAll,
				IsSwapAddress: &swapRelated,
				SatPerVbyte:   &satPerVbyte,
			})
			require.NoError(t, err)
			require.NotZero(t, response.Fee)
			require.Equal(t, satPerVbyte, response.FeeRate)
		})
	})

	t.Run("Send", func(t *testing.T) {
		satPerVbyte := 10.0
		sendAll := true

		defaultEstimate, err := chain.EstimateFee(boltz.CurrencyBtc)
		require.NoError(t, err)

		tests := []struct {
			desc    string
			request *boltzrpc.WalletSendRequest
			setup   mockWalletSetup
			result  string
			err     require.ErrorAssertionFunc
		}{
			{
				desc: "Normal",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					Amount:  1000,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: defaultEstimate,
					}).Return("txid", nil)
				},
			},
			{
				desc: "CustomFee",
				request: &boltzrpc.WalletSendRequest{
					Address:     "address",
					Amount:      1000,
					SatPerVbyte: &satPerVbyte,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: satPerVbyte,
						SendAll:     false,
					}).Return("txid", nil)
				},
			},
			{
				desc: "SendAll",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					Amount:  1000,
					SendAll: &sendAll,
				},
				result: "txid",
				err:    require.NoError,
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: defaultEstimate,
						SendAll:     sendAll,
					}).Return("txid", nil)
				},
			},
			{
				desc: "MissingAddress",
				request: &boltzrpc.WalletSendRequest{
					Amount: 1000,
				},
				err: require.Error,
			},
			{
				desc: "MissingAmount",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
				},
				err: require.Error,
			},
			{
				desc: "WalletError",
				request: &boltzrpc.WalletSendRequest{
					Address: "address",
					Amount:  1000,
				},
				setup: func(mockWallet *onchainmock.MockWallet) {
					mockWallet.EXPECT().SendToAddress(onchain.WalletSendArgs{
						Address:     "address",
						Amount:      1000,
						SatPerVbyte: defaultEstimate,
					}).Return("", errors.New("wallet error"))
				},
				err: require.Error,
			},
		}

		for _, tc := range tests {
			t.Run(tc.desc, func(t *testing.T) {
				mockWallet, walletInfo := newMockWallet(t, chain)
				if tc.setup != nil {
					tc.setup(mockWallet)
				}
				tc.request.Id = walletInfo.Id

				response, err := client.WalletSend(tc.request)
				tc.err(t, err)
				if err == nil {
					require.Equal(t, tc.result, response.TxId)
				}
			})
		}
	})
}

func TestImportDuplicateCredentials(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
	credentials, err := client.GetWalletCredentials(testWallet.Id, nil)
	require.NoError(t, err)

	// duplicates are only allowed for different currencies
	second := &boltzrpc.WalletParams{Name: "another", Currency: boltzrpc.Currency_LBTC}
	_, err = client.ImportWallet(second, credentials)
	require.Error(t, err)

	second.Currency = boltzrpc.Currency_BTC
	_, err = client.ImportWallet(second, credentials)
	require.NoError(t, err)
}

func TestChangePassword(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	testWallet := fundedWallet(t, client, boltzrpc.Currency_LBTC)
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

// the order of tests is important here.
// since the refund tests mine a lot of blocks at once, channel force closes can happen
// so after these are finished, other tests which do LN payments might fail
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
			invoice, err := node.CreateInvoice(100000, nil, 0, "test")
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
			stream(boltzrpc.SwapState_SUCCESSFUL)

			paid, err := node.CheckInvoicePaid(invoice.PaymentHash)
			require.NoError(t, err)
			require.True(t, paid)
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
										refundFee := getTransactionFee(t, chain, from, info.RefundTransactionId)
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

func TestChainSwap(t *testing.T) {
	cfg := loadConfig(t)
	chain := getOnchain(t, cfg)
	boltzApi := getBoltz(t, cfg)

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
			info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed)
			require.NotEmpty(t, info.ChainSwap.Error)
		})

		t.Run("Amountless", func(t *testing.T) {
			swap, err := client.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{
				Pair:           pair,
				ExternalPay:    &externalPay,
				ToAddress:      &to,
				AcceptZeroConf: &acceptZeroConf,
			})
			require.NoError(t, err)

			amount := pairInfo.Limits.Minimal
			test.SendToAddress(test.BtcCli, swap.FromData.LockupAddress, amount)
			test.MineBlock()

			stream, _ := swapStream(t, client, swap.Id)

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
		originalTx := chain.Btc.Tx
		t.Cleanup(func() {
			chain.Btc.Tx = originalTx
		})
		toWallet := fundedWallet(t, client, boltzrpc.Currency_BTC)

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
			fromCli := getCli(tc.from)
			toCli := getCli(tc.to)

			refundAddress := fromCli("getnewaddress")
			toAddress := toCli("getnewaddress")

			fromWallet := fundedWallet(t, client, tc.from)
			toWallet := fundedWallet(t, client, tc.to)

			checkSwap := func(t *testing.T, id string) {
				response, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{})
				require.NoError(t, err)
				require.NotEmpty(t, response.ChainSwaps)
				for _, swap := range response.ChainSwaps {
					if swap.Id == id {
						fromFee := getTransactionFee(t, chain, parseCurrency(tc.from), swap.FromData.GetLockupTransactionId())
						require.NoError(t, err)
						if swap.FromData.WalletId == nil {
							fromFee = 0
						}
						toFee := getTransactionFee(t, chain, parseCurrency(tc.to), swap.ToData.GetLockupTransactionId())
						require.NoError(t, err)
						claimFee := getTransactionFee(t, chain, parseCurrency(tc.to), swap.ToData.GetTransactionId())
						require.NoError(t, err)

						require.Equal(t, int(fromFee+toFee+claimFee), int(*swap.OnchainFee))
						return
					}
				}
				require.Fail(t, "swap not returned by listswaps", id)
			}

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

					checkSwap(t, swap.Id)
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
							info := statusStream(boltzrpc.SwapState_ERROR, boltz.TransactionLockupFailed).ChainSwap
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
							Amount:      &swapAmount,
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
						require.Contains(t, info.ClaimableSwaps, swap.Id)

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
							destination.WalletId = toWallet.Id
							_, err := client.ClaimSwaps(request)
							require.NoError(t, err)

							info = stream(boltzrpc.SwapState_SUCCESSFUL).ChainSwap
							checkSwap(t, info.Id)

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

func TestPasswordAuth(t *testing.T) {
	cfg := loadConfig(t)
	cfg.Standalone = true
	cfg.RPC.Password = "testpassword"

	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	t.Run("Success", func(t *testing.T) {
		info, err := client.GetInfo()
		require.NoError(t, err)
		require.Equal(t, "regtest", info.Network)
	})

	t.Run("WrongPassword", func(t *testing.T) {
		wrongClient := client
		wrongClient.SetPassword("wrongpassword")
		_, err := wrongClient.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unauthenticated)
	})

	t.Run("NoPassword", func(t *testing.T) {
		noPasswordClient := client
		noPasswordClient.SetPassword("")
		_, err := noPasswordClient.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unauthenticated)
	})
}
