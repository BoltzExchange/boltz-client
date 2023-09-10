package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	boltz_lnd "github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setup(t *testing.T) (*boltzrpc.Boltz, func()) {

	cfg := boltz_lnd.LoadConfig()
	cfg.RPC.NoTls = true
	cfg.RPC.NoMacaroons = true

	logger.InitLogger(cfg.LogFile, cfg.LogPrefix)

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

	close := func() {
		err := lis.Close()
		if err != nil {
			logger.Error("error closing listener: " + err.Error())
		}
		server.Stop()
	}

	client := boltzrpc.Boltz{
		Client: boltzrpc.NewBoltzClient(conn),
		Ctx:    context.Background(),
	}

	return &client, close
}

func getBtcRpc() *rpcclient.Client {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:18443",
		User:         "lnbits",
		Pass:         "lnbits",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		logger.Fatal(err.Error())
	}

	return client
}

func TestGetInfo(t *testing.T) {
	client, close := setup(t)
	defer close()

	info, err := client.GetInfo()

	fmt.Println(err)
	require.Nil(t, err)
	assert.Equal(t, "regtest", info.Network)
}

func TestDeposit(t *testing.T) {
	client, close := setup(t)
	defer close()

	swap, err := client.Deposit(25)
	require.Nil(t, err)

	info, err := client.GetSwapInfo(swap.Id)
	require.Nil(t, err)
	assert.Equal(t, boltzrpc.SwapState_PENDING, info.Swap.State)

	btc_cli("sendtoaddress " + swap.Address + " 0.0025")
	btc_cli("-generate 1")

	time.Sleep(500 * time.Millisecond)

	info, err = client.GetSwapInfo(swap.Id)
	require.Nil(t, err)
	assert.Equal(t, boltzrpc.SwapState_SUCCESSFUL, info.Swap.State)
}

func TestReverseSwap(t *testing.T) {
	client, close := setup(t)
	defer close()

	swap, err := client.CreateReverseSwap(250000, "", false)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	info, err := client.GetSwapInfo(swap.Id)
	require.Nil(t, err)
	assert.Equal(t, boltzrpc.SwapState_PENDING, info.ReverseSwap.State)

	btc_cli("-generate 1")
	time.Sleep(1500 * time.Millisecond)

	info, err = client.GetSwapInfo(swap.Id)
	require.Nil(t, err)
	assert.Equal(t, boltzrpc.SwapState_SUCCESSFUL, info.ReverseSwap.State)

}

func TestReverseSwapZeroConf(t *testing.T) {
	client, close := setup(t)
	defer close()

	swap, err := client.CreateReverseSwap(250000, "", true)
	require.Nil(t, err)

	time.Sleep(100 * time.Millisecond)

	info, err := client.GetSwapInfo(swap.Id)
	require.Nil(t, err)
	assert.Equal(t, boltzrpc.SwapState_SUCCESSFUL, info.ReverseSwap.State)

}

func sh(cmd string) string {
	out, err := exec.Command("sh", "-c", cmd).Output()

	if err != nil {
		logger.Fatal("could not execute cmd: " + cmd + " err:" + err.Error())
	}

	return strings.TrimSuffix(string(out), "\n")
}

func btc_cli(cmd string) string {
	return sh("docker exec lnbits-legend-bitcoind-1 bitcoin-cli -rpcuser=lnbits -rpcpassword=lnbits -regtest " + cmd)
}
