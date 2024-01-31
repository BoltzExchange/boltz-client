package nursery

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"strconv"
	"time"
)

func (nursery *Nursery) streamSwapStatus(
	id string,
	swapType string,
	eventStream chan *boltz.SwapStatusResponse,
	stopListening <-chan bool,
) {
	go func() {
		err := nursery.boltz.StreamSwapStatus(id, eventStream, stopListening)
		if err == nil {
			logger.Info("Stopping event listener of Swap" + " " + id)
			close(eventStream)
		} else {
			logger.Error("Could not listen to events of Swap " + id + ": " + err.Error())
			logRetry(id, swapType)

			ticker := time.NewTicker(retryInterval * time.Second)

			for {
				select {
				case <-ticker.C:
					latestStatus, err := nursery.boltz.SwapStatus(id)

					// If the request was successful, the latest status should be handled and the daemon should
					// start listening to the SSE stream again
					if err == nil {
						ticker.Stop()

						logger.Info("Reconnected to event stream of " + swapType + "" + id)

						eventStream <- latestStatus
						nursery.streamSwapStatus(id, swapType, eventStream, stopListening)

						return
					}

					logger.Info("Could not fetch status of " + swapType + " " + id + ": " + err.Error())
					logRetry(id, swapType)
				case <-stopListening:
					logger.Info("Stopping reconnection loop of " + swapType + " " + id)
					close(eventStream)

					ticker.Stop()
					return
				}
			}
		}
	}()
}

func logRetry(id string, swapType string) {
	logger.Info("Retrying to listen to events of " + swapType + " " + id + " in " +
		strconv.Itoa(retryInterval) + " seconds")
}
