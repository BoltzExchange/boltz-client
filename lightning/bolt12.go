package lightning

/*
#include "./lib/bolt12/target/bolt12.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"strings"
	"unsafe"
)

type Invoice = C.Invoice

func IsOffer(offer string) bool {
	return strings.Contains(offer, "lno")
}

func DecodeOffer(offer string) (bool, error) {
	offerPtr := C.CString(offer)
	defer C.free(unsafe.Pointer(offerPtr))
	ptr := C.decode_offer(offerPtr)
	if ptr == nil {
		return false, errors.New("invalid offer")
	}
	return true, nil
}

func DecodeBolt12(bolt12 string) (*Invoice, error) {
	offerPtr := C.CString(bolt12)
	defer C.free(unsafe.Pointer(offerPtr))
	ptr := C.decode_invoice(offerPtr)
	if ptr == nil {
		return ptr, errors.New("invalid offer")
	}
	return ptr, nil
}
