package lightning

import "C"
import (
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
	"time"
)

type DecodedInvoice struct {
	AmountSat        uint64
	PaymentHash      [32]byte
	Expiry           time.Time
	MagicRoutingHint *btcec.PublicKey
}

func DecodeInvoice(invoice string, network *chaincfg.Params) (*DecodedInvoice, error) {
	bolt11, err := zpay32.Decode(invoice, network)
	if err == nil {
		var amount uint64
		if bolt11.MilliSat != nil {
			amount = uint64(bolt11.MilliSat.ToSatoshis())
		}
		return &DecodedInvoice{
			AmountSat:        amount,
			PaymentHash:      *bolt11.PaymentHash,
			Expiry:           bolt11.Timestamp.Add(bolt11.Expiry()),
			MagicRoutingHint: boltz.FindMagicRoutingHint(bolt11),
		}, nil
	}
	return DecodeBolt12Invoice(invoice)
}
