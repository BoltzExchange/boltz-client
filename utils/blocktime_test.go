package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBlocksToHours(t *testing.T) {
	assert.Equal(t, float64(10)/float64(60), BitcoinBlockTime)
	assert.Equal(t, 2.5/float64(60), LitecoinBlockTime)

	assert.Equal(t, "23.3", BlocksToHours(140, BitcoinBlockTime))
	assert.Equal(t, "1.0", BlocksToHours(6, BitcoinBlockTime))

	assert.Equal(t, "2.5", BlocksToHours(60, LitecoinBlockTime))
}
