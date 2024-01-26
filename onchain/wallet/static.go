//go:build static && !dynamic

package wallet

/*
#cgo LDFLAGS: ${SRCDIR}/lib/libgreenaddress_full.a -Wl,--no-as-needed -ldl
*/
import "C"
