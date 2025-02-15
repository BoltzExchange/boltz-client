package test

import (
	"fmt"
	"github.com/BoltzExchange/go-electrum/electrum"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
)

type Cli func(string) string

func bash(cmd string) string {
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()

	if err != nil {
		fmt.Println(string(out))
		logger.Fatal("could not execute cmd: " + cmd + " err:" + err.Error())
	}

	return strings.TrimSuffix(string(out), "\n")
}

func run(cmd string) string {
	return bash(fmt.Sprintf("docker exec -i boltz-scripts bash -c \"source /etc/profile.d/utils.sh && %s\"", cmd))
}

const walletDataDir = "./test-data"

func ClearWalletDataDir() {
	os.RemoveAll(walletDataDir)
}

func InitTestWallet(currency boltz.Currency, debug bool) (*wallet.Wallet, *wallet.Credentials, error) {
	var err error
	InitLogger()
	ClearWalletDataDir()
	if !wallet.Initialized() {
		err = wallet.Init(wallet.Config{
			DataDir:                  walletDataDir,
			Network:                  boltz.Regtest,
			Debug:                    debug,
			Electrum:                 onchain.RegtestElectrumConfig,
			AutoConsolidateThreshold: wallet.DefaultAutoConsolidateThreshold,
			MaxInputs:                wallet.MaxInputs,
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
	electrum.DebugMode = true
	logger.Init(logger.Options{Level: "debug"})
}

func BtcCli(cmd string) string {
	return run("bitcoin-cli-sim-server " + cmd)
}

func BackendCli(cmd string) string {
	return bash("docker exec -i boltz-backend /boltz-backend/bin/boltz-cli " + cmd)
}

func LiquidCli(cmd string) string {
	return run("elements-cli-sim-server " + cmd)
}

func ClnCli(cmd string) string {
	return run("lightning-cli-sim 1 " + cmd)
}

func syncCln() {
	for ClnCli("getinfo | jq -r .blockheight") != BtcCli("getblockcount") {
		time.Sleep(250 * time.Millisecond)
	}
}

func MineBlock() {
	BtcCli("-generate 1")
	LiquidCli("-generate 1")
	syncCln()
}

func MineUntil(t *testing.T, cli Cli, height int64) {
	blockHeight, err := strconv.ParseInt(cli("getblockcount"), 10, 64)
	require.NoError(t, err)
	blocks := height - blockHeight
	cli(fmt.Sprintf("-generate %d", blocks))
	syncCln()
}

func GetNewAddress(cli Cli) string {
	return cli("getnewaddress")
}

func SendToAddress(cli Cli, address string, amount uint64) string {
	return cli("sendtoaddress " + address + " " + fmt.Sprint(float64(amount)/1e8))
}

func GetBolt12Offer() string {
	return ClnCli("offer any '' | jq -r .bolt12")
}

type FakeSwaps struct {
	Swaps        []database.Swap
	ReverseSwaps []database.ReverseSwap
	ChainSwaps   []database.ChainSwap
}

func RandomId() string {
	return fmt.Sprint(rand.Uint32())
}

func tenantId(existing database.Id) database.Id {
	if existing == 0 {
		return database.DefaultTenantId
	}
	return existing
}

func (f FakeSwaps) Create(t *testing.T, db *database.Database) {
	for _, swap := range f.Swaps {
		swap.TenantId = tenantId(swap.TenantId)
		swap.Id = RandomId()
		require.NoError(t, db.CreateSwap(swap))
	}

	for _, reverseSwap := range f.ReverseSwaps {
		reverseSwap.TenantId = tenantId(reverseSwap.TenantId)
		reverseSwap.Id = RandomId()
		require.NoError(t, db.CreateReverseSwap(reverseSwap))
	}

	for _, chainSwap := range f.ChainSwaps {
		chainSwap.TenantId = tenantId(chainSwap.TenantId)
		id := RandomId()
		chainSwap.Id = id
		chainSwap.Pair = boltz.Pair{
			From: boltz.CurrencyLiquid,
			To:   boltz.CurrencyBtc,
		}
		if chainSwap.FromData == nil {
			chainSwap.FromData = &database.ChainSwapData{}
		}
		if chainSwap.ToData == nil {
			chainSwap.ToData = &database.ChainSwapData{}
		}
		chainSwap.FromData.Id = id
		chainSwap.FromData.Currency = chainSwap.Pair.From
		chainSwap.ToData.Id = id
		chainSwap.ToData.Currency = chainSwap.Pair.To
		require.NoError(t, db.CreateChainSwap(chainSwap))
	}
}
func PastDate(duration time.Duration) time.Time {
	return time.Now().Add(-duration)
}

func PrintBackendLogs() {
	fmt.Println(bash("docker logs boltz-backend"))
}
