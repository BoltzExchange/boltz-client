package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/config"
	"github.com/BoltzExchange/boltz-client/electrum"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/mempool"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/utils"
)

// TODO: close dangling channels

func main() {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(defaultDataDir)
	if err != nil {
		fmt.Println("Could not load config: " + err.Error())
		os.Exit(1)
	}

	logger.Init(cfg.LogFile, cfg.LogLevel)

	formattedCfg, err := utils.FormatJson(cfg)

	if err != nil {
		logger.Fatal("Could not format config: " + err.Error())
	}

	logger.Info("Parsed config and CLI arguments: " + formattedCfg)

	if strings.HasSuffix(defaultDataDir, "boltz-lnd") {
		logger.Warn("You still have data in the .boltz-lnd folder - please rename to .boltz")
	}

	Init(cfg)
	Start(cfg)
}

func Init(cfg *config.Config) {

	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	if !cfg.Standalone {
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

		cfg.Network = info.Network

		waitForLightningSynced(cfg.Lightning)

		if err := lightning.ConnectBoltz(cfg.Lightning, cfg.Boltz); err != nil {
			logger.Warn("Could not connect to to boltz node: " + err.Error())
		}

	} else if cfg.Network == "" {
		logger.Fatal("Network must be set when no lightning node is configured")
	}

	network, err := boltz.ParseChain(cfg.Network)

	if err != nil {
		logger.Fatal("Could not parse chain: " + err.Error())
	}

	logger.Info("Parsed chain: " + network.Name)

	setBoltzEndpoint(cfg.Boltz, network)

	checkBoltzVersion(cfg.Boltz)

	onchain, err := initOnchain(cfg, network)
	if err != nil {
		logger.Fatalf("could not init onchain: %v", err)
	}

	autoSwapConfPath := path.Join(cfg.DataDir, "autoswap.toml")

	err = cfg.RPC.Init(network, cfg.Lightning, cfg.Boltz, cfg.Database, onchain, autoSwapConfPath)

	if err != nil {
		logger.Fatalf("Could not initialize Server: %v", err)
	}
}

func Start(cfg *config.Config) {
	errChannel := cfg.RPC.Start()

	err := <-errChannel

	if err != nil {
		logger.Fatal("Could not start gRPC server: " + err.Error())
	}
}

func setLightningNode(cfg *config.Config) {
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

func initOnchain(cfg *config.Config, network *boltz.Network) (*onchain.Onchain, error) {
	chain := &onchain.Onchain{
		Btc: &onchain.Currency{
			Tx: onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyBtc),
		},
		Liquid: &onchain.Currency{
			Tx: onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyLiquid),
		},
		Network: network,
	}

	chain.Init()

	if !wallet.Initialized() {
		err := wallet.Init(wallet.Config{
			DataDir: cfg.DataDir,
			Network: network,
			Debug:   false,
		})
		if err != nil {
			return nil, fmt.Errorf("could not init wallet: %v", err)
		}
	}

	if cfg.ElectrumUrl != "" {
		logger.Info("Using configured Electrum RPC: " + cfg.ElectrumUrl)
		client, err := electrum.NewClient(cfg.ElectrumUrl, cfg.ElectrumSSL)
		if err != nil {
			return nil, fmt.Errorf("could not connect to electrum: %v", err)
		}
		chain.Btc.Blocks = client
		chain.Btc.Tx = client
	}
	if cfg.ElectrumLiquidUrl != "" {
		logger.Info("Using configured Electrum Liquid RPC: " + cfg.ElectrumLiquidUrl)
		client, err := electrum.NewClient(cfg.ElectrumLiquidUrl, cfg.ElectrumLiquiLiquidSSL)
		if err != nil {
			return nil, fmt.Errorf("could not connect to electrum: %v", err)
		}
		chain.Liquid.Blocks = client
		chain.Liquid.Tx = client
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
		client := mempool.InitClient(cfg.MempoolApi)
		chain.Btc.Blocks = client
		chain.Btc.Tx = client
	}

	if cfg.MempoolLiquidApi != "" {
		logger.Info("liquid.network API: " + cfg.MempoolLiquidApi)
		client := mempool.InitClient(cfg.MempoolLiquidApi)
		chain.Liquid.Blocks = client
		chain.Liquid.Tx = client
	}

	// use boltz in regtest to avoid situations where electrum doesnt know about the tx yet
	if network == boltz.Regtest {
		chain.Btc.Tx = onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyBtc)
		chain.Liquid.Tx = onchain.NewBoltzTxProvider(cfg.Boltz, boltz.CurrencyLiquid)
	}

	return chain, nil
}
