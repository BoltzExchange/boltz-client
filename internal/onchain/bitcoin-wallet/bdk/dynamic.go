//go:build dynamic || !static

package bdk

/*
#cgo LDFLAGS: -L${SRCDIR} -lbdk -Wl,-rpath=${SRCDIR}

*/
import "C"
