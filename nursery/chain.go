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
func (nursery *Nursery) RegisterChainSwap(swap database.ChainSwap) error {
	if err := nursery.registerSwap(swap.Id); err != nil {
		return err
	}
	nursery.sendChainSwapUpdate(swap)
	return nil
}

/*
func (nursery *Nursery) getChainSwapOutputDetails(swap *database.ChainSwap) ([]boltz.OutputDetails, error) {
	outputDetails := make([]boltz.OutputDetails, 0, 2)

	if swap.FromData.LockupTransactionId != "" {
		outputDetails = append(outputDetails, boltz.OutputDetails{
			SwapId:       swap.Id,
			SwapType:     boltz.ChainSwap,
			TransactionId: swap.FromData.LockupTransactionId,
			OutputIndex:  0,
			Amount:       swap.FromData.Amount,
		})
	}

	if swap.ToData.LockupTransactionId != "" {
		outputDetails = append(outputDetails, boltz.OutputDetails{
			SwapId:       swap.Id,
			SwapType:     boltz.ChainSwap,
			TransactionId: swap.ToData.LockupTransactionId,
			OutputIndex:  0,
			Amount:       swap.ToData.Amount,
		})
	}

	return outputDetails, nil
}

*/

func (nursery *Nursery) setChainSwapLockupTransaction(swap *database.ChainSwap, data *database.ChainSwapData, transactionId string) error {
	data.LockupTransactionId = transactionId
	_, _, value, err := nursery.findLockupVout(data)
	if err != nil {
		return errors.New("Could not find lockup vout" + err.Error())
	}

	logger.Info("Got lockup transaction of Swap " + data.Id + " from Boltz: " + transactionId)

	if err := nursery.database.SetChainSwapLockupTransactionId(data, transactionId); err != nil {
		return errors.New("Could not set lockup transaction in database: " + err.Error())
	}

	logger.Infof("Found output for Swap %s of %d satoshis", data.Id, value)

	if data == swap.ToData || data.Wallet != "" {
		fee, err := nursery.onchain.GetTransactionFee(data.Currency, transactionId)
		if err != nil {
			return errors.New("could not get lockup transaction fee: " + err.Error())
		}
		if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
			return errors.New("could not set lockup transaction fee in database: " + err.Error())
		}
	}

	if data == swap.ToData && value < data.Amount {
		return errors.New("boltz locked up less onchain coins than expected")
	}

	return nil
}

func (nursery *Nursery) getChainSwapClaimOutput(swap *database.ChainSwap) (*boltz.OutputDetails, error) {
	lockupTransaction, vout, _, err := nursery.findLockupVout(swap.FromData)
	if err != nil {
		return nil, errors.New("Could not find lockup vout" + err.Error())
	}

	return &boltz.OutputDetails{
		SwapId:            swap.Id,
		SwapType:          boltz.ChainSwap,
		Preimage:          swap.Preimage,
		PrivateKey:        swap.ToData.PrivateKey,
		SwapTree:          swap.ToData.Tree,
		Vout:              vout,
		LockupTransaction: lockupTransaction,
		Cooperative:       true,
		RefundSwapTree:    swap.FromData.Tree,
		Address:           swap.ToData.Address,
	}, nil
}

func (nursery *Nursery) findLockupVout(data *database.ChainSwapData) (boltz.Transaction, uint32, uint64, error) {
	lockupTransaction, err := nursery.onchain.GetTransaction(data.Currency, data.LockupTransactionId, data.BlindingKey)
	if err != nil {
		return nil, 0, 0, errors.New("Could not decode lockup transaction: " + err.Error())
	}

	vout, value, err := lockupTransaction.FindVout(nursery.network, data.LockupAddress)
	if err != nil {
		return nil, 0, 0, errors.New("Could not decode lockup transaction: " + err.Error())
	}

	return lockupTransaction, vout, value, nil
}

func (nursery *Nursery) getChainSwapRefundOutput(swap *database.ChainSwap) (*boltz.OutputDetails, error) {
	lockupTransaction, vout, _, err := nursery.findLockupVout(swap.FromData)
	if err != nil {
		return nil, errors.New("Could not find lockup vout" + err.Error())
	}

	return &boltz.OutputDetails{
		SwapId:            swap.Id,
		SwapType:          boltz.ChainSwap,
		Preimage:          swap.Preimage,
		PrivateKey:        swap.FromData.PrivateKey,
		SwapTree:          swap.FromData.Tree,
		Vout:              vout,
		LockupTransaction: lockupTransaction,
		Cooperative:       true,
		Address:           swap.FromData.Address,
	}, nil
}

