package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetBlockTime(t *testing.T) {
	assert.Equal(t, float64(10), GetBlockTime("BTC"))
	assert.Equal(t, 2.5, GetBlockTime("LTC"))

	// Should return 0 when the symbol cannot be found
	assert.Equal(t, float64(0), GetBlockTime(""))
	assert.Equal(t, float64(0), GetBlockTime("NotFound"))
}

func TestBlocksToHours(t *testing.T) {
	assert.Equal(t, float64(10), BitcoinBlockTime)

	assert.Equal(t, "23.3", BlocksToHours(140, BitcoinBlockTime))
	assert.Equal(t, "1.0", BlocksToHours(6, BitcoinBlockTime))

}

func TestCalculateInvoiceExpiry(t *testing.T) {
	assert.Equal(t, int64(7800), CalculateInvoiceExpiry(12, BitcoinBlockTime))
	assert.Equal(t, int64(193200), CalculateInvoiceExpiry(321, BitcoinBlockTime))
}
