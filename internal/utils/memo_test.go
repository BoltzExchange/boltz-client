package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetSwapMemo(t *testing.T) {
	assert.Equal(t, "Submarine Swap from BTC", GetSwapMemo("BTC"))
	assert.Equal(t, "Submarine Swap from LTC", GetSwapMemo("LTC"))
}
