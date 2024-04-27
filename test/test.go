package test

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/stretchr/testify/require"

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

func InitTestWallet(currency boltz.Currency, debug bool) (*wallet.Wallet, *wallet.Credentials, error) {
	var err error
	InitLogger()
	if !wallet.Initialized() {
		err = wallet.Init(wallet.Config{
			DataDir: "./test-data",
			Network: boltz.Regtest,
			Debug:   debug,
		})
		if err != nil {
			return nil, nil, err
		}
	}
	credentials := &wallet.Credentials{
		WalletInfo: onchain.WalletInfo{
			Currency: currency,
		},
		Mnemonic: "fog pen possible deer cool muscle describe awkward enforce injury pelican ridge used enrich female enrich museum verify emotion ask office tonight primary large",
	}
	wallet, err := wallet.Login(credentials)
	if err != nil {
		return nil, nil, err
	}
	time.Sleep(200 * time.Millisecond)
	var subaccount *uint64
	subaccounts, err := wallet.GetSubaccounts(true)
	if err != nil {
		return nil, nil, err
	}
	if len(subaccounts) != 0 {
		subaccount = &subaccounts[0].Pointer
	}
	if credentials.Subaccount, err = wallet.SetSubaccount(subaccount); err != nil {
		return nil, nil, err
	}
	time.Sleep(200 * time.Millisecond)
	addr, err := wallet.NewAddress()
	if err != nil {
		return nil, nil, err
	}
	balance, err := wallet.GetBalance()
	if err != nil {
		return nil, nil, err
	}
	if balance.Confirmed == 0 {
		// gdk takes a bit to sync, so make sure we have plenty of utxos available
		for i := 0; i < 10; i++ {
			if currency == boltz.CurrencyBtc {
				SendToAddress(BtcCli, addr, 10000000)
			} else {
				SendToAddress(LiquidCli, addr, 10000000)
			}
		}
		MineBlock()
		ticker := time.NewTicker(1 * time.Second)
		timeout := time.After(15 * time.Second)
		for {
			select {
			case <-ticker.C:
				balance, err := wallet.GetBalance()
				if err != nil {
					return nil, nil, err
				}
				if balance.Confirmed > 0 {
					return wallet, credentials, nil
				}
			case <-timeout:
				return nil, nil, fmt.Errorf("timeout")
			}
		}
	}
	return wallet, credentials, nil
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
