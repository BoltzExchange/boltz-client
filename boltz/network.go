package boltz

import (
	"errors"

	btc "github.com/btcsuite/btcd/chaincfg"
	liquid "github.com/vulpemventures/go-elements/network"
)

type Network struct {
	Btc    *btc.Params
	Liquid *liquid.Network
	Name   string
}

var MainNet = &Network{
	Btc:    &btc.MainNetParams,
	Liquid: &liquid.Liquid,
	Name:   "mainnet",
}

var TestNet = &Network{
	Btc:    &btc.TestNet3Params,
	Liquid: &liquid.Testnet,
	Name:   "testnet",
}

var Regtest = &Network{
	Btc:    &btc.RegressionNetParams,
	Liquid: &liquid.Regtest,
	Name:   "regtest",
}

func ParseChain(network string) (*Network, error) {
	switch network {
	case "mainnet":
		// #reckless
		return MainNet, nil
	case "testnet":
		return TestNet, nil
	case "regtest":
		return Regtest, nil
	default:
		return nil, errors.New("Network " + network + " not supported")
	}
}
