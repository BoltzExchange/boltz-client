package build

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetVersion(t *testing.T) {
	assert.Equal(t, "v"+version, GetVersion())

	Commit = "123"
	assert.Equal(t, "v"+version+"-"+Commit, GetVersion())
}
