package nursery

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
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
			logger.Debugf("Received new block, checking refundable and externally paid reverse swaps: %d", newBlock.Height)
			swaps, chainSwaps, err := nursery.database.QueryAllRefundableSwaps(nil, currency, newBlock.Height)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swaps) > 0 || len(chainSwaps) > 0 {
				logger.Infof("Found %d Swaps to refund at height %d", len(swaps)+len(chainSwaps), newBlock.Height)

				if _, err := nursery.RefundSwaps(currency, swaps, chainSwaps); err != nil {
					logger.Error("Could not refund Swaps: " + err.Error())
				}
			}

			if err := nursery.checkExternalReverseSwaps(currency, ""); err != nil {
				logger.Error("Could not check external reverse swaps: " + err.Error())
			}
		}
	}()

	return blockNotifier
}

func (nursery *Nursery) RefundSwaps(currency boltz.Currency, swaps []*database.Swap, chainSwaps []*database.ChainSwap) (string, error) {
	var outputs []*Output

	for _, swap := range swaps {
		outputs = append(outputs, nursery.getRefundOutput(swap))
	}
	for _, swap := range chainSwaps {
		outputs = append(outputs, nursery.getChainSwapRefundOutput(swap))
	}

	height, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		return "", fmt.Errorf("could not get block height: %w", err)
	}
	for _, output := range outputs {
		output.Cooperative = output.TimeoutBlockHeight > height
		logger.Debugf(
			"Output for swap %s cooperative: %t (%d > %d)",
			output.SwapId, output.Cooperative, output.TimeoutBlockHeight, height,
		)
	}

	if len(outputs) == 0 {
		logger.Info("Did not find any outputs to refund")
		return "", nil
	}
	return nursery.createTransaction(currency, outputs)
}
