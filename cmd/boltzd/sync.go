package main

import (
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"strconv"
	"time"
)

const retryInterval = 30

var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func connectToLnd(lnd *lnd.LND) *lnrpc.GetInfoResponse {
	lndInfo, err := lnd.GetInfo()

	if err != nil {
		logger.Warning("Could not connect to LND: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		_ = lnd.Connect()
		return connectToLnd(lnd)
	} else {
		return lndInfo
	}
}

func waitForLndSynced(lnd *lnd.LND) {
	info, err := lnd.GetInfo()

	if err == nil {
		if !info.SyncedToChain {
			logger.Warning("LND node not synced yet")
			logger.Info(retryMessage)
			time.Sleep(retryInterval * time.Second)

			waitForLndSynced(lnd)
		}
	} else {
		logger.Error("Could not get LND info: " + err.Error())
		logger.Info(retryMessage)
		time.Sleep(retryInterval * time.Second)

		waitForLndSynced(lnd)
	}
}
