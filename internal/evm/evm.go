package evm

import (
	"math/big"
	"strings"
)

// WeiFactor is the conversion factor between satoshis and wei.
// 1 BTC = 10^8 satoshis, 1 ETH/RBTC = 10^18 wei, difference = 10^10.
var WeiFactor = new(big.Int).Exp(big.NewInt(10), big.NewInt(10), nil)

// ChainName identifies an EVM chain in configuration.
type ChainName string

const (
	ChainArbitrum  ChainName = "arbitrum"
	ChainRootstock ChainName = "rootstock"
	ChainEthereum  ChainName = "ethereum"
)

// SatoshiToWei converts a satoshi amount to wei.
func SatoshiToWei(satoshis uint64) *big.Int {
	sat := new(big.Int).SetUint64(satoshis)
	return sat.Mul(sat, WeiFactor)
}

// WeiToSatoshi converts a wei amount to satoshis (truncates).
func WeiToSatoshi(wei *big.Int) uint64 {
	sat := new(big.Int).Div(wei, WeiFactor)
	return sat.Uint64()
}

// Prefix0x ensures a hex string has the "0x" prefix.
func Prefix0x(hex string) string {
	if strings.HasPrefix(hex, "0x") || strings.HasPrefix(hex, "0X") {
		return hex
	}

	return "0x" + hex
}

// Strip0x removes the "0x" prefix from a hex string if present.
func Strip0x(hex string) string {
	return strings.TrimPrefix(strings.TrimPrefix(hex, "0x"), "0X")
}
