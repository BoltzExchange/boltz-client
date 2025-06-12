package lightning

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

const (
	minPaymentFee = btcutil.Amount(5)
)

// getFeeLimit calculates the fee limit of a payment in sat
func CalculateFeeLimit(invoice string, chainParams *chaincfg.Params, feeLimitPpm uint64) (uint, error) {
	decodedInvoice, err := zpay32.Decode(invoice, chainParams)

	if err != nil {
		return 0, err
	}

	// Use the minimum value for small payments
	feeLimit := max(
		decodedInvoice.MilliSat.ToSatoshis().MulF64(float64(feeLimitPpm)/1_000_000),
		minPaymentFee,
	)

	// feeLimit is in sats already, no need for ToUnit
	return uint(feeLimit), nil
}
