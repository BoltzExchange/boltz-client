package nursery

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/lightningnetwork/lnd/zpay32"
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

func (nursery *Nursery) startBlockListener(currency boltz.Currency) {
	blockNotifier := nursery.registerBlockListener(currency)

	go func() {
		for newBlock := range blockNotifier {
			if nursery.stopped {
				return
			}
			swapsToRefund, err := nursery.database.QueryRefundableSwaps(newBlock.Height, currency)
			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swapsToRefund) > 0 {
				logger.Info("Found " + strconv.Itoa(len(swapsToRefund)) + " Swaps to refund at height " + strconv.FormatUint(uint64(newBlock.Height), 10))

				if err := nursery.refundSwaps(swapsToRefund, false); err != nil {
					logger.Error("Could not refund Swaps: " + err.Error())
				}
			}
		}
	}()
}

func (nursery *Nursery) refundSwaps(swapsToRefund []database.Swap, cooperative bool) error {
	currency := swapsToRefund[0].Pair.From

	var refundedSwaps []database.Swap
	var refundOutputs []boltz.OutputDetails

	for _, swapToRefund := range swapsToRefund {
		refundOutput, err := nursery.getRefundOutput(&swapToRefund)
		if err != nil {
			logger.Warnf("Could not get refund output of swap %s: %v", swapToRefund.Id, err)
			continue
		}

		refundOutput.Cooperative = cooperative
		if refundOutput.Address == "" {
			wallet, err := nursery.onchain.GetAnyWallet(currency, true)
			if err != nil {
				message := "%d Swaps can not be refunded because they got no refund address and no wallet for currency %s is available! Set up a wallet to refund"
				return fmt.Errorf(message, len(refundedSwaps), currency)
			}
			refundOutput.Address, err = wallet.NewAddress()
			if err != nil {
				return fmt.Errorf("%d swaps can not be refunded because they got no refund address and wallet failed to generate address: %v", len(refundedSwaps), err)
			}
		}
		refundedSwaps = append(refundedSwaps, swapToRefund)
		refundOutputs = append(refundOutputs, *refundOutput)
	}

	if len(refundOutputs) == 0 {
		logger.Info("Did not find any outputs to refund")
		return nil
	}

	feeSatPerVbyte, err := nursery.getFeeEstimation(currency)

	if err != nil {
		return errors.New("Could not get fee estimation: " + err.Error())
	}

	logger.Info(fmt.Sprintf("Using fee of %v sat/vbyte for refund transaction", feeSatPerVbyte))

	var signer boltz.Signer = func(transaction string, pubNonce string, i int) (*boltz.PartialSignature, error) {
		return nursery.boltz.RefundSwap(boltz.RefundSwapRequest{
			Id:          swapsToRefund[i].Id,
			PubNonce:    pubNonce,
			Transaction: transaction,
			Index:       i,
		})
	}
	refundTransactionId, totalRefundFee, err := nursery.createTransaction(currency, refundOutputs, feeSatPerVbyte, signer)
	if err != nil {
		return errors.New("Could not create refund transaction: " + err.Error())
	}

	logger.Infof("Constructed refund transaction for %d swaps: %s", len(refundOutputs), refundTransactionId)

	count := uint64(len(refundedSwaps))
	refundFee := totalRefundFee / count
	for i, refundedSwap := range refundedSwaps {
		// distribute the remainder of the fee to the last swap
		if i == int(count)-1 {
			refundFee += totalRefundFee % count
		}
		err = nursery.database.SetSwapRefundTransactionId(&refundedSwap, refundTransactionId, refundFee)

		if err != nil {
			logger.Error("Could not set refund transaction id in database: " + err.Error())
			continue
		}

		nursery.sendSwapUpdate(refundedSwap)

		logger.Infof("Refunded Swap %s with refund transaction %s", refundedSwap.Id, refundTransactionId)
	}
	return nil
}

func (nursery *Nursery) getRefundOutput(swap *database.Swap) (*boltz.OutputDetails, error) {
	lockupTransaction, err := nursery.onchain.GetTransaction(swap.Pair.From, swap.LockupTransactionId, swap.BlindingKey)
	if err != nil {
		return nil, errors.New("could not fetch lockup transaction: " + err.Error())
	}

	lockupVout, _, err := lockupTransaction.FindVout(nursery.network, swap.Address)
	if err != nil {
		return nil, fmt.Errorf("could not find lockup vout of Swap %s: %w", swap.Id, err)
	}

	return &boltz.OutputDetails{
		SwapId:             swap.Id,
		SwapType:           boltz.NormalSwap,
		LockupTransaction:  lockupTransaction,
		Vout:               lockupVout,
		Address:            swap.RefundAddress,
		PrivateKey:         swap.PrivateKey,
		Preimage:           []byte{},
		TimeoutBlockHeight: swap.TimoutBlockHeight,
		SwapTree:           swap.SwapTree,
		// TODO: remember if cooperative fails and set this to false
		Cooperative: true,
	}, nil
}

