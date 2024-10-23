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
	if ptr.error != nil {
		defer C.free_c_string(ptr.error)
		return nil, errors.New(C.GoString(ptr.error))
	}
	defer C.free(unsafe.Pointer(ptr.result))
	return &Offer{
		MinAmount: uint64(ptr.result.min_amount),
	}, nil
}

func DecodeBolt12(bolt12 string) (*DecodedInvoice, error) {
	offerPtr := C.CString(bolt12)
	defer C.free(unsafe.Pointer(offerPtr))
	ptr := C.decode_invoice(offerPtr)
	if ptr.error != nil {
		defer C.free_c_string(ptr.error)
		return nil, errors.New(C.GoString(ptr.error))
	}
	defer C.free(unsafe.Pointer(ptr.result))
	invoice := ptr.result
	return &DecodedInvoice{
		Amount:      uint64(invoice.amount),
		PaymentHash: *(*[32]byte)(unsafe.Pointer(&invoice.payment_hash)),
		Expiry:      time.Unix(int64(invoice.expiry_date), 0),
	}, nil
}

func CheckInvoiceIsForOffer(invoice string, offer string) bool {
	offerPtr := C.CString(offer)
	invoicePtr := C.CString(invoice)
	defer C.free(unsafe.Pointer(offerPtr))
	defer C.free(unsafe.Pointer(invoicePtr))
	return bool(C.check_invoice_is_for_offer(invoicePtr, offerPtr))
}
