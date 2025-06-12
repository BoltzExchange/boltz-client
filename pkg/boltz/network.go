package boltz

import (
	"errors"

	btc "github.com/btcsuite/btcd/chaincfg"
	liquid "github.com/vulpemventures/go-elements/network"
)

type Network struct {
	Btc                *btc.Params
	Liquid             *liquid.Network
	Name               string
	DummyLockupAddress map[Currency]string
	DefaultBoltzUrl    string
}

var MainNet = &Network{
	Btc:    &btc.MainNetParams,
	Liquid: &liquid.Liquid,
	Name:   "mainnet",
	DummyLockupAddress: map[Currency]string{
		CurrencyBtc:    "bc1p28f027j7nte0pprte30nz4qxx65uc3rur23pukjganmzfwejj5lqjq5lky",
		CurrencyLiquid: "lq1pqtfldcsfag6u5lv20f85zjp68x99er90jxlqv3yc3ucy9zd3tt0ndztxkr9jaxynl8l4hvsfch7slg7l52pfw49te3wrhwazr9lq9s6y2cgwtpn9wv7z",
	},
	DefaultBoltzUrl: "https://api.boltz.exchange",
}

var TestNet = &Network{
	Btc:    &btc.TestNet3Params,
	Liquid: &liquid.Testnet,
	Name:   "testnet",
	DummyLockupAddress: map[Currency]string{
		CurrencyBtc:    "tb1p5a2rc0hcuf8n2rssmfr9mqk08nlxzl9ngnlhj47gwegj7epjph5q9739y6",
		CurrencyLiquid: "tlq1pqghwg6s98dfhtrncxck6rl359eckxdwrk4680npy4m6q2lgud9y6p0w2jytj4akr2zhwze587d823zu5rg8vwfq0ehkk8c74lrvt77kmwqr5vwy7p47u",
	},
	DefaultBoltzUrl: "https://api.testnet.boltz.exchange",
}

var Regtest = &Network{
	Btc:    &btc.RegressionNetParams,
	Liquid: &liquid.Regtest,
	Name:   "regtest",
	DummyLockupAddress: map[Currency]string{
		CurrencyBtc:    "bcrt1pedm5v4z658f3ad4gyxmnren7gdnnqhm6pdtgheksfvm8f4k74uas7tz83f",
		CurrencyLiquid: "el1pqfg7mxz4cnpu8sj2pza285vh062eq0sxwt982nprnx0d975tvmzpdcqdwvpsds5q664fp90645wlze8544j8x59vzhhy6hylmad6ycjw07nsa6thmkz7",
	},
	DefaultBoltzUrl: "http://127.0.0.1:9001",
}

func ParseChain(network string) (*Network, error) {
	switch network {
	case "mainnet", "bitcoin":
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
