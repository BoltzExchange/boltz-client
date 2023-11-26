package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	boltzClient "github.com/BoltzExchange/boltz-client"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/mempool"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/liquid"
	"github.com/BoltzExchange/boltz-client/utils"
)

// TODO: close dangling channels

func main() {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	cfg := boltzClient.LoadConfig(defaultDataDir)

	logger.Init(cfg.LogFile, cfg.LogLevel)

	formattedCfg, err := utils.FormatJson(cfg)

	if err != nil {
		logger.Fatal("Could not format config: " + err.Error())
	}

	logger.Info("Parsed config and CLI arguments: " + formattedCfg)

	Init(cfg)
	Start(cfg)
}

func Init(cfg *boltzClient.Config) {

	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	setLightningNode(cfg)

	if err = cfg.Lightning.Connect(); err != nil {
		logger.Fatal("Could not initialize lightning client: " + err.Error())
	}

	info := connectLightning(cfg.Lightning)

	if cfg.Lightning == cfg.Cln {
		checkClnVersion(info)
	} else if cfg.Lightning == cfg.LND {
		checkLndVersion(info)
	}

	logger.Info(fmt.Sprintf("Connected to lightning node %v (%v): %v", cfg.Node, info.Version, info.Pubkey))

	network, err := boltz.ParseChain(info.Network)

	if err != nil {
		logger.Fatal("Could not parse chain: " + err.Error())
	}

	logger.Info("Parsed chain: " + network.Name)

	cfg.LND.ChainParams = network.Btc

	waitForLightningSynced(cfg.Lightning)

	setBoltzEndpoint(cfg.Boltz, network)

	checkBoltzVersion(cfg.Boltz)

	_, err = utils.ConnectBoltz(cfg.Lightning, cfg.Boltz)

	if err != nil {
		logger.Warn("Could not connect to to Boltz LND node: " + err.Error())
	}

	onchain := &onchain.Onchain{
		Btc: &onchain.Currency{
			Listener: cfg.Lightning,
			Wallet:   cfg.Lightning,
		},
		Liquid:  &onchain.Currency{},
		Network: network,
	}

	if network == boltz.MainNet {
		cfg.MempoolApi = "https://mempool.space/api"
		cfg.MempoolLiquidApi = "https://liquid.network/api"
	} else if network == boltz.TestNet {
		cfg.MempoolApi = "https://mempool.space/testnet/api"
		cfg.MempoolLiquidApi = "https://liquid.network/liquidtestnet/api"
	}

	if cfg.MempoolApi != "" {
		logger.Info("mempool.space API: " + cfg.MempoolApi)
		mempoolBtc := mempool.InitClient(cfg.MempoolApi)
		onchain.Btc.Fees = mempoolBtc
		// TODO: boltz as alternative
		onchain.Btc.Tx = mempoolBtc
	}

	if cfg.MempoolLiquidApi != "" {
		logger.Info("liquid.network API: " + cfg.MempoolLiquidApi)
		mempoolLiquid := mempool.InitClient(cfg.MempoolLiquidApi)
		onchain.Liquid.Fees = mempoolLiquid
		onchain.Liquid.Listener = mempoolLiquid
		onchain.Liquid.Tx = mempoolLiquid
	}

	if cfg.LiquidWallet == nil {
		liquidWallet, err := liquid.InitWallet(cfg.DataDir, network, false)
		if err != nil {
			logger.Error("Could not init Liquid wallet: " + err.Error())
		} else if liquidWallet.Exists() {
			if err := liquidWallet.Login(); err != nil {
				logger.Error("Could not log into liquid wallet: " + err.Error())
			} else {
				logger.Info("Successfully logged into liquid wallet")
			}
		}
		onchain.Liquid.Wallet = liquidWallet
	} else {
		onchain.Liquid.Wallet = cfg.LiquidWallet
	}

	autoSwapConfPath := path.Join(cfg.DataDir, "autoswap.toml")

	err = cfg.RPC.Init(network, cfg.Lightning, cfg.Boltz, cfg.Database, onchain, autoSwapConfPath)

	if err != nil {
		logger.Fatalf("Could not initialize Server: %v", err)
	}
}

func Start(cfg *boltzClient.Config) {
	errChannel := cfg.RPC.Start()

	err := <-errChannel

	if err != nil {
		logger.Fatal("Could not start gRPC server: " + err.Error())
	}
}

func setLightningNode(cfg *boltzClient.Config) {
	isClnConfigured := cfg.Cln.RootCert != ""
	isLndConfigured := cfg.LND.Macaroon != ""

	if strings.EqualFold(cfg.Node, "CLN") {
		cfg.Lightning = cfg.Cln
	} else if strings.EqualFold(cfg.Node, "LND") {
		cfg.Lightning = cfg.LND
	} else if isClnConfigured && isLndConfigured {
		logger.Fatal("Both CLN and LND are configured. Set --node to specify which node to use.")
	} else if isClnConfigured {
		cfg.Lightning = cfg.Cln
	} else if isLndConfigured {
		cfg.Lightning = cfg.LND
	} else {
		logger.Fatal("No lightning node configured. Set either CLN or LND.")
	}
}

func setBoltzEndpoint(boltzCfg *boltz.Boltz, network *boltz.Network) {
	if boltzCfg.URL != "" {
		logger.Info("Using configured Boltz endpoint: " + boltzCfg.URL)
		return
	}

	switch network {
	case boltz.MainNet:
		boltzCfg.URL = "https://api.boltz.exchange"
	case boltz.TestNet:
		boltzCfg.URL = "https://testnet.boltz.exchange/api"
	case boltz.Regtest:
		boltzCfg.URL = "http://127.0.0.1:9001"
	}

	logger.Info("Using default Boltz endpoint for network " + network.Name + ": " + boltzCfg.URL)
}
