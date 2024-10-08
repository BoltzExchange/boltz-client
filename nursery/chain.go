package nursery

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
)

func (nursery *Nursery) sendChainSwapUpdate(swap database.ChainSwap) {
	isFinal := swap.State == boltzrpc.SwapState_SUCCESSFUL || swap.State == boltzrpc.SwapState_REFUNDED
	if swap.FromData.LockupTransactionId == "" && swap.State != boltzrpc.SwapState_PENDING {
		isFinal = false
	}

	nursery.sendUpdate(swap.Id, SwapUpdate{
		ChainSwap: &swap,
		IsFinal:   isFinal,
	})
}

func (nursery *Nursery) RegisterChainSwap(chainSwap database.ChainSwap) error {
	if err := nursery.registerSwaps([]string{chainSwap.Id}); err != nil {
		return err
	}
	nursery.sendChainSwapUpdate(chainSwap)
	return nil
}

func (nursery *Nursery) setChainSwapLockupTransaction(swap *database.ChainSwap, data *database.ChainSwapData, transactionId string) error {
	data.LockupTransactionId = transactionId
	_, _, _, err := nursery.findVout(chainVoutInfo(data))
	if err != nil {
		return fmt.Errorf("could not find lockup vout: %s", err)
	}

	if err := nursery.database.SetChainSwapLockupTransactionId(data, transactionId); err != nil {
		return errors.New("Could not set lockup transaction in database: " + err.Error())
	}

	if data == swap.ToData || data.WalletId != nil {
		fee, err := nursery.onchain.GetTransactionFee(data.Currency, transactionId)
		if err != nil {
			return errors.New("could not get lockup transaction fee: " + err.Error())
		}
		if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
			return errors.New("could not set lockup transaction fee in database: " + err.Error())
		}
	}

	return nil
}

func chainVoutInfo(data *database.ChainSwapData) voutInfo {
	info := voutInfo{
		transactionId: data.LockupTransactionId,
		currency:      data.Currency,
		address:       data.LockupAddress,
		blindingKey:   data.BlindingKey,
	}
	return info
}

func (nursery *Nursery) getChainSwapClaimOutput(swap *database.ChainSwap) *Output {
	info := chainVoutInfo(swap.ToData)
	info.requireConfirmed = !swap.AcceptZeroConf
	info.expectedAmount = swap.ToData.Amount
	return &Output{
		OutputDetails: &boltz.OutputDetails{
			SwapId:         swap.Id,
			SwapType:       boltz.ChainSwap,
			Preimage:       swap.Preimage,
			PrivateKey:     swap.ToData.PrivateKey,
			SwapTree:       swap.ToData.Tree,
			Cooperative:    true,
			RefundSwapTree: swap.FromData.Tree,
			Address:        swap.ToData.Address,
		},
		walletId: swap.ToData.WalletId,
		voutInfo: info,
		setTransaction: func(transactionId string, fee uint64) error {
			if err := nursery.database.SetChainSwapTransactionId(swap.ToData, transactionId); err != nil {
				return fmt.Errorf("Could not set lockup transaction in database: %w", err)
			}

			if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
				return fmt.Errorf("Could not set lockup transaction in database: %w", err)
			}

			return nil
		},
		setError: func(err error) {
			nursery.handleChainSwapError(swap, err)
		},
	}
}

func (nursery *Nursery) getChainSwapRefundOutput(swap *database.ChainSwap) *Output {
	return &Output{
		&boltz.OutputDetails{
			SwapId:             swap.Id,
			SwapType:           boltz.ChainSwap,
			PrivateKey:         swap.FromData.PrivateKey,
			SwapTree:           swap.FromData.Tree,
			TimeoutBlockHeight: swap.FromData.TimeoutBlockHeight,
			Cooperative:        true,
			Address:            swap.FromData.Address,
		},
		swap.FromData.WalletId,
		chainVoutInfo(swap.FromData),
		func(transactionId string, fee uint64) error {
			if err := nursery.database.SetChainSwapTransactionId(swap.FromData, transactionId); err != nil {
				return fmt.Errorf("could not set refund transaction id in database: %s", err)
			}

			if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_REFUNDED, ""); err != nil {
				return err
			}

			if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
				return fmt.Errorf("could not add onchain fee in db: %s", err)
			}

			nursery.sendChainSwapUpdate(*swap)

			return nil
		},
		func(err error) {
			nursery.handleChainSwapError(swap, err)
		},
	}
}

