package boltz

// Block times in minutes
const BitcoinBlockTime = float64(10)
const LiquidBlockTime = float64(1)

func GetBlockTime(currency Currency) float64 {
	switch currency {
	case CurrencyBtc:
		return BitcoinBlockTime
	case CurrencyLiquid:
		return LiquidBlockTime
	default:
		return 0
	}
}

func BlocksToHours(blockDelta uint32, currency Currency) float64 {
	return float64(blockDelta) * (GetBlockTime(currency) / 60)
}

func CalculateInvoiceExpiry(blockDelta uint32, currency Currency) int64 {
	// Add one block to the delta to make sure that the invoice expiry is long enough
	blockDelta += 1

	return int64(blockDelta) * int64(GetBlockTime(currency)*60)
}
