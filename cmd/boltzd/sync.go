package main

import (
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

const retryInterval = 15

var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func connectLightning(lightning lightning.LightningNode) (*lightning.LightningInfo, error) {
	err := lightning.Connect()
	if err != nil {
		return nil, err
	}

	return waitForLightningSynced(lightning), nil
}

func waitForLightningSynced(lightning lightning.LightningNode) *lightning.LightningInfo {
	info, err := lightning.GetInfo()

	if err == nil {
		if !info.Synced {
			logger.Warn("Lightning node not synced yet")
			logger.Info(retryMessage)
			time.Sleep(retryInterval * time.Second)

			return waitForLightningSynced(lightning)
		}
		return info
	} else {
		logger.Error("Could not get lightning info: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		return waitForLightningSynced(lightning)
	}
}
