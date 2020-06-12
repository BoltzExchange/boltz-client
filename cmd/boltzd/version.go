package main

import (
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"strconv"
	"strings"
)

const minLndVersion = "0.10.0"

func checkLndVersion(lndInfo *lnrpc.GetInfoResponse) {
	version := strings.Split(lndInfo.Version, "-")[0]
	rawVersion := strings.Replace(version, ".", "", 2)
	rawVersionInt, err := strconv.ParseInt(rawVersion, 10, 64)

	if err != nil {
		logger.Fatal("Could node parse LND version: " + err.Error())
	}

	minVersionInt, _ := strconv.ParseInt(strings.Replace(minLndVersion, ".", "", 2), 10, 64)

	if rawVersionInt < minVersionInt {
		logger.Fatal("Incompatible LND version detected. Minimal supported version is: " + minLndVersion)
	}
}