func (nursery *Nursery) handleChainSwapError(swap *database.ChainSwap, err error) {
	if dbErr := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
		logger.Error(dbErr.Error())
	}
	logger.Errorf("Chain Swap %s error: %s", swap.Id, err)
	nursery.sendChainSwapUpdate(*swap)
}

func (nursery *Nursery) handleChainSwapStatus(swap *database.ChainSwap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Debugf("Status of Chain Swap %s is %s already", swap.Id, parsedStatus)
		return
	}

	logger.Infof("Status of Chain Swap %s changed to: %s", swap.Id, parsedStatus)

	handleError := func(err string) {
		nursery.handleChainSwapError(swap, errors.New(err))
	}

	if swap.FromData.LockupTransactionId == "" || swap.ToData.LockupTransactionId == "" {
		response, err := nursery.boltz.GetChainSwapTransactions(swap.Id)
		if err != nil {
			var boltzErr boltz.Error
			if !errors.As(err, &boltzErr) {
				handleError("Could not get lockup tx from boltz: " + err.Error())
				return
			}
		} else {
			if swap.FromData.LockupTransactionId == "" && response.UserLock != nil {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.FromData, response.UserLock.Transaction.Id); err != nil {
					handleError("Could not set lockup transaction in database: " + err.Error())
					return
				}
				logger.Infof("Found user lockup for Chain Swap %s", swap.Id)
			}
			if swap.ToData.LockupTransactionId == "" && response.ServerLock != nil {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.ToData, response.ServerLock.Transaction.Id); err != nil {
					handleError("Could not set lockup transaction in database: " + err.Error())
					return
				}
				logger.Infof("Found server lockup for Chain Swap %s", swap.Id)
			}
		}
	}

	switch parsedStatus {
	case boltz.TransactionServerConfirmed, boltz.TransactionServerMempoool:
		if (parsedStatus == boltz.TransactionServerMempoool && !swap.AcceptZeroConf) || swap.ToData.Transactionid != "" {
			break
		}

		output := nursery.getChainSwapClaimOutput(swap)
		if _, err := nursery.createTransaction(swap.Pair.To, []*Output{output}); err != nil {
			logger.Infof("Could not claim chain swap output: %s", err)
			return
		}
	}

	logger.Debugf("Updating status of Chain Swap %s to %s", swap.Id, parsedStatus)

	err := nursery.database.UpdateChainSwapStatus(swap, parsedStatus)

	if err != nil {
		handleError(fmt.Sprintf("Could not update status of Chain Swap %s to %s: %s", swap.Id, parsedStatus, err))
		return
	}

	if parsedStatus.IsCompletedStatus() {
		serviceFee := swap.ServiceFeePercent.Calculate(swap.FromData.Amount)

		logger.Infof("Chain Swap service fee: %dsat onchain fee: %dsat", serviceFee, *swap.OnchainFee)

		if err := nursery.database.SetChainSwapServiceFee(swap, serviceFee); err != nil {
			handleError("Could not set swap service fee in database: " + err.Error())
			return
		}

		if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			handleError(err.Error())
			return
		}
	} else if parsedStatus.IsFailedStatus() {
		logger.Infof("Chain Swap %s failed", swap.Id)

		if swap.State == boltzrpc.SwapState_PENDING {
			if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				handleError(err.Error())
				return
			}
		}

		if swap.FromData.LockupTransactionId != "" {
			if _, err := nursery.RefundSwaps(swap.Pair.From, nil, []*database.ChainSwap{swap}); err != nil {
				handleError("Could not refund Swap " + swap.Id + ": " + err.Error())
				return
			}
		}

		return
	}
	nursery.sendChainSwapUpdate(*swap)

}
