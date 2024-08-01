//go:build static && !dynamic

package wallet

/*
#cgo LDFLAGS: ${SRCDIR}/lib/libgreen_gdk.a -Wl,--no-as-needed -ldl
*/
import "C"
