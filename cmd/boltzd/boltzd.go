package main

import (
	"github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
)

func main() {
	cfg := boltz_lnd.LoadConfig()

	boltz_lnd.InitLogger(cfg.LogFile)
	err := cfg.Database.Connect()

	if err != nil {
		logger.Fatal("Could not connect to database: " + err.Error())
	}

	err = cfg.LND.Connect()

	if err != nil {
		logger.Fatal("Could not initialize LND client: " + err.Error())
	}

	lndInfo, err := cfg.LND.GetInfo()

	if err != nil {
		logger.Fatal("Could not connect to LND: " + err.Error())
		return
	}

	symbol, chainParams := parseChain(lndInfo.Chains[0])
	logger.Info("Parsed chain: " + symbol + " " + chainParams.Name)

	swapNursery := &nursery.Nursery{}
	err = swapNursery.Init(chainParams, cfg.LND, cfg.Boltz, cfg.Database)

	if err != nil {
		logger.Fatal("Could no start Swap nursery: " + err.Error())
	}

	err = cfg.RPC.Start(symbol, chainParams, cfg.LND, cfg.Boltz, swapNursery, cfg.Database)

	if err != nil {
		logger.Fatal("Could not start RPC server: " + err.Error())
	}
}

func parseChain(chain *lnrpc.Chain) (symbol string, params *chaincfg.Params) {
	switch chain.Chain {
	case "bitcoin":
		symbol = "BTC"
	case "litecoin":
		symbol = "LTC"
	default:
		logger.Fatal("Chain " + chain.Chain + " not supported")
	}

	switch chain.Network {
	case "mainnet":
		// #reckless
		params = &chaincfg.MainNetParams
	case "testnet":
		params = &chaincfg.TestNet3Params
	case "regtest":
		params = &chaincfg.RegressionNetParams
	default:
		logger.Fatal("Chain " + chain.Network + " no supported")
	}

	return symbol, params
}
