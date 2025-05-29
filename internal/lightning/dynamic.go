//go:build dynamic || !static

package lightning

/*
#cgo LDFLAGS: -L${SRCDIR}/lib -lbolt12 -Wl,-rpath=${SRCDIR}/lib
*/
import "C"
