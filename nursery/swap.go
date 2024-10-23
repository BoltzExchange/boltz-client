package nursery

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
)

func (nursery *Nursery) sendSwapUpdate(swap database.Swap) {
	isFinal := swap.State == boltzrpc.SwapState_SUCCESSFUL || swap.State == boltzrpc.SwapState_REFUNDED
	if swap.LockupTransactionId == "" && swap.State != boltzrpc.SwapState_PENDING {
		isFinal = false
	}

	nursery.sendUpdate(swap.Id, SwapUpdate{
		Swap:    &swap,
		IsFinal: isFinal,
	})
}

// TODO: abstract interactions with chain (querying and broadcasting transactions) into interface to be able to switch between Boltz API and bitcoin core

type Output struct {
	*boltz.OutputDetails
	walletId *database.Id
	findArgs onchain.OutputArgs

	setTransaction func(transactionId string, fee uint64) error
	setError       func(err error)
}

func swapOutputArgs(swap *database.Swap) onchain.OutputArgs {
	return onchain.OutputArgs{
		TransactionId: swap.LockupTransactionId,
		Currency:      swap.Pair.From,
		Address:       swap.Address,
		BlindingKey:   swap.BlindingKey,
	}
}

func (nursery *Nursery) getRefundOutput(swap *database.Swap) *Output {
	return &Output{
		OutputDetails: &boltz.OutputDetails{
			SwapId:             swap.Id,
			SwapType:           boltz.NormalSwap,
			Address:            swap.RefundAddress,
			PrivateKey:         swap.PrivateKey,
			Preimage:           []byte{},
			TimeoutBlockHeight: swap.TimoutBlockHeight,
			SwapTree:           swap.SwapTree,
			// TODO: remember if cooperative fails and set this to false
			Cooperative: true,
		},
		walletId: swap.WalletId,
		findArgs: swapOutputArgs(swap),
		setTransaction: func(transactionId string, fee uint64) error {
			if err := nursery.database.SetSwapRefundTransactionId(swap, transactionId, fee); err != nil {
				return err
			}

			nursery.sendSwapUpdate(*swap)

			return nil
		},
		setError: func(err error) {
			nursery.handleSwapError(swap, err)
		},
	}
}

func (nursery *Nursery) RegisterSwap(swap database.Swap) error {
	if err := nursery.registerSwaps([]string{swap.Id}); err != nil {
		return err
	}
	nursery.sendSwapUpdate(swap)
	return nil
}

func (nursery *Nursery) cooperativeSwapClaim(swap *database.Swap, status boltz.SwapStatusResponse) error {
	logger.Debugf("Trying to claim swap %s cooperatively", swap.Id)

	claimDetails, err := nursery.boltz.GetSwapClaimDetails(swap.Id)
	if err != nil {
		return fmt.Errorf("Could not get claim details from boltz: %w", err)
	}

	// Verify that the invoice was actually paid
	decodedInvoice, err := lightning.DecodeInvoice(swap.Invoice, nursery.network.Btc)
	if err != nil {
		return fmt.Errorf("could not decode swap invoice: %w", err)
	}

	preimageHash := sha256.Sum256(claimDetails.Preimage)
	if !bytes.Equal(decodedInvoice.PaymentHash[:], preimageHash[:]) {
		return fmt.Errorf("boltz returned wrong preimage: %x", claimDetails.Preimage)
	}

	session, err := boltz.NewSigningSession(swap.SwapTree)
	if err != nil {
		return fmt.Errorf("could not create signing session: %w", err)
	}

	partial, err := session.Sign(claimDetails.TransactionHash, claimDetails.PubNonce)
	if err != nil {
		return fmt.Errorf("could not create partial signature: %w", err)
	}

	if err := nursery.boltz.SendSwapClaimSignature(swap.Id, partial); err != nil {
		return fmt.Errorf("could not send partial signature to boltz: %w", err)
	}
	return nil
}

