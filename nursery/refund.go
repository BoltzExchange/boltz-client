package nursery

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"strconv"
)

func (nursery *Nursery) startBlockListener(currency boltz.Currency) {
	blockNotifier := nursery.registerBlockListener(currency)

	go func() {
		for newBlock := range blockNotifier {
			if nursery.stopped {
				return
			}
			swaps, err := nursery.database.QueryRefundableSwapsForBlockHeight(newBlock.Height, currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			chainSwaps, err := nursery.database.QueryRefundableChainSwapsForBlockHeight(newBlock.Height, currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swaps) > 0 || len(chainSwaps) > 0 {
				logger.Info("Found " + strconv.Itoa(len(swaps)) + " Swaps to refund at height " + strconv.FormatUint(uint64(newBlock.Height), 10))

				if err := nursery.RefundSwaps(currency, swaps, chainSwaps, false); err != nil {
					logger.Error("Could not refund Swaps: " + err.Error())
				}
			}
		}
	}()
}

func (nursery *Nursery) RefundSwaps(currency boltz.Currency, swaps []database.Swap, chainSwaps []database.ChainSwap, cooperative bool) error {
	var outputs []*Output

	for i := range swaps {
		outputs = append(outputs, nursery.getRefundOutput(&swaps[i]))
	}
	for i := range chainSwaps {
		outputs = append(outputs, nursery.getChainSwapRefundOutput(&chainSwaps[i]))
	}

	for _, output := range outputs {
		output.Cooperative = cooperative
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
