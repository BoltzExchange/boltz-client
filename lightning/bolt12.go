package lightning

/*
#include "./lib/bolt12/target/bolt12.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"time"
	"unsafe"
)

type Invoice = C.Invoice

type Offer struct {
	MinAmount uint64
}

func DecodeOffer(offer string) (*Offer, error) {
	offerPtr := C.CString(offer)
	defer C.free(unsafe.Pointer(offerPtr))
	ptr := C.decode_offer(offerPtr)
	if ptr == nil {
		return nil, errors.New("invalid offer")
	}
	defer C.free(unsafe.Pointer(ptr))
	return &Offer{
		MinAmount: uint64(ptr.min_amount),
	}, nil
}

func DecodeBolt12(bolt12 string) (*DecodedInvoice, error) {
	offerPtr := C.CString(bolt12)
	defer C.free(unsafe.Pointer(offerPtr))
	ptr := C.decode_invoice(offerPtr)
	if ptr == nil {
		return nil, errors.New("invalid invoice")
	}
	defer C.free(unsafe.Pointer(ptr))
	return &DecodedInvoice{
		Amount:      uint64(ptr.amount),
		PaymentHash: *(*[32]byte)(unsafe.Pointer(&ptr.payment_hash)),
		Expiry:      time.Unix(int64(ptr.expiry_date), 0),
	}, nil
}

func CheckBolt12Offer(bolt12 string, offer string) bool {
	offerPtr := C.CString(offer)
	invoicePtr := C.CString(bolt12)
	defer C.free(unsafe.Pointer(offerPtr))
	defer C.free(unsafe.Pointer(invoicePtr))
	return bool(C.check_invoice_offer(invoicePtr, offerPtr))
}
