package boltz

import (
	"crypto/sha256"
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/vulpemventures/go-elements/address"
)

func CheckSwapAddress(pair Pair, net *Network, address string, redeemScript []byte, isNested bool, blindingKey *btcec.PublicKey) error {
	var err error
	var encodedAddress string

	if pair == PairLiquid {
		encodedAddress, err = LiquidWitnessScriptHashAddress(net, redeemScript, blindingKey)
	} else {
		if isNested {
			encodedAddress, err = NestedScriptHashAddress(net.Btc, redeemScript)
		} else {
			encodedAddress, err = BtcWitnessScriptHashAddress(net.Btc, redeemScript)
		}
	}

	if err != nil {
		return errors.New("could not encode address")
	}

	if address != encodedAddress {
		return errors.New("invalid address")
	}

	return nil
}

func WitnessScriptHashAddress(pair Pair, net *Network, redeemScript []byte, blindingKey *btcec.PublicKey) (string, error) {
	if pair == PairLiquid {
		return LiquidWitnessScriptHashAddress(net, redeemScript, blindingKey)
	} else {
		return BtcWitnessScriptHashAddress(net.Btc, redeemScript)
	}
}

func BtcWitnessScriptHashAddress(chainParams *chaincfg.Params, redeemScript []byte) (string, error) {
	hash := sha256.Sum256(redeemScript)
	address, err := btcutil.NewAddressWitnessScriptHash(hash[:], chainParams)

	if err != nil {
		return "", err
	}

	return address.EncodeAddress(), err
}

func ScriptHashAddress(chainParams *chaincfg.Params, redeemScript []byte) (string, error) {
	address, err := btcutil.NewAddressScriptHash(redeemScript, chainParams)

	if err != nil {
		return "", err
	}

	return address.EncodeAddress(), err
}

func NestedScriptHashAddress(chainParams *chaincfg.Params, redeemScript []byte) (string, error) {
	addressScript := createNestedP2shScript(redeemScript)
	encodedAddress, err := ScriptHashAddress(chainParams, addressScript)

	return encodedAddress, err
}

func ValidateAddress(network *Network, rawAddress string, pair Pair) error {
	var err error
	if pair == PairBtc {
		var address btcutil.Address
		address, err = btcutil.DecodeAddress(rawAddress, network.Btc)
		if _, ok := address.(*btcutil.AddressPubKey); ok {
			err = errors.New("p2pk addresses are not allowed")
		}
	} else {
		// elements library does not implement p2pk addresses, so we dont have to check for that
		_, err = address.DecodeType(rawAddress)
	}
	return err
}
