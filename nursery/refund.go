package nursery

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
)

func (nursery *Nursery) startBlockListener(currency boltz.Currency) *utils.ChannelForwarder[*onchain.BlockEpoch] {
	blockNotifier := nursery.onchain.RegisterBlockListener(nursery.ctx, currency)

	nursery.waitGroup.Add(1)
	go func() {
		defer nursery.waitGroup.Done()
		for newBlock := range blockNotifier.Get() {
			logger.Debugf("Received new block, checking externally paid reverse swaps: %d", newBlock.Height)
			if err := nursery.checkExternalReverseSwaps(currency, ""); err != nil {
				logger.Error("Could not check external reverse swaps: " + err.Error())
			}
		}
	}()

	return blockNotifier
}
