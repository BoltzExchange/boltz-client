package nursery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxInt64(t *testing.T) {
	assert.Equal(t, maxInt64(0, 2), int64(2))
	assert.Equal(t, maxInt64(12, 2), int64(12))
}
