//go:build dynamic || !static

package wallet

/*
#cgo LDFLAGS: -L${SRCDIR}/lib -lgreen_gdk -Wl,-rpath=${SRCDIR}/lib
*/
import "C"
