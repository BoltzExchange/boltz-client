package main

import (
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/google/logger"
	"strconv"
	"time"
)

const retryInterval = 30
var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func waitForLndSynced(lnd *lnd.LND) {
	info, err := lnd.GetInfo()

	if err == nil {
		if !info.SyncedToChain {
			logger.Info("LND node not synced yet")
			logger.Info(retryMessage)
			time.Sleep(retryInterval * time.Second)

			waitForLndSynced(lnd)
		}
	} else {
		logger.Info("Could not get LND info: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		waitForLndSynced(lnd)
	}

}