func (nursery *Nursery) RegisterSwap(swap database.Swap) error {
	if err := nursery.registerSwap(swap.Id); err != nil {
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
	decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.network.Btc)
	if err != nil {
		return fmt.Errorf("could not decode swap invoice: %w", err)
	}

	preimageHash := sha256.Sum256(claimDetails.Preimage)
	if !bytes.Equal(decodedInvoice.PaymentHash[:], preimageHash[:]) {
		return fmt.Errorf("boltz returned wrong preimage: %x", claimDetails.Preimage)
	}

	output, err := nursery.getRefundOutput(swap)
	if err != nil {
		return fmt.Errorf("could not get swap output: %w", err)
	}
	session, err := boltz.NewSigningSession([]boltz.OutputDetails{*output}, 0)
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

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Info("Status of Swap " + swap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	handleError := func(err string) {
		if dbErr := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_ERROR, err); dbErr != nil {
			logger.Error("Could not update swap state: " + dbErr.Error())
		}
		logger.Error(err)
		nursery.sendSwapUpdate(*swap)
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

			logger.Info("Got lockup transaction of Swap " + swap.Id + " from Boltz: " + lockupTransaction.Hash())

			if err := nursery.database.SetSwapLockupTransactionId(swap, lockupTransaction.Hash()); err != nil {
				handleError("Could not set lockup transaction in database: " + err.Error())
				return
			}

			_, value, err := lockupTransaction.FindVout(nursery.network, swap.Address)
			if err != nil {
				handleError("Could not find lockup vout of Swap " + swap.Id)
				return
			}

			logger.Infof("Found output for Swap %s of %d satoshis", swap.Id, value)

			if err := nursery.database.SetSwapExpectedAmount(swap, value); err != nil {
				handleError("Could not set expected amount in database: " + err.Error())
				return
			}

			if swap.AutoSend {
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

		blockHeight, err := nursery.onchain.GetBlockHeight(swap.Pair.From)

		if err != nil {
			handleError("Could not get block height: " + err.Error())
			return
		}

		invoice, err := nursery.lightning.CreateInvoice(
			int64(swapRates.InvoiceAmount),
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

	case boltz.TransactionClaimed:
	case boltz.TransactionClaimPending:
		// Verify that the invoice was actually paid
		decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.network.Btc)

		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}

		paid, err := nursery.lightning.CheckInvoicePaid(decodedInvoice.PaymentHash[:])

		if err != nil {
			handleError("Could not get invoice information from lightning node: " + err.Error())
			return
		}

		if !paid {
			logger.Warnf("Swap %s was not actually settled. Refunding at block %d", swap.Id, swap.TimoutBlockHeight)
			return
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
		handleError("Could not update status of Swap " + swap.Id + ": " + err.Error())
		return
	}

	if parsedStatus.IsCompletedStatus() {
		decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.network.Btc)
		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}
		invoiceAmount := uint64(decodedInvoice.MilliSat.ToSatoshis())
		serviceFee := uint64(swap.ServiceFeePercent.Calculate(float64(swap.ExpectedAmount)))
		boltzOnchainFee := swap.ExpectedAmount - invoiceAmount - serviceFee

		logger.Infof("Swap service fee: %dsat onchain fee: %dsat", serviceFee, boltzOnchainFee)

		if err := nursery.database.SetSwapServiceFee(swap, serviceFee, boltzOnchainFee); err != nil {
			handleError("Could not set swap service fee in database: " + err.Error())
			return
		}

		if err := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			handleError("Could not update state of Swap " + swap.Id + ": " + err.Error())
			return
		}
	} else if parsedStatus.IsFailedStatus() {
		if swap.State == boltzrpc.SwapState_PENDING {
			if err := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				handleError("Could not update state of Swap " + swap.Id + ": " + err.Error())
				return
			}
		}

		logger.Infof("Swap %s failed, trying to refund cooperatively", swap.Id)
		if err := nursery.refundSwaps([]database.Swap{*swap}, true); err != nil {
			handleError("Could not refund Swap " + swap.Id + ": " + err.Error())
			return
		}
		return
	}
	nursery.sendSwapUpdate(*swap)
}
