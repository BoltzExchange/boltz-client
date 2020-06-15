package boltz_lnd

import (
	bitcoinCfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	litecoinCfg "github.com/ltcsuite/ltcd/chaincfg"
)

func ApplyLitecoinParams(litecoinParams litecoinCfg.Params) *bitcoinCfg.Params {
	var bitcoinParams bitcoinCfg.Params

	bitcoinParams.Name = litecoinParams.Name
	bitcoinParams.Net = wire.BitcoinNet(litecoinParams.Net)
	bitcoinParams.DefaultPort = litecoinParams.DefaultPort

	bitcoinParams.Bech32HRPSegwit = litecoinParams.Bech32HRPSegwit

	bitcoinParams.PubKeyHashAddrID = litecoinParams.PubKeyHashAddrID
	bitcoinParams.ScriptHashAddrID = litecoinParams.ScriptHashAddrID
	bitcoinParams.PrivateKeyID = litecoinParams.PrivateKeyID
	bitcoinParams.WitnessPubKeyHashAddrID = litecoinParams.WitnessPubKeyHashAddrID
	bitcoinParams.WitnessScriptHashAddrID = litecoinParams.WitnessScriptHashAddrID

	bitcoinParams.HDPrivateKeyID = litecoinParams.HDPrivateKeyID
	bitcoinParams.HDPublicKeyID = litecoinParams.HDPublicKeyID

	bitcoinParams.HDCoinType = litecoinParams.HDCoinType

	return &bitcoinParams
}
