package utils

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"runtime"
	"testing"
)

func TestExpandDefaultPath(t *testing.T) {
	dataDir := "/test/"
	currentValue := ""
	defaultFileName := "test.txt"

	assert.Equal(t, path.Join(dataDir, defaultFileName), ExpandDefaultPath(dataDir, currentValue, defaultFileName))

	// Should leave the current value untouched if it is not empty
	currentValue = "/some/path.txt"

	assert.Equal(t, currentValue, ExpandDefaultPath(dataDir, currentValue, defaultFileName))
}

func TestGetDefaultDataDir(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	dataFolder := "boltz-lnd"

	if runtime.GOOS != "windows" {
		dataFolder = "." + dataFolder
	}

	dataDir, err := GetDefaultDataDir()

	assert.Nil(t, err)
	assert.Equal(t, path.Join(homeDir, dataFolder), dataDir)
}
