package utils

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

func ExpandHomeDir(path string) string {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return path
	}

	if path == "~" {
		// In case of "~", which won't be caught by the "else if"
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		// Use strings.HasPrefix so we don't match paths like
		// "/something/~/something/"
		path = filepath.Join(homeDir, path[2:])
	}

	return path

}

func ExpandDefaultPath(dataDir string, currentValue string, defaultFileName string) string {
	if currentValue == "" {
		currentValue = path.Join(dataDir, defaultFileName)
	}

	return ExpandHomeDir(currentValue)
}

func GetDefaultDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return "", err
	}

	var dataFolder string
	if _, err := os.Stat(path.Join(homeDir, ".boltz-lnd")); err == nil {
		dataFolder = ".boltz-lnd"
	} else {
		dataFolder = ".boltz"
	}

	return path.Join(homeDir, dataFolder), nil
}
