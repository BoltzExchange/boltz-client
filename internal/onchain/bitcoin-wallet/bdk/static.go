//go:build static && !dynamic

package bdk

/*
#cgo LDFLAGS: ${SRCDIR}/libbdk.a -Wl,--no-as-needed -ldl
*/
import "C"
