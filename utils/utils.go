package utils

import "strconv"

// TODO: test this on real network
func FormatMilliSat(milliSat int64) string {
	return strconv.FormatFloat(float64(milliSat)/1000, 'f', 3, 64)
}
