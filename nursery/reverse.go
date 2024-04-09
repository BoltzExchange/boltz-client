package nursery

import (
	"fmt"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
)

func (nursery *Nursery) sendReverseSwapUpdate(reverseSwap database.ReverseSwap) {
	nursery.sendUpdate(reverseSwap.Id, SwapUpdate{
		ReverseSwap: &reverseSwap,
		IsFinal:     reverseSwap.State != boltzrpc.SwapState_PENDING,
	})
}

func (nursery *Nursery) RegisterReverseSwap(reverseSwap database.ReverseSwap) error {
	if err := nursery.registerSwap(reverseSwap.Id); err != nil {
		return err
	}
	nursery.sendReverseSwapUpdate(reverseSwap)
	return nil
}

func (nursery *Nursery) PayReverseSwap(reverseSwap *database.ReverseSwap) error {
	feeLimit, err := lightning.GetFeeLimit(reverseSwap.Invoice, nursery.network.Btc)
	if err != nil {
		return err
	}

	if nursery.lightning == nil {
		return fmt.Errorf("no lightning node available to pay invoice")
	}

	go func() {
		payment, err := nursery.lightning.PayInvoice(reverseSwap.Invoice, feeLimit, 30, reverseSwap.ChanIds)
		if err != nil {
			if dbErr := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
				logger.Error("Could not update Reverse Swap state: " + dbErr.Error())
			}
			logger.Errorf("Could not pay invoice %s: %v", reverseSwap.Invoice, err)
			nursery.sendReverseSwapUpdate(*reverseSwap)
		} else {
			logger.Info("Paid invoice of Reverse Swap " + reverseSwap.Id + " with fee of " + utils.FormatMilliSat(int64(payment.FeeMsat)) + " satoshis")
		}
	}()
	return nil
}

func (nursery *Nursery) getReverseSwapClaimOutput(reverseSwap *database.ReverseSwap) *Output {
	var blindingKey *btcec.PublicKey
	if reverseSwap.BlindingKey != nil {
		blindingKey = reverseSwap.BlindingKey.PubKey()
	}
	lockupAddress, _ := reverseSwap.SwapTree.Address(nursery.network, blindingKey)

	logger.Info("Derived lockup address: " + lockupAddress)

	return &Output{
		OutputDetails: &boltz.OutputDetails{
			SwapId:      reverseSwap.Id,
			SwapType:    boltz.ReverseSwap,
			Address:     reverseSwap.ClaimAddress,
			PrivateKey:  reverseSwap.PrivateKey,
			Preimage:    reverseSwap.Preimage,
			SwapTree:    reverseSwap.SwapTree,
			Cooperative: true,
		},
		walletId: reverseSwap.WalletId,
		voutInfo: voutInfo{
			transactionId:  reverseSwap.LockupTransactionId,
			currency:       reverseSwap.Pair.To,
			address:        lockupAddress,
			blindingKey:    reverseSwap.BlindingKey,
			expectedAmount: reverseSwap.OnchainAmount,
		},
		setTransaction: func(transactionId string, fee uint64) error {
			if err := nursery.database.SetReverseSwapClaimTransactionId(reverseSwap, transactionId, fee); err != nil {
				return fmt.Errorf("Could not set claim transaction id in database: %w", err)
			}
			return nil
		},
	}
}

// TODO: fail swap after "transaction.failed" event
func (nursery *Nursery) handleReverseSwapStatus(reverseSwap *database.ReverseSwap, event boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(event.Status)

	if parsedStatus == reverseSwap.Status {
		logger.Info("Status of Reverse Swap " + reverseSwap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	handleError := func(err string) {
		if dbErr := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_ERROR, err); dbErr != nil {
			logger.Error("Could not update Reverse Swap state: " + dbErr.Error())
		}
		logger.Error(err)
		nursery.sendReverseSwapUpdate(*reverseSwap)
	}

	switch parsedStatus {
	case boltz.TransactionMempool:
		fallthrough

	case boltz.TransactionConfirmed:
		err := nursery.database.SetReverseSwapLockupTransactionId(reverseSwap, event.Transaction.Id)

		if err != nil {
			handleError("Could not set lockup transaction id in database: " + err.Error())
			return
		}

		if parsedStatus == boltz.TransactionMempool && !reverseSwap.AcceptZeroConf {
			break
		}

		feeSatPerVbyte, err := nursery.getFeeEstimation(reverseSwap.Pair.To)

		if err != nil {
			handleError("Could not get fee estimation: " + err.Error())
			return
		}

		logger.Info(fmt.Sprintf("Using fee of %v sat/vbyte for claim transaction", feeSatPerVbyte))

		logger.Infof("Constructing claim transaction for Reverse Swap %s", reverseSwap.Id)

		output := nursery.getReverseSwapClaimOutput(reverseSwap)
		_, err = nursery.claimOutputs(reverseSwap.Pair.To, []*Output{output})
		if err != nil {
			handleError("Could not construct claim transaction: " + err.Error())
			return
		}
	}

	if err := nursery.database.UpdateReverseSwapStatus(reverseSwap, parsedStatus); err != nil {
		handleError("Could not update status of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
		return
	}

	if parsedStatus.IsCompletedStatus() {
		decodedInvoice, err := zpay32.Decode(reverseSwap.Invoice, nursery.network.Btc)
		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}

		paymentHash := *decodedInvoice.PaymentHash
		if nursery.lightning != nil {
			status, err := nursery.lightning.PaymentStatus(paymentHash[:])
			if err != nil {
				handleError("Could not get payment status: " + err.Error())
			} else if status.State == lightning.PaymentSucceeded {
				if err := nursery.database.SetReverseSwapRoutingFee(reverseSwap, status.FeeMsat); err != nil {
					handleError("Could not set reverse swap routing fee in database: " + err.Error())
					return
				}
			} else {
				logger.Warnf("Reverse Swap %s has state completed but payment did not succeed", reverseSwap.Id)
			}
		}

		invoiceAmount := uint64(decodedInvoice.MilliSat.ToSatoshis())
		serviceFee := uint64(reverseSwap.ServiceFeePercent.Calculate(float64(invoiceAmount)))
		boltzOnchainFee := invoiceAmount - reverseSwap.OnchainAmount - serviceFee

		logger.Infof("Reverse Swap service fee: %dsat; boltz onchain fee: %dsat", serviceFee, boltzOnchainFee)

		if err := nursery.database.SetReverseSwapServiceFee(reverseSwap, serviceFee, boltzOnchainFee); err != nil {
			handleError("Could not set reverse swap service fee in database: " + err.Error())
			return
		}
		if err := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			handleError("Could not update state of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
			return
		}
	} else if parsedStatus.IsFailedStatus() {
		if reverseSwap.State == boltzrpc.SwapState_PENDING {
			if err := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_SERVER_ERROR, ""); err != nil {
				handleError("Could not update state of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
				return
			}
		}
	}

	nursery.sendReverseSwapUpdate(*reverseSwap)
}