func (nursery *Nursery) handleChainSwapStatus(swap *database.ChainSwap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Info("Status of Swap " + swap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	handleError := func(err string) {
		if dbErr := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_ERROR, err); dbErr != nil {
			logger.Error("Could not update swap state: " + dbErr.Error())
		}
		logger.Error(err)
		nursery.sendChainSwapUpdate(*swap)
	}

	/*
		if swap.FromData.LockupTransactionId == "" || swap.ToData.LockupTransactionId == "" {
			response, err := nursery.boltz.GetChainSwapTransactions(swap.Id)
			if err != nil {
				var boltzErr boltz.Error
				if !errors.As(err, &boltzErr) {
					handleError("Could not get lockup tx from boltz: " + err.Error())
					return
				}
			} else {
				if swap.FromData.LockupTransactionId == "" {
					if err := nursery.setChainSwapLockupTransaction(swap, swap.FromData, response.UserLock.Transaction.Id); err != nil {
						handleError("Could not set lockup transaction in database: " + err.Error())
						return
					}

				}
				if swap.ToData.LockupTransactionId == "" {
					if err := nursery.setChainSwapLockupTransaction(swap, swap.ToData, response.ServerLock.Transaction.Id); err != nil {
						handleError("Could not set lockup transaction in database: " + err.Error())
						return
					}
				}
			}
		}
	*/

	switch parsedStatus {
	case boltz.TransactionServerConfirmed, boltz.TransactionServerMempoool:
		response, err := nursery.boltz.GetChainSwapTransactions(swap.Id)
		if err != nil {
			var boltzErr boltz.Error
			if !errors.As(err, &boltzErr) {
				handleError("Could not get lockup tx from boltz: " + err.Error())
				return
			}
		} else {
			if swap.FromData.LockupTransactionId == "" {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.FromData, response.UserLock.Transaction.Id); err != nil {
					handleError("Could not set lockup transaction in database: " + err.Error())
					return
				}
			}
			if swap.ToData.LockupTransactionId == "" {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.ToData, response.ServerLock.Transaction.Id); err != nil {
					handleError("Could not set lockup transaction in database: " + err.Error())
					return
				}
			}
		}

		if parsedStatus == boltz.TransactionServerMempoool && !swap.AcceptZeroConf {
			break
		}

		lockupTransaction, err := nursery.onchain.GetTransaction(swap.Pair.To, swap.ToData.LockupTransactionId, swap.ToData.BlindingKey)
		if err != nil {
			handleError("Could not decode lockup transaction: " + err.Error())
			return
		}

		vout, _, err := lockupTransaction.FindVout(nursery.network, swap.ToData.LockupAddress)
		if err != nil {
			handleError("Could not decode lockup transaction: " + err.Error())
			return
		}

		claimOutput := boltz.OutputDetails{
			SwapId:            swap.Id,
			SwapType:          boltz.ChainSwap,
			Preimage:          swap.Preimage,
			PrivateKey:        swap.ToData.PrivateKey,
			SwapTree:          swap.ToData.Tree,
			Vout:              vout,
			LockupTransaction: lockupTransaction,
			Cooperative:       true,
			RefundSwapTree:    swap.FromData.Tree,
			Address:           swap.ToData.Address,
		}

		claimTransactionId, claimFee, err := nursery.claimOutputs(swap.Pair.To, []boltz.OutputDetails{claimOutput})

		if err := nursery.database.SetChainSwapTransactionId(swap.ToData, claimTransactionId); err != nil {
			handleError("Could not set lockup transaction in database: " + err.Error())
			return
		}

		if err := nursery.database.AddChainSwapOnchainFee(swap, claimFee); err != nil {
			handleError("Could not set lockup transaction in database: " + err.Error())
			return
		}

	}

	err := nursery.database.UpdateChainSwapStatus(swap, parsedStatus)

	if err != nil {
		handleError(fmt.Sprintf("Could not update status of Chain Swap %s to %s: %s", swap.Id, parsedStatus, err))
		return
	}

	if parsedStatus.IsCompletedStatus() {
		serviceFee := uint64(swap.ServiceFeePercent.Calculate(float64(swap.FromData.Amount)))

		logger.Infof("Chain Swap service fee: %dsat onchain fee: %dsat", serviceFee, *swap.OnchainFee)

		if err := nursery.database.SetChainSwapServiceFee(swap, serviceFee); err != nil {
			handleError("Could not set swap service fee in database: " + err.Error())
			return
		}

		if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			handleError("Could not update state of Swap " + swap.Id + ": " + err.Error())
			return
		}
	} else if parsedStatus.IsFailedStatus() {
		if swap.State == boltzrpc.SwapState_PENDING {
			if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				handleError("Could not update state of Swap " + swap.Id + ": " + err.Error())
				return
			}
		}

		/*
			logger.Infof("Swap %s failed, trying to refund cooperatively", swap.Id)
			if err := nursery.RefundSwaps([]database.Swap{*swap}, true); err != nil {
				handleError("Could not refund Swap " + swap.Id + ": " + err.Error())
				return
			}
			return
		*/
	}
	nursery.sendChainSwapUpdate(*swap)

}
