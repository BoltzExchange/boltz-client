//go:build static && !dynamic

package liquid

/*
#cgo LDFLAGS: ${SRCDIR}/lib/libgreenaddress_full.a -Wl,--no-as-needed -ldl
*/
import "C"
