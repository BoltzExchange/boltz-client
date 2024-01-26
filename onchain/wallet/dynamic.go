//go:build dynamic || !static

package wallet

/*
#cgo LDFLAGS: -L${SRCDIR}/lib -lgreenaddress -Wl,-rpath=${SRCDIR}/lib
*/
import "C"
