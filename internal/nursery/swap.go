package nursery

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
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
	walletId   *database.Id
	outputArgs onchain.OutputArgs

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
		walletId:   swap.WalletId,
		outputArgs: swapOutputArgs(swap),
		setTransaction: func(transactionId string, fee uint64) error {
			return nursery.database.SetSwapRefundTransactionId(swap, transactionId, fee)
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
}

func (nursery *Nursery) handleSwapStatus(tx *database.Transaction, swap *database.Swap, status boltz.SwapStatusResponse) error {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Debugf("Status of Swap %s is %s already", swap.Id, parsedStatus)
		return nil
	}

	logger.Infof("Status of Swap %s changed to: %s", swap.Id, parsedStatus)

	if parsedStatus != boltz.InvoiceSet && swap.LockupTransactionId == "" {
		swapTransactionResponse, err := nursery.boltz.GetSwapTransaction(swap.Id)
		if err != nil {
			var boltzErr boltz.Error
			if !errors.As(err, &boltzErr) {
				return fmt.Errorf("could not get lockup tx from boltz: %w", err)
			}
		} else {
			lockupTransaction, err := boltz.NewTxFromHex(swap.Pair.From, swapTransactionResponse.Hex, swap.BlindingKey)
			if err != nil {
				return fmt.Errorf("could not decode lockup transaction: %w", err)
			}

			if err := tx.SetSwapLockupTransactionId(swap, lockupTransaction.Hash()); err != nil {
				return fmt.Errorf("could not set lockup transaction in database: %w", err)
			}

			result, err := nursery.onchain.FindOutput(swapOutputArgs(swap))
			if err != nil {
				return err
			}

			logger.Infof("Got lockup transaction of Swap %s: %s", swap.Id, lockupTransaction.Hash())

			if err := tx.SetSwapExpectedAmount(swap, result.Value); err != nil {
				return fmt.Errorf("could not set expected amount in database: %w", err)
			}

			// dont add onchain fee if the swap was paid externally as it might have been part of a larger transaction
			if swap.WalletId != nil {
				fee, err := nursery.onchain.GetTransactionFee(swap.Pair.From, swap.LockupTransactionId)
				if err != nil {
					return fmt.Errorf("could not get lockup transaction fee: %w", err)
				}
				if err := tx.SetSwapOnchainFee(swap, fee); err != nil {
					return fmt.Errorf("could not set lockup transaction fee in database: %w", err)
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
			return fmt.Errorf("could not query swap rates of swap %s: %w", swap.Id, err)
		}

		if err := nursery.CheckAmounts(boltz.NormalSwap, swap.Pair, swap.ExpectedAmount, swapRates.InvoiceAmount, swap.ServiceFeePercent); err != nil {
			return fmt.Errorf("not accepting invoice amount %d from boltz: %s", swapRates.InvoiceAmount, err)
		}

		blockHeight, err := nursery.onchain.GetBlockHeight(swap.Pair.From)

		if err != nil {
			return fmt.Errorf("could not get block height: %w", err)
		}

		if nursery.lightning == nil {
			return fmt.Errorf("no lightning node available, can not create invoice for swap %s", swap.Id)
		}

		invoice, err := nursery.lightning.CreateInvoice(
			swapRates.InvoiceAmount,
			swap.Preimage,
			boltz.CalculateInvoiceExpiry(swap.TimoutBlockHeight-blockHeight, swap.Pair.From),
			utils.GetSwapMemo(string(swap.Pair.From)),
		)

		if err != nil {
			return fmt.Errorf("could not get new invoice for swap %s: %w", swap.Id, err)
		}

		logger.Infof("Generated new invoice for Swap %s for %d saothis", swap.Id, swapRates.InvoiceAmount)

		_, err = nursery.boltz.SetInvoice(swap.Id, invoice.PaymentRequest)

		if err != nil {
			return fmt.Errorf("could not set invoice of swap %s: %w", swap.Id, err)
		}

		err = tx.SetSwapInvoice(swap, invoice.PaymentRequest)

		if err != nil {
			return fmt.Errorf("could not set invoice of swap %s in database: %w", swap.Id, err)
		}

	case boltz.TransactionClaimPending, boltz.TransactionClaimed:
		// Verify that the invoice was actually paid
		decodedInvoice, err := lightning.DecodeInvoice(swap.Invoice, nursery.network.Btc)

		if err != nil {
			return fmt.Errorf("could not decode swap invoice: %w", err)
		}

		if nursery.lightning != nil {
			paid, err := nursery.lightning.CheckInvoicePaid(decodedInvoice.PaymentHash[:])
			if err != nil {
				if !errors.Is(err, lightning.ErrInvoiceNotFound) {
					return fmt.Errorf("could not get invoice information from lightning node: %w", err)
				}
			} else if !paid {
				return fmt.Errorf("invoice was not actually paid. refunding at block %d", swap.TimoutBlockHeight)
			}
		}

		logger.Infof("Swap %s succeeded", swap.Id)

		if parsedStatus == boltz.TransactionClaimPending {
			if err := nursery.cooperativeSwapClaim(swap, status); err != nil {
				logger.Warnf("Could not claim swap %s cooperatively: %s", swap.Id, err)
			}
		}
	}

	err := tx.UpdateSwapStatus(swap, parsedStatus)
	if err != nil {
		return fmt.Errorf("could not update status of swap %s to %s: %w", swap.Id, parsedStatus, err)
	}

	if parsedStatus.IsCompletedStatus() {
		decodedInvoice, err := lightning.DecodeInvoice(swap.Invoice, nursery.network.Btc)
		if err != nil {
			return fmt.Errorf("could not decode invoice: %w", err)
		}
		serviceFee := swap.ServiceFeePercent.Calculate(decodedInvoice.AmountSat)
		boltzOnchainFee := int64(swap.ExpectedAmount - decodedInvoice.AmountSat - serviceFee)
		if boltzOnchainFee < 0 {
			logger.Warnf("Boltz onchain fee seems to be negative")
			boltzOnchainFee = 0
		}

		logger.Infof("Swap service fee: %dsat onchain fee: %dsat", serviceFee, boltzOnchainFee)

		if err := tx.SetSwapServiceFee(swap, serviceFee, uint64(boltzOnchainFee)); err != nil {
			return fmt.Errorf("could not set swap service fee in database: %w", err)
		}

		if err := tx.UpdateSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			return fmt.Errorf("update swap state: %w", err)
		}
	} else if parsedStatus.IsFailedStatus() {
		if swap.State == boltzrpc.SwapState_PENDING {
			if err := tx.UpdateSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				return fmt.Errorf("update swap state: %w", err)
			}
		}

		logger.Infof("Swap %s failed, trying to refund cooperatively", swap.Id)
		if _, err := nursery.RefundSwaps(swap.Pair.From, []*database.Swap{swap}, nil); err != nil {
			return fmt.Errorf("could not refund: %w", err)
		}
	}
	return nil
}
