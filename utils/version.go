package utils

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
)

func CheckVersion(name string, version string, minVersion string) error {
	// strip lnd commit
	version = strings.Split(version, " ")[0]
	// strip cln rc
	version = strings.Split(version, "rc")[0]
	version = strings.Split(version, "-")[0]

	parsed, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("Could not parse %s version %s: %s", name, version, err.Error())
	}

	minParsed, err := semver.NewVersion(minVersion)
	if err != nil {
		return fmt.Errorf("Could not parse %s min version %s: %s", name, minVersion, err.Error())
	}

	if parsed.LessThan(minParsed) {
		return fmt.Errorf("Incompatible %s version %s detected. Minimal supported version is: %s", name, version, minVersion)
	}
	return nil
}
