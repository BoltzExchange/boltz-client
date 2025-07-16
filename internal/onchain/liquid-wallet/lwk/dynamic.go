//go:build dynamic || !static

package lwk

/*
#cgo LDFLAGS: -L${SRCDIR} -llwk -Wl,-rpath=${SRCDIR}

*/
import "C"
