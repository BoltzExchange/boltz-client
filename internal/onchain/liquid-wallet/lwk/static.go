//go:build static && !dynamic

package lwk

/*
#cgo LDFLAGS: ${SRCDIR}/liblwk.a -Wl,--no-as-needed -ldl
*/
import "C"
