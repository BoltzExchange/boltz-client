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

func (nursery *Nursery) setChainSwapLockupTransaction(swap *database.ChainSwap, data *database.ChainSwapData, transactionId string) error {
	data.LockupTransactionId = transactionId
	_, _, _, err := nursery.findVout(chainVoutInfo(data, false))
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

func chainVoutInfo(data *database.ChainSwapData, checkAmount bool) voutInfo {
	info := voutInfo{
		transactionId: data.LockupTransactionId,
		currency:      data.Currency,
		address:       data.LockupAddress,
		blindingKey:   data.BlindingKey,
	}
	if checkAmount {
		info.expectedAmount = data.Amount
	}
	return info
}

func (nursery *Nursery) getChainSwapClaimOutput(swap *database.ChainSwap) *Output {
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
		voutInfo: chainVoutInfo(swap.ToData, true),
		setTransaction: func(transactionId string, fee uint64) error {
			if err := nursery.database.SetChainSwapTransactionId(swap.ToData, transactionId); err != nil {
				return fmt.Errorf("Could not set lockup transaction in database: %w", err)
			}

			if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
				return fmt.Errorf("Could not set lockup transaction in database: %w", err)
			}

			return nil
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
		chainVoutInfo(swap.FromData, false),
		func(transactionId string, fee uint64) error {
			if err := nursery.database.SetChainSwapTransactionId(swap.FromData, transactionId); err != nil {
				return fmt.Errorf("could not set refund transaction id in database: %s", err)
			}

			if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_REFUNDED, ""); err != nil {
				return fmt.Errorf("could not update state: %s", err)
			}

			if err := nursery.database.AddChainSwapOnchainFee(swap, fee); err != nil {
				return fmt.Errorf("could not add onchain fee in db: %s", err)
			}

			nursery.sendChainSwapUpdate(*swap)

			return nil
		},
	}
}

func (nursery *Nursery) handleChainSwapStatus(swap *database.ChainSwap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Infof("Status of Chain Swap %s is %s already", swap.Id, parsedStatus)
		return
	}

	handleError := func(err string) {
		if dbErr := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_ERROR, err); dbErr != nil {
			logger.Errorf("Could not update Chain Swap state: %s", dbErr)
		}
		logger.Errorf("Chain Swap %s error: %s", swap.Id, err)
		nursery.sendChainSwapUpdate(*swap)
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
		_, err := nursery.createTransaction(swap.Pair.To, []*Output{output})
		if err != nil {
			handleError("Could not claim chain swap output: " + err.Error())
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

		logger.Infof("Chain Swap %s failed, trying to refund cooperatively", swap.Id)
		if err := nursery.RefundSwaps(swap.Pair.From, nil, []database.ChainSwap{*swap}); err != nil {
			handleError("Could not refund Swap " + swap.Id + ": " + err.Error())
			return
		}
		return
	}
	nursery.sendChainSwapUpdate(*swap)

}
