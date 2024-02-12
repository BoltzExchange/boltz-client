package boltz

import (
	"errors"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/vulpemventures/go-elements/address"
)

func ValidateAddress(network *Network, rawAddress string, currency Currency) error {
	var err error
	if currency == CurrencyBtc {
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
