package boltz

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
)

func mnemonicToHdKey(mnemonic string, network *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate seed: %w", err)
	}

	return hdkeychain.NewMaster(seed, network)
}

func deriveKey(hdKey *hdkeychain.ExtendedKey, index uint32) (*hdkeychain.ExtendedKey, error) {
	path := []uint32{44, 0, 0, 0, index}
	extendedKey, err := hdKey.Derive(path[0])
	if err != nil {
		return nil, err
	}

	for _, p := range path[1:] {
		extendedKey, err = extendedKey.Derive(p)
		if err != nil {
			return nil, err
		}
	}
	return extendedKey, nil
}

func DeriveKey(mnemonic string, index uint32, network *chaincfg.Params) (*btcec.PrivateKey, error) {
	hdKey, err := mnemonicToHdKey(mnemonic, network)
	if err != nil {
		return nil, err
	}

	extendedKey, err := deriveKey(hdKey, index)
	if err != nil {
		return nil, err
	}

	return extendedKey.ECPrivKey()
}