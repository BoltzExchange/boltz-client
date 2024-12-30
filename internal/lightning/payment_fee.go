package lightning

import (
	"math"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

const (
	minPaymentFee              = 21
	maxPaymentFeeRatio float64 = 0.03
)

// getFeeLimit calculates the fee limit of a payment in sat
func GetFeeLimit(invoice string, chainParams *chaincfg.Params) (uint, error) {
	decodedInvoice, err := zpay32.Decode(invoice, chainParams)

	if err != nil {
		return 0, err
	}

	// Use the minimum value for small payments
	feeLimit := math.Max(
		decodedInvoice.MilliSat.ToSatoshis().MulF64(maxPaymentFeeRatio).ToUnit(btcutil.AmountSatoshi),
		minPaymentFee,
	)

	return uint(feeLimit), nil
}
