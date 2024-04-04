package nursery

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
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

			chainSwaps, err := nursery.database.QueryRefundableChainSwaps(newBlock.Height, currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swaps) > 0 || len(chainSwaps) > 0 {
				logger.Info("Found " + strconv.Itoa(len(swaps)) + " Swaps to refund at height " + strconv.FormatUint(uint64(newBlock.Height), 10))

				if err := nursery.RefundSwaps(swaps, chainSwaps, false); err != nil {
					logger.Error("Could not refund Swaps: " + err.Error())
				}
			}
		}
	}()
}

func (nursery *Nursery) RefundSwaps(swaps []database.Swap, chainSwaps []database.ChainSwap, cooperative bool) error {
	var currency boltz.Currency
	var refundedSwaps []database.Swap
	var refundedChainSwaps []database.ChainSwap
	var outputs []*Output

	for _, swap := range swaps {
		currency = swap.Pair.From
		refundOutput, err := nursery.getRefundOutput(&swap)
		if err != nil {
			return fmt.Errorf("could not get refund output of swap %s: %v", swap.Id, err)
		}

		refundOutput.Cooperative = cooperative
		refundedSwaps = append(refundedSwaps, swap)
		outputs = append(outputs, refundOutput)
	}
	for _, chainSwap := range chainSwaps {
		currency = chainSwap.Pair.From
		refundOutput, err := nursery.getChainSwapRefundOutput(&chainSwap)
		if err != nil {
			return fmt.Errorf("could not get refund output for chain swap %s: %v", chainSwap.Id, err)
		}

		refundOutput.Cooperative = cooperative
		refundedChainSwaps = append(refundedChainSwaps, chainSwap)
		outputs = append(outputs, refundOutput)
	}

	if len(outputs) == 0 {
		logger.Info("Did not find any outputs to refund")
		return nil
	}
	refundTransactionId, totalRefundFee, err := nursery.refundOutputs(currency, outputs)
	if err != nil {
		return err
	}

	logger.Infof("Constructed refund transaction for %d swaps: %s", len(outputs), refundTransactionId)

	count := uint64(len(refundedSwaps) + len(refundedChainSwaps))
	refundFee := totalRefundFee / count
	first := true
	// distribute the remainder of the fee to the first swap
	getFee := func() uint64 {
		fee := refundFee
		if first {
			fee += totalRefundFee % count
			first = false
		}
		return fee
	}
	for _, refundedSwap := range refundedSwaps {
		err = nursery.database.SetSwapRefundTransactionId(&refundedSwap, refundTransactionId, getFee())

		if err != nil {
			logger.Error("Could not set refund transaction id in database: " + err.Error())
			continue
		}

		nursery.sendSwapUpdate(refundedSwap)

		logger.Infof("Refunded Swap %s with refund transaction %s", refundedSwap.Id, refundTransactionId)
	}
	for _, refundedChainSwap := range refundedChainSwaps {
		err = nursery.database.SetChainSwapTransactionId(refundedChainSwap.FromData, refundTransactionId)
		if err != nil {
			logger.Error("Could not set refund transaction id in database: " + err.Error())
			continue
		}

		if err := nursery.database.UpdateChainSwapState(&refundedChainSwap, boltzrpc.SwapState_REFUNDED, ""); err != nil {
			logger.Error("Could not update chain swap state: " + err.Error())
			continue
		}

		err = nursery.database.AddChainSwapOnchainFee(&refundedChainSwap, getFee())
		if err != nil {
			logger.Error("Could not set refund transaction id in database: " + err.Error())
			continue
		}

		nursery.sendChainSwapUpdate(refundedChainSwap)

		logger.Infof("Refunded Chain Swap %s with refund transaction %s", refundedChainSwap.Id, refundTransactionId)
	}
	return nil
}
