package test

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

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

func ClearWalletDataDir() error {
	return os.RemoveAll(walletDataDir)
}

const WalletMnemonic = "fog pen possible deer cool muscle describe awkward enforce injury pelican ridge used enrich female enrich museum verify emotion ask office tonight primary large"
const WalletSubaccount = 0
const WalletId = 1

func WalletCredentials(currency boltz.Currency) *onchain.WalletCredentials {
	sub := uint64(WalletSubaccount)
	return &onchain.WalletCredentials{
		WalletInfo: onchain.WalletInfo{
			Name:     "regtest",
			Currency: currency,
			TenantId: database.DefaultTenantId,
			Id:       WalletId,
		},
		Mnemonic:   WalletMnemonic,
		Subaccount: &sub,
	}
}

func FundWallet(currency boltz.Currency, wallet onchain.Wallet) error {
	if err := wallet.Sync(); err != nil {
		return err
	}
	balance, err := wallet.GetBalance()
	if err != nil {
		return err
	}
	if balance.Total > 0 {
		return nil
	}

	amount := uint64(10000000)
	txes := 3
	for i := 0; i < txes; i++ {
		addr, err := wallet.NewAddress()
		if err != nil {
			return err
		}
		SendToAddress(GetCli(currency), addr, amount)
	}
	time.Sleep(1 * time.Second)
	MineBlock()
	ticker := time.NewTicker(1 * time.Second)
	timeout := time.After(15 * time.Second)

	for balance.Total < uint64(txes)*amount {
		select {
		case <-ticker.C:
			balance, err = wallet.GetBalance()
			if err != nil {
				return err
			}
		case <-timeout:
			return fmt.Errorf("timeout")
		}
	}
	time.Sleep(1 * time.Second)
	return nil
}

func InitTestWallet(debug bool) (map[boltz.Currency]*wallet.Wallet, error) {
	InitLogger()
	if err := ClearWalletDataDir(); err != nil {
		return nil, err
	}
	if !wallet.Initialized() {
		err := wallet.Init(wallet.Config{
			DataDir:                  walletDataDir,
			Network:                  boltz.Regtest,
			Debug:                    debug,
			Electrum:                 onchain.RegtestElectrumConfig,
			AutoConsolidateThreshold: wallet.DefaultAutoConsolidateThreshold,
			MaxInputs:                wallet.MaxInputs,
		})
		if err != nil {
			return nil, err
		}
	}

	result := make(map[boltz.Currency]*wallet.Wallet)
	var eg errgroup.Group
	for _, currency := range []boltz.Currency{boltz.CurrencyBtc, boltz.CurrencyLiquid} {
		currency := currency
		eg.Go(func() error {
			wallet, err := wallet.Login(WalletCredentials(currency))
			if err != nil {
				return err
			}
			time.Sleep(2 * time.Second)
			result[currency] = wallet
			return FundWallet(currency, wallet)
		})
	}
	return result, eg.Wait()
}

func InitLogger() {
	logger.Init(logger.Options{Level: "debug"})
}

func BtcCli(cmd string) string {
	return run("bitcoin-cli-sim-server " + cmd)
}

func GetCli(pair boltz.Currency) Cli {
	if pair == boltz.CurrencyLiquid {
		return LiquidCli
	} else {
		return BtcCli
	}
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

func BumpFee(cli Cli, txId string) string {
	return cli(fmt.Sprintf("bumpfee %s | jq -r .txid", txId))
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

func setSwapid(swapId *string) {
	if *swapId == "" {
		*swapId = RandomId()
	}
}

func (f FakeSwaps) Create(t *testing.T, db *database.Database) {
	for _, swap := range f.Swaps {
		swap.TenantId = tenantId(swap.TenantId)
		setSwapid(&swap.Id)
		require.NoError(t, db.CreateSwap(swap))
	}

	for _, reverseSwap := range f.ReverseSwaps {
		reverseSwap.TenantId = tenantId(reverseSwap.TenantId)
		setSwapid(&reverseSwap.Id)
		require.NoError(t, db.CreateReverseSwap(reverseSwap))
	}

	for _, chainSwap := range f.ChainSwaps {
		chainSwap.TenantId = tenantId(chainSwap.TenantId)
		setSwapid(&chainSwap.Id)
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
		chainSwap.FromData.Id = chainSwap.Id
		chainSwap.FromData.Currency = chainSwap.Pair.From
		chainSwap.ToData.Id = chainSwap.Id
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

func WaitWalletTx(t *testing.T, txId string) {
	notifier := wallet.TransactionNotifier.Get()
	defer wallet.TransactionNotifier.Remove(notifier)
	WaitWalletNotifier(t, txId, notifier)
}

func WaitWalletNotifier(t *testing.T, txId string, notifier <-chan wallet.TransactionNotification) {
	timeout := time.After(30 * time.Second)
	for {
		select {
		case notification := <-notifier:
			if notification.TxId == txId || txId == "" {
				return
			}
		case <-timeout:
			require.Fail(t, "timed out while waiting for tx")
		}
	}
}
