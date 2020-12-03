package main

import (
	"github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/BoltzExchange/boltz-lnd/utils"
	bitcoinCfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	litecoinCfg "github.com/ltcsuite/ltcd/chaincfg"
)

func main() {
	cfg := boltz_lnd.LoadConfig()

	logger.InitLogger(cfg.LogFile, cfg.LogPrefix)
	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	err = cfg.LND.Connect()

	if err != nil {
		logger.Fatal("Could not initialize LND client: " + err.Error())
	}

	lndInfo := connectToLnd(cfg.LND)

	checkLndVersion(lndInfo)

	symbol, chainParams := parseChain(lndInfo.Chains[0])
	logger.Info("Parsed chain: " + symbol + " " + chainParams.Name)

	waitForLndSynced(cfg.LND)

	setBoltzEndpoint(cfg.Boltz, chainParams.Name)
	cfg.Boltz.Init(symbol)

	checkBoltzVersion(cfg.Boltz)

	boltzPubKey, err := utils.ConnectBoltzLnd(cfg.LND, cfg.Boltz, symbol)

	if err != nil {
		logger.Warning("Could not connect to to Boltz LND node: " + err.Error())
	}

	swapNursery := &nursery.Nursery{}
	err = swapNursery.Init(symbol, boltzPubKey, chainParams, cfg.LND, cfg.Boltz, cfg.Database)

	if err != nil {
		logger.Fatal("Could not start Swap nursery: " + err.Error())
	}

	err = cfg.RPC.Start(symbol, chainParams, cfg.LND, cfg.Boltz, swapNursery, cfg.Database)

	if err != nil {
		logger.Fatal("Could not start gRPC server: " + err.Error())
	}
}

func parseChain(chain *lnrpc.Chain) (symbol string, params *bitcoinCfg.Params) {
	switch chain.Chain {
	case "bitcoin":
		symbol = "BTC"
	case "litecoin":
		symbol = "LTC"
	default:
		logger.Fatal("Chain " + chain.Chain + " not supported")
	}

	switch symbol {
	case "BTC":
		switch chain.Network {
		case "mainnet":
			// #reckless
			params = &bitcoinCfg.MainNetParams
		case "testnet":
			params = &bitcoinCfg.TestNet3Params
		case "regtest":
			params = &bitcoinCfg.RegressionNetParams
		default:
			logger.Fatal("Chain " + chain.Network + " no supported")
		}

	case "LTC":
		switch chain.Network {
		case "mainnet":
			// #reckless
			params = utils.ApplyLitecoinParams(litecoinCfg.MainNetParams)
		case "testnet":
			params = utils.ApplyLitecoinParams(litecoinCfg.TestNet4Params)
		case "regtest":
			params = utils.ApplyLitecoinParams(litecoinCfg.RegressionNetParams)
		default:
			logger.Fatal("Chain " + chain.Network + " no supported")
		}
	}

	return symbol, params
}

func setBoltzEndpoint(boltz *boltz.Boltz, chain string) {
	if boltz.URL != "" {
		logger.Info("Using configured Boltz endpoint: " + boltz.URL)
		return
	}

	switch chain {
	case bitcoinCfg.MainNetParams.Name:
		boltz.URL = "https://boltz.exchange/api"
	case bitcoinCfg.TestNet3Params.Name, litecoinCfg.TestNet4Params.Name:
		boltz.URL = "https://testnet.boltz.exchange/api"
	case bitcoinCfg.RegressionNetParams.Name:
		boltz.URL = "http://127.0.0.1:9001"
	}

	logger.Info("Using default Boltz endpoint for chain " + chain + ": " + boltz.URL)
}
