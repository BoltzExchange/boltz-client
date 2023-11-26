package utils

import (
	"strconv"

	"github.com/BoltzExchange/boltz-client/boltz"
)

// Block times in minutes
const BitcoinBlockTime = float64(10)
const LiquidBlockTime = float64(1)

func GetBlockTime(pairId boltz.Pair) float64 {
	var blockTime float64

	switch pairId {
	case boltz.PairBtc:
		blockTime = BitcoinBlockTime
	case boltz.PairLiquid:
		blockTime = LiquidBlockTime
	}

	return blockTime
}

func BlocksToHours(blockDelta uint32, blockTime float64) string {
	return strconv.FormatFloat(float64(blockDelta)*(blockTime/60), 'f', 1, 64)
}

func CalculateInvoiceExpiry(blockDelta uint32, blockTime float64) int64 {
	// Add one block to the delta to make sure that the invoice expiry is long enough
	blockDelta += 1

	return int64(blockDelta) * int64(blockTime*60)
}
