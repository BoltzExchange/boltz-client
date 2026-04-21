package test

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	bitcoin_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
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

const WalletMnemonic = "fog pen possible deer cool muscle describe awkward enforce injury pelican ridge used enrich female enrich museum verify emotion ask office tonight primary large"
const WalletId = 1

func WalletInfo(currency boltz.Currency) onchain.WalletInfo {
	return onchain.WalletInfo{
		Name:     "regtest",
		Currency: currency,
		TenantId: database.DefaultTenantId,
		Id:       WalletId,
	}
}

func WalletCredentials(currency boltz.Currency) *onchain.WalletCredentials {
	mnemonic, _ := liquid_wallet.GenerateMnemonic(boltz.Regtest)
	creds := &onchain.WalletCredentials{
		WalletInfo: WalletInfo(currency),
		Mnemonic:   mnemonic,
	}
	if currency == boltz.CurrencyBtc {
		creds.CoreDescriptor, _ = bitcoin_wallet.DeriveDefaultDescriptor(boltz.Regtest, mnemonic)
	}
	if currency == boltz.CurrencyLiquid {
		creds.CoreDescriptor, _ = liquid_wallet.DeriveDefaultDescriptor(boltz.Regtest, mnemonic)
	}
	return creds
}

func FundWallet(currency boltz.Currency, wallet onchain.Wallet) error {
	if err := wallet.FullScan(); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		addr, err := wallet.NewAddress()
		if err != nil {
			return err
		}

		tx := SendToAddress(GetCli(currency), addr, 1_000_000)
		txHex := GetRawTransaction(GetCli(currency), tx)
		if err := wallet.ApplyTransaction(txHex); err != nil {
			return err
		}
	}
	MineBlock()
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout")
		case <-ticker.C:
			balance, err := wallet.GetBalance()
			if err != nil {
				return err
			}
			if balance.Confirmed > 0 && balance.Confirmed == balance.Total {
				return nil
			}
			err = wallet.FullScan()
			if err != nil {
				return err
			}
		}
	}
}

func dbPersister(t *testing.T) liquid_wallet.Persister {
	db := database.Database{
		Path: ":memory:",
	}
	err := db.Connect()
	require.NoError(t, err)
	err = db.CreateWallet(&database.Wallet{
		WalletCredentials: WalletCredentials(boltz.CurrencyLiquid),
	})
	require.NoError(t, err)
	return database.NewWalletPersister(&db)
}

func boltzChainProvider(t *testing.T, currency boltz.Currency) onchain.ChainProvider {
	return onchain.NewBoltzChainProvider(
		&boltz.Api{URL: boltz.Regtest.DefaultBoltzUrl},
		currency,
	)
}

func LiquidBackendConfig(t *testing.T) liquid_wallet.Config {
	return liquid_wallet.Config{
		Network:       boltz.Regtest,
		DataDir:       t.TempDir(),
		Persister:     dbPersister(t),
		ChainProvider: boltzChainProvider(t, boltz.CurrencyLiquid),
	}
}

func WalletBackend(t *testing.T, currency boltz.Currency) onchain.WalletBackend {
	var backend onchain.WalletBackend
	var err error
	switch currency {
	case boltz.CurrencyBtc:
		backend, err = bitcoin_wallet.NewBackend(bitcoin_wallet.Config{
			Network:       boltz.Regtest,
			Electrum:      onchain.RegtestElectrumConfig.Btc,
			DataDir:       t.TempDir(),
			ChainProvider: boltzChainProvider(t, boltz.CurrencyBtc),
		})
	case boltz.CurrencyLiquid:
		backend, err = liquid_wallet.NewBackend(LiquidBackendConfig(t))
	default:
		require.Fail(t, "invalid currency")
	}
	require.NoError(t, err)
	return backend
}

func InitLogger() {
	logger.Init(logger.Options{Level: "debug"})
}

func BtcCli(cmd string) string {
	return run("bitcoin-cli-sim-client " + cmd)
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

func GetRawTransaction(cli Cli, txId string) string {
	return cli("getrawtransaction " + txId)
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
