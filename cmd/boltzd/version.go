package main

import (
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/lightning"
	"github.com/BoltzExchange/boltz-lnd/logger"
)

const minLndVersion = "0.15.0"
const minBoltzVersion = "2.3.0"

func checkVersion(name string, version string, minVersion string) {
	versionInt, err := parseVersion(version)

	if err != nil {
		logger.Fatal("Could not parse " + name + " version: " + err.Error())
	}

	minVersionInt, _ := strconv.ParseInt(strings.Replace(minVersion, ".", "", 2), 10, 64)

	if versionInt < minVersionInt {
		logger.Fatal("Incompatible " + name + " version detected. Minimal supported version is: " + minVersion)
	}
}

func checkLightningVersion(info *lightning.LightningInfo) {
	//checkVersion("LND", info.Version, minLndVersion)
}

func checkBoltzVersion(boltz *boltz.Boltz) {
	version, err := boltz.GetVersion()
	if err != nil {
		logger.Fatal("Could not get Boltz version: " + err.Error())
		return
	}
	checkVersion("Boltz", version.Version, minBoltzVersion)
}

func parseVersion(version string) (int64, error) {
	versionSplit := strings.Split(version, "-")[0]
	rawVersion := strings.Replace(versionSplit, ".", "", 2)
	return strconv.ParseInt(rawVersion, 10, 64)
}
