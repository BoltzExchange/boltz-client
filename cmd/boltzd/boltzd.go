package main

import (
	"errors"
	"github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	bitcoinCfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	litecoinCfg "github.com/ltcsuite/ltcd/chaincfg"
	"strings"
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

	boltzPubKey, err := connectBoltzLnd(cfg.LND, cfg.Boltz, symbol)

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
		logger.Fatal("Could not start RPC server: " + err.Error())
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
			params = boltz_lnd.ApplyLitecoinParams(litecoinCfg.MainNetParams)
		case "testnet":
			params = boltz_lnd.ApplyLitecoinParams(litecoinCfg.TestNet4Params)
		case "regtest":
			params = boltz_lnd.ApplyLitecoinParams(litecoinCfg.RegressionNetParams)
		default:
			logger.Fatal("Chain " + chain.Network + " no supported")
		}
	}

	return symbol, params
}

func connectBoltzLnd(lnd *lnd.LND, boltz *boltz.Boltz, symbol string) (string, error) {
	nodes, err := boltz.GetNodes()

	if err != nil {
		return "", err
	}

	node, hasNode := nodes.Nodes[symbol]

	if !hasNode {
		return "", errors.New("could not find Boltz LND node for symbol: " + symbol)
	}

	if len(node.URIs) == 0 {
		return node.NodeKey, errors.New("could not find URIs for Boltz LND node for symbol: " + symbol)
	}

	uriParts := strings.Split(node.URIs[0], "@")

	if len(uriParts) != 2 {
		return node.NodeKey, errors.New("could not parse URI of Boltz LND")
	}

	_, err = lnd.ConnectPeer(uriParts[0], uriParts[1])

	if err == nil {
		logger.Info("Connected to Boltz LND node: " + node.URIs[0])
	} else if strings.HasPrefix(err.Error(), "rpc error: code = Unknown desc = already connected to peer") {
		logger.Info("Already connected to Boltz LND node: " + node.URIs[0])
		err = nil
	}

	return node.NodeKey, err
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