func (nursery *Nursery) handleSwapError(swap *database.Swap, err error) {
	if dbErr := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
		logger.Error(dbErr.Error())
	}
	logger.Errorf("Swap %s error: %v", swap.Id, err)
	nursery.sendSwapUpdate(*swap)
}

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Debugf("Status of Swap %s is %s already", swap.Id, parsedStatus)
		return
	}

	logger.Infof("Status of Swap %s changed to: %s", swap.Id, parsedStatus)

	handleError := func(err string) {
		nursery.handleSwapError(swap, errors.New(err))
	}

	if parsedStatus != boltz.InvoiceSet && swap.LockupTransactionId == "" {
		swapTransactionResponse, err := nursery.boltz.GetSwapTransaction(swap.Id)
		if err != nil {
			var boltzErr boltz.Error
			if !errors.As(err, &boltzErr) {
				handleError("Could not get lockup tx from boltz: " + err.Error())
				return
			}
		} else {
			lockupTransaction, err := boltz.NewTxFromHex(swap.Pair.From, swapTransactionResponse.TransactionHex, swap.BlindingKey)
			if err != nil {
				handleError("Could not decode lockup transaction: " + err.Error())
				return
			}

			if err := nursery.database.SetSwapLockupTransactionId(swap, lockupTransaction.Hash()); err != nil {
				handleError("Could not set lockup transaction in database: " + err.Error())
				return
			}

			result, err := nursery.onchain.FindOutput(swapOutputArgs(swap))
			if err != nil {
				handleError(err.Error())
				return
			}

			logger.Infof("Got lockup transaction of Swap %s: %s", swap.Id, lockupTransaction.Hash())

			if err := nursery.database.SetSwapExpectedAmount(swap, result.Value); err != nil {
				handleError("Could not set expected amount in database: " + err.Error())
				return
			}

			// dont add onchain fee if the swap was paid externally as it might have been part of a larger transaction
			if swap.WalletId != nil {
				fee, err := nursery.onchain.GetTransactionFee(swap.Pair.From, swap.LockupTransactionId)
				if err != nil {
					handleError("could not get lockup transaction fee: " + err.Error())
					return
				}
				if err := nursery.database.SetSwapOnchainFee(swap, fee); err != nil {
					handleError("could not set lockup transaction fee in database: " + err.Error())
					return
				}
			}
		}
	}

	switch parsedStatus {
	case boltz.TransactionMempool:
		fallthrough

	case boltz.TransactionConfirmed:
		// Set the invoice of Swaps that were created with only a preimage hash
		if swap.Invoice != "" {
			break
		}

		swapRates, err := nursery.boltz.GetInvoiceAmount(swap.Id)
		if err != nil {
			handleError("Could not query Swap rates of Swap " + swap.Id + ": " + err.Error())
			return
		}

		if err := nursery.CheckAmounts(boltz.NormalSwap, swap.Pair, swap.ExpectedAmount, swapRates.InvoiceAmount, swap.ServiceFeePercent); err != nil {
			handleError(fmt.Sprintf("not accepting invoice amount %d from boltz: %s", swapRates.InvoiceAmount, err))
			return
		}

		blockHeight, err := nursery.onchain.GetBlockHeight(swap.Pair.From)

		if err != nil {
			handleError("Could not get block height: " + err.Error())
			return
		}

		if nursery.lightning == nil {
			handleError("No lightning node available, can not create invoice for Swap " + swap.Id)
			return
		}

		invoice, err := nursery.lightning.CreateInvoice(
			swapRates.InvoiceAmount,
			swap.Preimage,
			boltz.CalculateInvoiceExpiry(swap.TimoutBlockHeight-blockHeight, swap.Pair.From),
			utils.GetSwapMemo(string(swap.Pair.From)),
		)

		if err != nil {
			handleError("Could not get new invoice for Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Infof("Generated new invoice for Swap %s for %d saothis", swap.Id, swapRates.InvoiceAmount)

		_, err = nursery.boltz.SetInvoice(swap.Id, invoice.PaymentRequest)

		if err != nil {
			handleError("Could not set invoice of Swap: " + err.Error())
			return
		}

		err = nursery.database.SetSwapInvoice(swap, invoice.PaymentRequest)

		if err != nil {
			handleError("Could not set invoice of Swap in database: " + err.Error())
			return
		}

	case boltz.TransactionClaimPending, boltz.TransactionClaimed:
		// Verify that the invoice was actually paid
		decodedInvoice, err := lightning.DecodeInvoice(swap.Invoice, nursery.network.Btc)

		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}

		if nursery.lightning != nil {
			paid, err := nursery.lightning.CheckInvoicePaid(decodedInvoice.PaymentHash[:])
			if err != nil {
				if !errors.Is(err, lightning.ErrInvoiceNotFound) {
					handleError("Could not get invoice information from lightning node: " + err.Error())
					return
				}
			} else if !paid {
				logger.Warnf("Swap %s was not actually settled. Refunding at block %d", swap.Id, swap.TimoutBlockHeight)
				return
			}
		}

		logger.Infof("Swap %s succeeded", swap.Id)

		if parsedStatus == boltz.TransactionClaimPending {
			if err := nursery.cooperativeSwapClaim(swap, status); err != nil {
				logger.Warnf("Could not claim swap %s cooperatively: %s", swap.Id, err)
			}
		}
	}

	err := nursery.database.UpdateSwapStatus(swap, parsedStatus)

	if err != nil {
		handleError(fmt.Sprintf("Could not update status of Swap %s to %s: %s", swap.Id, parsedStatus, err))
		return
	}

	if parsedStatus.IsCompletedStatus() {
		decodedInvoice, err := lightning.DecodeInvoice(swap.Invoice, nursery.network.Btc)
		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}
		serviceFee := swap.ServiceFeePercent.Calculate(decodedInvoice.AmountSat)
		boltzOnchainFee := int64(swap.ExpectedAmount - decodedInvoice.AmountSat - serviceFee)
		if boltzOnchainFee < 0 {
			logger.Warnf("Boltz onchain fee seems to be negative")
			boltzOnchainFee = 0
		}

		logger.Infof("Swap service fee: %dsat onchain fee: %dsat", serviceFee, boltzOnchainFee)

		if err := nursery.database.SetSwapServiceFee(swap, serviceFee, uint64(boltzOnchainFee)); err != nil {
			handleError("Could not set swap service fee in database: " + err.Error())
			return
		}

		if err := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			handleError(err.Error())
			return
		}
	} else if parsedStatus.IsFailedStatus() {
		if swap.State == boltzrpc.SwapState_PENDING {
			if err := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				handleError(err.Error())
				return
			}
		}

		logger.Infof("Swap %s failed", swap.Id)
		if err := nursery.checkSweep(nursery.getRefundOutput(swap)); err != nil {
			return
		}
		return
	}
	nursery.sendSwapUpdate(*swap)
}
