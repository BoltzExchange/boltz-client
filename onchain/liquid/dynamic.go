//go:build dynamic || !static

package liquid

/*
#cgo LDFLAGS: -L${SRCDIR}/lib -lgreenaddress -Wl,-rpath=${SRCDIR}/lib
*/
import "C"
