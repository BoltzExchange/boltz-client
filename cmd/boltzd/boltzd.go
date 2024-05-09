package main

import (
	"fmt"
	"net/http"
	"net/url"
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

func initLightning(cfg *config.Config) (lightning.LightningNode, error) {
	if cfg.Standalone {
		if cfg.Network == "" {
			logger.Fatal("Standalone mode requires a lightning node to be set")
		}
		return nil, nil
	}
	var node lightning.LightningNode

	isClnConfigured := cfg.Cln.RootCert != ""
	isLndConfigured := cfg.LND.Macaroon != ""

	if strings.EqualFold(cfg.Node, "CLN") {
		node = cfg.Cln
	} else if strings.EqualFold(cfg.Node, "LND") {
		node = cfg.LND
	} else if isClnConfigured && isLndConfigured {
		logger.Fatal("Both CLN and LND are configured. Set --node to specify which node to use.")
	} else if isClnConfigured {
		node = cfg.Cln
	} else if isLndConfigured {
		node = cfg.LND
	} else {
		logger.Fatal("No lightning node configured. Set either CLN or LND.")
	}

	info, err := connectLightning(node)
	if err != nil {
		return nil, fmt.Errorf("could not connect to lightning node: %v", err)
	}

	if node == cfg.Cln {
		checkClnVersion(info)
	} else if node == cfg.LND {
		checkLndVersion(info)
	}

	logger.Info(fmt.Sprintf("Connected to lightning node %v (%v): %v", cfg.Node, info.Version, info.Pubkey))

	cfg.Network = info.Network

	return node, nil
}

func Init(cfg *config.Config) {
	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	lightningNode, err := initLightning(cfg)
	if err != nil {
		logger.Fatal("Could not initialize lightning client: " + err.Error())
	}

	network, err := boltz.ParseChain(cfg.Network)

	if err != nil {
		logger.Fatal("Could not parse chain: " + err.Error())
	}

	logger.Info("Parsed chain: " + network.Name)

	chain, err := initOnchain(cfg, network)
	if err != nil {
		logger.Fatalf("could not init onchain: %v", err)
	}

	autoSwapConfPath := path.Join(cfg.DataDir, "autoswap.toml")

	boltzApi, err := initBoltz(cfg, network)
	if err != nil {
		logger.Fatalf("could not init Boltz API: %v", err)
	}

	// Use the Boltz API in regtest to avoid situations where electrum does not know about the tx yet
	if network == boltz.Regtest {
		chain.Btc.Tx = onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyBtc)
		chain.Liquid.Tx = onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyLiquid)
	}

	if lightningNode != nil {
		if err := lightning.ConnectBoltz(lightningNode, boltzApi); err != nil {
			logger.Warn("Could not connect to to boltz node: " + err.Error())
		}
	}

	err = cfg.RPC.Init(network, lightningNode, boltzApi, cfg.Database, chain, autoSwapConfPath)

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

func initBoltz(cfg *config.Config, network *boltz.Network) (*boltz.Api, error) {
	boltzUrl := cfg.Boltz.URL
	if boltzUrl == "" {
		switch network {
		case boltz.MainNet:
			boltzUrl = "https://api.boltz.exchange"
		case boltz.TestNet:
			boltzUrl = "https://api.testnet.boltz.exchange"
		case boltz.Regtest:
			boltzUrl = "http://127.0.0.1:9001"
		}
		logger.Infof("Using default Boltz endpoint for network %s: %s", network.Name, boltzUrl)
	} else {
		logger.Info("Using configured Boltz endpoint: " + boltzUrl)
	}

	boltzApi := &boltz.Api{URL: boltzUrl}
	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxy)
		boltzApi.Client = http.Client{
			Transport: transport,
		}
	}

	checkBoltzVersion(boltzApi)

	return boltzApi, nil
}

func initOnchain(cfg *config.Config, network *boltz.Network) (*onchain.Onchain, error) {
	chain := &onchain.Onchain{
		Btc:     &onchain.Currency{},
		Liquid:  &onchain.Currency{},
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

	return chain, nil
}
