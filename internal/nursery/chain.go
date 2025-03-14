package nursery

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
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
	return nil
}

func (nursery *Nursery) setChainSwapLockupTransaction(swap *database.ChainSwap, data *database.ChainSwapData, transactionId string) error {
	data.LockupTransactionId = transactionId
	_, err := nursery.onchain.FindOutput(chainOutputArgs(data))
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

func chainOutputArgs(data *database.ChainSwapData) onchain.OutputArgs {
	info := onchain.OutputArgs{
		TransactionId: data.LockupTransactionId,
		Currency:      data.Currency,
		Address:       data.LockupAddress,
		BlindingKey:   data.BlindingKey,
	}
	return info
}

func (nursery *Nursery) getChainSwapClaimOutput(swap *database.ChainSwap) *Output {
	info := chainOutputArgs(swap.ToData)
	info.RequireConfirmed = !swap.AcceptZeroConf
	info.ExpectedAmount = swap.ToData.Amount
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
		walletId:   swap.ToData.WalletId,
		outputArgs: info,
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
		chainOutputArgs(swap.FromData),
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
}

func (nursery *Nursery) handleChainSwapStatus(tx *database.Transaction, swap *database.ChainSwap, status boltz.SwapStatusResponse) error {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Debugf("Status of Chain Swap %s is %s already", swap.Id, parsedStatus)
		return nil
	}

	logger.Infof("Status of Chain Swap %s changed to: %s", swap.Id, parsedStatus)

	if swap.FromData.LockupTransactionId == "" || swap.ToData.LockupTransactionId == "" {
		response, err := nursery.boltz.GetChainSwapTransactions(swap.Id)
		if err != nil {
			var boltzErr boltz.Error
			if !errors.As(err, &boltzErr) {
				return fmt.Errorf("get lockup tx from boltz: %w", err)
			}
		} else {
			if swap.FromData.LockupTransactionId == "" && response.UserLock != nil {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.FromData, response.UserLock.Transaction.Id); err != nil {
					return fmt.Errorf("could not set lockup transaction in database: %w", err)
				}
				logger.Infof("Found user lockup for Chain Swap %s", swap.Id)
			}
			if swap.ToData.LockupTransactionId == "" && response.ServerLock != nil {
				if err := nursery.setChainSwapLockupTransaction(swap, swap.ToData, response.ServerLock.Transaction.Id); err != nil {
					return fmt.Errorf("could not set lockup transaction in database: %w", err)
				}
				logger.Infof("Found server lockup for Chain Swap %s", swap.Id)
			}
		}
	}

	var quoteError boltz.Error
	switch parsedStatus {
	case boltz.TransactionLockupFailed, boltz.TransactionMempool:
		if swap.FromData.Amount == 0 || parsedStatus == boltz.TransactionLockupFailed {
			quote, err := nursery.boltz.GetChainSwapQuote(swap.Id)
			if err != nil {
				if errors.As(err, &quoteError) {
					// TODO: store error
					logger.Warnf("Boltz did not give us a new quote for Chain Swap %s: %v", swap.Id, quoteError)
				} else {
					return fmt.Errorf("could not get quote: %w", err)
				}
			}
			if quote != nil {
				result, err := nursery.onchain.FindOutput(chainOutputArgs(swap.FromData))
				if err != nil {
					return fmt.Errorf("could not find lockup vout: %w", err)
				}

				if err := nursery.CheckAmounts(boltz.ChainSwap, swap.Pair, result.Value, quote.Amount, swap.ServiceFeePercent); err != nil {
					return fmt.Errorf("quote amounts not correct: %w", err)
				}

				if err := nursery.boltz.AcceptChainSwapQuote(swap.Id, quote); err != nil {
					return fmt.Errorf("could not accept quote: %w", err)
				}

				if err := tx.SetChainSwapAmount(swap.ToData, quote.Amount); err != nil {
					return fmt.Errorf("update to to amount: %w", err)
				}

				if err := tx.SetChainSwapAmount(swap.FromData, result.Value); err != nil {
					return fmt.Errorf("update from from amount: %w", err)
				}
			}
		}

	case boltz.TransactionServerConfirmed, boltz.TransactionServerMempoool:
		if (parsedStatus == boltz.TransactionServerMempoool && !swap.AcceptZeroConf) || swap.ToData.Transactionid != "" {
			break
		}

		output := nursery.getChainSwapClaimOutput(swap)
		if _, err := nursery.createTransaction(swap.Pair.To, []*Output{output}); err != nil {
			return fmt.Errorf("could not claim chain swap output: %w", err)
		}
	default:
	}

	logger.Debugf("Updating status of Chain Swap %s to %s", swap.Id, parsedStatus)

	err := nursery.database.UpdateChainSwapStatus(swap, parsedStatus)
	if err != nil {
		return fmt.Errorf("could not update status to %s: %w", parsedStatus, err)
	}

	if parsedStatus.IsCompletedStatus() {
		serviceFee := swap.ServiceFeePercent.Calculate(swap.FromData.Amount)

		logger.Infof("Chain Swap service fee: %dsat onchain fee: %dsat", serviceFee, *swap.OnchainFee)

		if err := nursery.database.SetChainSwapServiceFee(swap, serviceFee); err != nil {
			return fmt.Errorf("could not set swap service fee in database: %w", err)
		}

		if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			return fmt.Errorf("could not update swap state: %w", err)
		}
	} else if parsedStatus.IsFailedStatus() {
		// only set to SERVER_ERROR if we are not eligible for a new quote
		if parsedStatus != boltz.TransactionLockupFailed || quoteError != nil {
			logger.Infof("Chain Swap %s failed", swap.Id)

			if swap.State == boltzrpc.SwapState_PENDING {
				if err := nursery.database.UpdateChainSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
					return fmt.Errorf("could not update swap state: %w", err)
				}
			}

			if swap.FromData.LockupTransactionId != "" {
				if _, err := nursery.RefundSwaps(swap.Pair.From, nil, []*database.ChainSwap{swap}); err != nil {
					return fmt.Errorf("could not refund swap: %w", err)
				}
			}

		}
	}
	return nil
}
