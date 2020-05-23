package boltz

import (
	"errors"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

func CheckSwapAddress(chainParams *chaincfg.Params, address string, redeemScript []byte) error {
	addressScript := createNestedP2shScript(redeemScript)
	encodedAddress, err := scriptHashAddress(chainParams, addressScript)

	if err != nil {
		return errors.New("could not encode address")
	}

	if address != encodedAddress {
		return errors.New("invalid address")
	}

	return nil
}

func scriptHashAddress(chainParams *chaincfg.Params, redeemScript []byte) (string, error) {
	address, err := btcutil.NewAddressScriptHash(redeemScript, chainParams)

	if err != nil {
		return "", err
	}

	return address.EncodeAddress(), err
}
