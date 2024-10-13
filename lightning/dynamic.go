//go:build dynamic || !static

package lightning

/*
#cgo LDFLAGS: -L${SRCDIR}/lib/bolt12/target/release -lbolt12 -Wl,-rpath=${SRCDIR}/lib/bolt12/target/release
*/
import "C"
