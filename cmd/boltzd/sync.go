package main

import (
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

const retryInterval = 15

var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func connectLightning(lightning lightning.LightningNode) *lightning.LightningInfo {
	err := lightning.Connect()

	if err != nil {
		logger.Fatal("Could not connect to lightning node: " + err.Error())
	}

	info, err := lightning.GetInfo()

	if err != nil {
		logger.Warn("Could not connect to lightning node: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		return connectLightning(lightning)
	} else {
		return info
	}
}

func waitForLightningSynced(lightning lightning.LightningNode) {
	info, err := lightning.GetInfo()

	if err == nil {
		if !info.Synced {
			logger.Warn("Lightning node not synced yet")
			logger.Info(retryMessage)
			time.Sleep(retryInterval * time.Second)

			waitForLightningSynced(lightning)
		}
	} else {
		logger.Error("Could not get lightning info: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		waitForLightningSynced(lightning)
	}
}
