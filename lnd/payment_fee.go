package lnd

import (
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/zpay32"
	"math"
)

const (
	minPaymentFee              = 21
	maxPaymentFeeRatio float64 = 0.03
)

// getFeeLimit calculates the fee limit of a payment in sat
func (lnd *LND) getFeeLimit(invoice string) (int64, error) {
	decodedInvoice, err := zpay32.Decode(invoice, lnd.ChainParams)

	if err != nil {
		return 0, err
	}

	// Use the minimum value for small payments
	feeLimit := math.Max(
		decodedInvoice.MilliSat.ToSatoshis().MulF64(maxPaymentFeeRatio).ToUnit(btcutil.AmountSatoshi),
		minPaymentFee,
	)

	return int64(feeLimit), nil
}
