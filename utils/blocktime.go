package utils

import (
	"strconv"
)

// Block times in hours
const BitcoinBlockTime = float64(10) / float64(60)
const LitecoinBlockTime = float64(2.5) / float64(60)

func BlocksToHours(blockDelta uint32, blockTime float64) string {
	return strconv.FormatFloat(float64(blockDelta)*blockTime, 'f', 1, 64)
}
