package test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/BoltzExchange/boltz-client/logger"
)

type Cli func(string) string

func sh(cmd string) string {
	out, err := exec.Command("sh", "-c", cmd).Output()

	if err != nil {
		logger.Fatal("could not execute cmd: " + cmd + " err:" + err.Error())
	}

	return strings.TrimSuffix(string(out), "\n")
}

func InitLogger() {
	logger.Init("", "debug")
}

func BtcCli(cmd string) string {
	return sh("docker exec regtest-bitcoind-1 bitcoin-cli -rpcuser=regtest -rpcpassword=regtest -regtest " + cmd)
}

func LiquidCli(cmd string) string {
	return sh("docker exec regtest-elementsd-1 elements-cli " + cmd)
}

func MineBlock() {
	BtcCli("-generate 1")
	LiquidCli("rescanblockchain")
	LiquidCli("-generate 1")
}

func MineUntil(t *testing.T, cli Cli, height int64) {
	blockHeight, err := strconv.ParseInt(cli("getblockcount"), 10, 64)
	require.NoError(t, err)
	blocks := height - blockHeight
	cli(fmt.Sprintf("-generate %d", blocks))
}

func SendToAddress(cli Cli, address string, amount int64) string {
	return cli("sendtoaddress " + address + " " + fmt.Sprint(float64(amount)/1e8))
}

func LnCli(cmd string) string {
	return sh("docker exec regtest-lnd-1-1 lncli --network regtest --rpcserver=lnd-1:10009 " + cmd)
}

func ClnCli(cmd string) string {
	return sh("docker exec regtest-clightning-1-1 lightning-cli --network regtest " + cmd)
}

func BoltzLnCli(cmd string) string {
	return sh("docker exec regtest-lnd-2-1 lncli --network regtest --rpcserver=lnd-2:10009 " + cmd)
}
