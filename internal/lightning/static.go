//go:build static && !dynamic

package lightning

/*
#cgo LDFLAGS: ${SRCDIR}/lib/libbolt12.a -Wl,--no-as-needed -Wl,--allow-multiple-definition -ldl
*/
import "C"
