package build

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetVersion(t *testing.T) {
	Version = "1.0.0"
	assert.Equal(t, "v"+Version, GetVersion())

	Commit = "123"
	assert.Equal(t, "v"+Version+"-"+Commit, GetVersion())
}
