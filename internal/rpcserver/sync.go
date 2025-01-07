package rpcserver

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"strconv"
	"strings"
	"time"
)

const retryInterval = 15

var retryMessage = "Retrying in " + strconv.Itoa(retryInterval) + " seconds"

func connectLightning(stop chan bool, lightning lightning.LightningNode) (*lightning.LightningInfo, error) {
	err := lightning.Connect()
	if err != nil {
		return nil, err
	}

	for {
		info, err := lightning.GetInfo()
		if err != nil {
			if strings.Contains(err.Error(), "unlock") {
				logger.Warn("Lightning node is locked")
			} else {
				return nil, err
			}
		}
		if info.Synced {
			return info, nil
		} else {
			logger.Warn("Lightning node not synced yet")
		}
		logger.Info(retryMessage)
		wait := time.After(retryInterval * time.Second)
		if stop != nil {
			select {
			case <-stop:
				return nil, nil
			case <-wait:
			}
		} else {
			<-wait
		}
	}
}
