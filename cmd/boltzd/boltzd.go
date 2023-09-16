package main

import (
	boltz_lnd "github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/mempool"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/BoltzExchange/boltz-lnd/utils"
	bitcoinCfg "github.com/btcsuite/btcd/chaincfg"
)

// TODO: close dangling channels

func main() {
	cfg := boltz_lnd.LoadConfig()

	logger.InitLogger(cfg.LogFile, cfg.LogPrefix)

	formattedCfg, err := utils.FormatJson(cfg)

	if err != nil {
		logger.Fatal("Could not format config: " + err.Error())
	}

	logger.Info("Parsed config and CLI arguments: " + formattedCfg)

	Init(cfg)
	Start(cfg)
}

func Init(cfg *boltz_lnd.Config) {

	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	err = cfg.Lightning.Connect()
	err = cfg.LND.Connect()

	if err != nil {
		logger.Fatal("Could not initialize lightning client: " + err.Error())
	}

	info := connectLightning(cfg.Lightning)

	checkLightningVersion(info)

	logger.Info("Connected to lightning node: " + info.Pubkey + " (" + info.Version + ")")

	chainParams := parseChain(info.Network)
	logger.Info("Parsed chain: " + chainParams.Name)

	cfg.LND.ChainParams = chainParams

	waitForLightningSynced(cfg.Lightning)

	setBoltzEndpoint(cfg.Boltz, chainParams.Name)

	checkBoltzVersion(cfg.Boltz)

	boltzPubKey, err := utils.ConnectBoltzLnd(cfg.LND, cfg.Boltz)

	if err != nil {
		logger.Warning("Could not connect to to Boltz LND node: " + err.Error())
	}

	swapNursery := &nursery.Nursery{}
	err = swapNursery.Init(
		boltzPubKey,
		chainParams,
		cfg.LND,
		cfg.Boltz,
		mempool.Init(cfg.LND, cfg.MempoolApi),
		cfg.Database,
	)

	if err != nil {
		logger.Fatal("Could not start Swap nursery: " + err.Error())
	}

	err = cfg.RPC.Init(chainParams, cfg.LND, cfg.Boltz, swapNursery, cfg.Database)

	if err != nil {
		logger.Fatal("Could not initialize Server" + err.Error())
	}
}

func Start(cfg *boltz_lnd.Config) {
	errChannel := cfg.RPC.Start()

	err := <-errChannel

	if err != nil {
		logger.Fatal("Could not start gRPC server: " + err.Error())
	}
}

func parseChain(network string) (params *bitcoinCfg.Params) {

	switch network {
	case "mainnet":
		// #reckless
		params = &bitcoinCfg.MainNetParams
	case "testnet":
		params = &bitcoinCfg.TestNet3Params
	case "regtest":
		params = &bitcoinCfg.RegressionNetParams
	default:
		logger.Fatal("Network " + network + " no supported")
	}

	return params
}

func setBoltzEndpoint(boltz *boltz.Boltz, chain string) {
	if boltz.URL != "" {
		logger.Info("Using configured Boltz endpoint: " + boltz.URL)
		return
	}

	switch chain {
	case bitcoinCfg.MainNetParams.Name:
		boltz.URL = "https://boltz.exchange/api"
	case bitcoinCfg.TestNet3Params.Name:
		boltz.URL = "https://testnet.boltz.exchange/api"
	case bitcoinCfg.RegressionNetParams.Name:
		boltz.URL = "http://127.0.0.1:9001"
	}

	logger.Info("Using default Boltz endpoint for chain " + chain + ": " + boltz.URL)
}
