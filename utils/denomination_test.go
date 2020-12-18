package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetSmallestUnitName(t *testing.T) {
	assert.Equal(t, "satoshi", GetSmallestUnitName("BTC"))
	assert.Equal(t, "litoshi", GetSmallestUnitName("LTC"))

	// Default value
	assert.Equal(t, "satoshi", GetSmallestUnitName("NOTFOUND"))
}
