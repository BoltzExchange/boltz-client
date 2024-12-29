package utils

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
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

	dataFolder := ".boltz"

	dataDir, err := GetDefaultDataDir()

	assert.Nil(t, err)
	assert.Equal(t, path.Join(homeDir, dataFolder), dataDir)
}
