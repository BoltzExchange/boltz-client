package main

import (
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"strconv"
	"strings"
)

const minLndVersion = "0.11.0"
const minBoltzVersion = "2.3.0"

func checkLndVersion(lndInfo *lnrpc.GetInfoResponse) {
	versionInt, err := parseVersion(lndInfo.Version)

	if err != nil {
		logger.Fatal("Could node parse LND version: " + err.Error())
	}

	minVersionInt, _ := strconv.ParseInt(strings.Replace(minLndVersion, ".", "", 2), 10, 64)

	if versionInt < minVersionInt {
		errorMinVersion("LND", minLndVersion)
	}
}

func checkBoltzVersion(boltz *boltz.Boltz) {
	version, err := boltz.GetVersion()

	if err != nil {
		logger.Fatal("Could not get Boltz version: " + err.Error())
		return
	}

	versionInt, err := parseVersion(version.Version)

	if err != nil {
		logger.Fatal("Could not parse Boltz version: " + err.Error())
	}

	minVersionInt, _ := strconv.ParseInt(strings.Replace(minBoltzVersion, ".", "", 2), 10, 64)

	if versionInt < minVersionInt {
		errorMinVersion("Boltz", minBoltzVersion)
	}
}

func parseVersion(version string) (int64, error) {
	versionSplit := strings.Split(version, "-")[0]
	rawVersion := strings.Replace(versionSplit, ".", "", 2)
	return strconv.ParseInt(rawVersion, 10, 64)
}

func errorMinVersion(service string, minVersion string) {
	logger.Fatal("Incompatible " + service + " version detected. Minimal supported version is: " + minVersion)
}
