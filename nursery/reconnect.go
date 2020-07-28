package nursery

import (
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"strconv"
	"time"
)

const retryInterval = time.Second * 15

func (nursery *Nursery) streamSwapStatus(
	id string,
	swapType string,
	eventStream chan *boltz.SwapStatusResponse,
	stopListening chan bool,
	stopHandler chan bool,
) {
	go func() {
		err := nursery.boltz.StreamSwapStatus(id, eventStream, stopListening)

		if err == nil {
			logger.Info("Stopping event listener of " + swapType + " " + id)
		} else {
			logger.Error("Could not listen to events of " + swapType + " " + id + ": " + err.Error())
			logRetry(id, swapType)

			ticker := time.NewTicker(retryInterval)

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
						nursery.streamSwapStatus(id, swapType, eventStream, stopListening, stopHandler)

						return
					}

					logger.Info("Could not fetch status of " + swapType + " " + id + ": " + err.Error())
					logRetry(id, swapType)
					break

				case <-stopListening:
					logger.Info("Stopping reconnection loop of " + swapType + " " + id)

					ticker.Stop()
					return
				}
			}
		}

		eventListenersLock.Lock()
		delete(eventListeners, id)
		eventListenersLock.Unlock()

		stopHandler <- true
	}()
}

func logRetry(id string, swapType string) {
	logger.Info("Retrying to listen to events of " + swapType + " " + id + " in " +
		strconv.FormatFloat(retryInterval.Seconds(), 'f', 0, 64) + " seconds")
}
