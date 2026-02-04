//go:build !unit

package rpcserver

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"

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

func requireCode(t *testing.T, err error, code codes.Code) {
	assert.Equal(t, code, status.Code(err))
}

func loadConfig(t *testing.T) *config.Config {
	dataDir := "test"
	cfg, err := config.LoadConfig(dataDir)
	require.NoError(t, err)
	cfg.Log.Level = "debug"
	cfg.Node = "lnd"
	cfg.DataDir = t.TempDir()
	cfg.Database.Path = cfg.DataDir + "/boltz.db"
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
	mockWallet.EXPECT().Sync().Maybe()
	chain.AddWallet(mockWallet)
	t.Cleanup(func() {
		chain.RemoveWallet(info.Id)
	})
	return mockWallet, info
}

type mockWalletSetup func(mock *onchainmock.MockWallet)

type chainMocker func(t *testing.T, original onchain.ChainProvider) *onchainmock.MockChainProvider

func coverChainProvider(t *testing.T, mocked *onchainmock.MockChainProvider, original onchain.ChainProvider) {
	mocked.EXPECT().EstimateFee().RunAndReturn(original.EstimateFee).Maybe()
	mocked.EXPECT().GetBlockHeight().RunAndReturn(original.GetBlockHeight).Maybe()
	mocked.EXPECT().GetRawTransaction(mock.Anything).RunAndReturn(original.GetRawTransaction).Maybe()
	mocked.EXPECT().BroadcastTransaction(mock.Anything).RunAndReturn(original.BroadcastTransaction).Maybe()
	mocked.EXPECT().GetUnspentOutputs(mock.Anything).RunAndReturn(original.GetUnspentOutputs).Maybe()
	mocked.EXPECT().IsTransactionConfirmed(mock.Anything).RunAndReturn(original.IsTransactionConfirmed).Maybe()
	mocked.EXPECT().Disconnect().RunAndReturn(original.Disconnect).Maybe()
}

func lessValueChainProvider(t *testing.T, original onchain.ChainProvider) *onchainmock.MockChainProvider {
	txMock := onchainmock.NewMockChainProvider(t)
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
	coverChainProvider(t, txMock, original)
	return txMock
}

// flakyChainProvider initially says that a transaction isn't confirmed, but upon retry it is
func flakyChainProvider(t *testing.T, original onchain.ChainProvider) *onchainmock.MockChainProvider {
	chainMock := onchainmock.NewMockChainProvider(t)
	called := false
	chainMock.EXPECT().BroadcastTransaction(mock.Anything).RunAndReturn(func(txHex string) (string, error) {
		if called {
			return original.BroadcastTransaction(txHex)
		}
		called = true
		return "", errors.New("flaky")
	}).Maybe()
	coverChainProvider(t, chainMock, original)
	return chainMock
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
