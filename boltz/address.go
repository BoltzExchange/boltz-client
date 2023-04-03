package boltz

import (
	"crypto/sha256"
	"errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
)

func CheckSwapAddress(chainParams *chaincfg.Params, address string, redeemScript []byte, isNested bool) error {
	var err error
	var encodedAddress string

	if isNested {
		encodedAddress, err = NestedScriptHashAddress(chainParams, redeemScript)
	} else {
		encodedAddress, err = WitnessScriptHashAddress(chainParams, redeemScript)
	}

	if err != nil {
		return errors.New("could not encode address")
	}

	if address != encodedAddress {
		return errors.New("invalid address")
	}

	return nil
}

func WitnessScriptHashAddress(chainParams *chaincfg.Params, redeemScript []byte) (string, error) {
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
