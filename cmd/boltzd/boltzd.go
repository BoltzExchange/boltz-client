package main

import (
	"errors"
	"github.com/BoltzExchange/boltz-lnd"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"strings"
)

// TODO: LND and Boltz compatibility checks
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

	setBoltzEndpoint(cfg.Boltz, chainParams.Name)
	cfg.Boltz.Init(symbol)

	boltzPubKey, err := connectBoltzLnd(cfg.LND, cfg.Boltz, symbol)

	if err != nil {
		logger.Warning("Could not connect to to Boltz LND node: " + err.Error())
	}

	swapNursery := &nursery.Nursery{}
	err = swapNursery.Init(symbol, boltzPubKey, chainParams, cfg.LND, cfg.Boltz, cfg.Database)

	if err != nil {
		logger.Fatal("Could no start Swap nursery: " + err.Error())
	}

	err = cfg.RPC.Start(symbol, chainParams, cfg.LND, cfg.Boltz, swapNursery, cfg.Database)

	if err != nil {
		logger.Fatal("Could not start RPC server: " + err.Error())
	}
}

// TODO: litecoin support
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

// TODO: handle cases in which the nodes are already connected
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

	logger.Info("Connected to Boltz LND node: " + node.URIs[0])

	return node.NodeKey, err
}

func setBoltzEndpoint(boltz *boltz.Boltz, chain string) {
	if boltz.URL != "" {
		logger.Info("Using configured Boltz endpoint: " + boltz.URL)
		return
	}

	switch chain {
	case "mainnet":
		boltz.URL = "https://boltz.exchange/api"
	case "testnet3":
		boltz.URL = "https://testnet.boltz.exchange/api"
	case "regtest":
		boltz.URL = "http://127.0.0.1:9001"
	}

	logger.Info("Using default Boltz endpoint for chain " + chain + ": " + boltz.URL)
}
