package utils

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

func LoadSeedPhrase(phrase string) (*btcec.PrivateKey, error) {

	seed, err := bip39.NewSeedWithErrorChecking(phrase, "")
	if err != nil {
		return nil, err
	}

	key, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}

	child, err := key.NewChildKey(0)
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(child.Key)
	return privKey, err
}
