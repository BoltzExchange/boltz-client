package rpcserver

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

const minLndVersion = "0.15.0"
const minClnVersion = "23.0.0"
const minBoltzVersion = "3.5.0"

func checkLndVersion(info *lightning.LightningInfo) {
	if err := utils.CheckVersion("LND", info.Version, minLndVersion); err != nil {
		logger.Fatal(err.Error())
	}
}

func checkClnVersion(info *lightning.LightningInfo) {
	if err := utils.CheckVersion("CLN", info.Version, minClnVersion); err != nil {
		logger.Fatal(err.Error())
	}
}

func checkBoltzVersion(boltz *boltz.Api) {
	version, err := boltz.GetVersion()
	if err != nil {
		logger.Fatal("Could not get Boltz version: " + err.Error())
	}
	if err := utils.CheckVersion("Boltz", version.Version, minBoltzVersion); err != nil {
		logger.Fatal(err.Error())
	}
}
