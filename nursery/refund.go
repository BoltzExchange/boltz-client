package nursery

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
)

func (nursery *Nursery) startBlockListener(currency boltz.Currency) {
	blockNotifier := nursery.registerBlockListener(currency)

	go func() {
		for newBlock := range blockNotifier {
			if nursery.stopped {
				return
			}
			swaps, err := nursery.database.QueryRefundableSwaps(currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			chainSwaps, err := nursery.database.QueryRefundableChainSwaps(currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swaps) > 0 || len(chainSwaps) > 0 {
				logger.Infof("Found %d Swaps to refund at height %d", len(swaps), newBlock.Height)

				if err := nursery.RefundSwaps(currency, swaps, chainSwaps); err != nil {
					logger.Error("Could not refund Swaps: " + err.Error())
				}
			}
		}
	}()
}

func (nursery *Nursery) RefundSwaps(currency boltz.Currency, swaps []database.Swap, chainSwaps []database.ChainSwap) error {
	var outputs []*Output

	for i := range swaps {
		outputs = append(outputs, nursery.getRefundOutput(&swaps[i]))
	}
	for i := range chainSwaps {
		outputs = append(outputs, nursery.getChainSwapRefundOutput(&chainSwaps[i]))
	}

	height, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		return fmt.Errorf("could not get block height: %w", err)
	}
	for _, output := range outputs {
		output.Cooperative = output.TimeoutBlockHeight > height
	}

	if len(outputs) == 0 {
		logger.Info("Did not find any outputs to refund")
		return nil
	}
	refundTransactionId, err := nursery.refundOutputs(currency, outputs)
	if err != nil {
		return err
	}

	logger.Infof("Constructed refund transaction for %d swaps: %s", len(outputs), refundTransactionId)
	return nil
}
