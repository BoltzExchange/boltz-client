package main

import (
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-lnd/lightning"
	"github.com/BoltzExchange/boltz-lnd/logger"
)

const retryInterval = 15

var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func connectLightning(lightning lightning.LightningNode) *lightning.LightningInfo {
	info, err := lightning.GetInfo()

	if err != nil {
		logger.Warning("Could not connect to lightning node: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		_ = lightning.Connect()
		return connectLightning(lightning)
	} else {
		return info
	}
}

func waitForLightningSynced(lightning lightning.LightningNode) {
	info, err := lightning.GetInfo()

	if err == nil {
		if !info.Synced {
			logger.Warning("LND node not synced yet")
			logger.Info(retryMessage)
			time.Sleep(retryInterval * time.Second)

			waitForLightningSynced(lightning)
		}
	} else {
		logger.Error("Could not get LND info: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		waitForLightningSynced(lightning)
	}
}
