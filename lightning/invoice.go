package lightning

import "C"
import (
	"errors"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
	"time"
	"unsafe"
)

type DecodedInvoice struct {
	Amount      uint64
	PaymentHash [32]byte
	Expiry      time.Time
	Hint        *btcec.PublicKey
}

func DecodeInvoice(invoice string, network *chaincfg.Params) (*DecodedInvoice, error) {
	bolt11, err := zpay32.Decode(invoice, network)
	if err == nil {
		var amount uint64
		if bolt11.MilliSat != nil {
			amount = uint64(bolt11.MilliSat.ToSatoshis())
		}
		return &DecodedInvoice{
			Amount:      amount,
			PaymentHash: *bolt11.PaymentHash,
			Expiry:      bolt11.Timestamp.Add(bolt11.Expiry()),
			Hint:        boltz.FindMagicRoutingHint(bolt11),
		}, nil
	}
	bolt12, err := DecodeBolt12(invoice)
	if err == nil {
		return &DecodedInvoice{
			Amount:      uint64(bolt12.amount),
			PaymentHash: *(*[32]byte)(unsafe.Pointer(&bolt12.payment_hash)),
			Expiry:      time.Unix(int64(bolt12.expiry_date), 0),
		}, nil
	}

	return nil, errors.New("invalid invoice")
}
