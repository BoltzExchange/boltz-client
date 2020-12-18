package utils

import (
	"os"
	"path"
	"runtime"
)

func ExpandDefaultPath(dataDir string, currentValue string, defaultFileName string) string {
	if currentValue == "" {
		return path.Join(dataDir, defaultFileName)
	}

	return currentValue
}

func GetDefaultDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return "", err
	}

	dataFolder := "boltz-lnd"

	if runtime.GOOS != "windows" {
		dataFolder = "." + dataFolder
	}

	return path.Join(homeDir, dataFolder), nil
}
