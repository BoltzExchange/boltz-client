package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFormatMilliSat(t *testing.T) {
	assert.Equal(t, "3.567", FormatMilliSat(3567))
	assert.Equal(t, "10.000", FormatMilliSat(10000))
}
