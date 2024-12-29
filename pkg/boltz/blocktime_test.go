package boltz

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetBlockTime(t *testing.T) {
	assert.Equal(t, float64(10), GetBlockTime(CurrencyBtc))
	assert.Equal(t, float64(1), GetBlockTime(CurrencyLiquid))

	// Should return 0 when the symbol cannot be found
	assert.Equal(t, float64(0), GetBlockTime(""))
}

func TestBlocksToHours(t *testing.T) {
	assert.Equal(t, float64(10), BitcoinBlockTime)

	assert.Equal(t, float64(2), BlocksToHours(12, CurrencyBtc))
	assert.Equal(t, 0.1, BlocksToHours(6, CurrencyLiquid))

}

func TestCalculateInvoiceExpiry(t *testing.T) {
	assert.Equal(t, int64(7800), CalculateInvoiceExpiry(12, CurrencyBtc))
	assert.Equal(t, int64(19320), CalculateInvoiceExpiry(321, CurrencyLiquid))
}
