package nursery

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/lightningnetwork/lnd/zpay32"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/btcec/v2"
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

		lockupTx, err := boltz.NewTxFromHex(reverseSwap.Pair.To, event.Transaction.Hex, reverseSwap.BlindingKey)
		if err != nil {
			handleError("Could not decode lockup transaction: " + err.Error())
			return
		}

		var blindingKey *btcec.PublicKey
		if reverseSwap.BlindingKey != nil {
			blindingKey = reverseSwap.BlindingKey.PubKey()
		}
		lockupAddress, err := reverseSwap.SwapTree.Address(nursery.network, blindingKey)

		logger.Info("Derived lockup address: " + lockupAddress)

		if err != nil {
			handleError("Could not derive lockup address: " + err.Error())
			return
		}

		lockupVout, lockupValue, err := lockupTx.FindVout(nursery.network, lockupAddress)
		if err != nil {
			handleError("Could not find lockup vout of Reverse Swap " + reverseSwap.Id)
			return
		}

		if lockupValue < reverseSwap.OnchainAmount {
			handleError("Boltz locked up less onchain coins than expected. Abandoning Reverse Swap")
			return
		}

		logger.Info("Constructing claim transaction for Reverse Swap " + reverseSwap.Id + " with output: " + lockupTx.Hash() + ":" + strconv.Itoa(int(lockupVout)))

		output := boltz.OutputDetails{
			LockupTransaction: lockupTx,
			Vout:              lockupVout,
			PrivateKey:        reverseSwap.PrivateKey,
			Preimage:          reverseSwap.Preimage,
			SwapTree:          reverseSwap.SwapTree,
			Cooperative:       true,
		}

		claimTransactionId, claimFee, err := nursery.claimReverseSwap(reverseSwap, output, feeSatPerVbyte)
		if err != nil {
			logger.Warnf("Could not construct cooperative claim transaction: %v", err)
			output.Cooperative = false
			claimTransactionId, claimFee, err = nursery.claimReverseSwap(reverseSwap, output, feeSatPerVbyte)
			if err != nil {
				handleError("Could not construct claim transaction: " + err.Error())
				return
			}
		}

		err = nursery.database.SetReverseSwapClaimTransactionId(reverseSwap, claimTransactionId, claimFee)

		if err != nil {
			handleError("Could not set claim transaction id in database: " + err.Error())
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
		status, err := nursery.lightning.PaymentStatus(paymentHash[:])
		if err != nil {
			handleError("Could not get payment status: " + err.Error())
		} else if status.State == lightning.PaymentSucceeded {
			if err := nursery.database.SetReverseSwapRoutingFee(reverseSwap, status.FeeMsat); err != nil {
				handleError("Could not set reverse swap routing fee in database: " + err.Error())
				return
			}
		} else {
			logger.Warn("Reverse Swap " + reverseSwap.Id + " has state completed but payment didnt succeed")
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

func (nursery *Nursery) claimReverseSwap(reverseSwap *database.ReverseSwap, output boltz.OutputDetails, feeSatPerVbyte float64) (string, uint64, error) {
	var signer boltz.Signer = func(transaction string, pubNonce string, i int) (*boltz.PartialSignature, error) {
		return nursery.boltz.ClaimReverseSwap(boltz.ClaimReverseSwapRequest{
			Id:          reverseSwap.Id,
			Preimage:    hex.EncodeToString(reverseSwap.Preimage),
			PubNonce:    pubNonce,
			Transaction: transaction,
			Index:       i,
		})
	}

	return nursery.createTransaction(reverseSwap.Pair.To, []boltz.OutputDetails{output}, reverseSwap.ClaimAddress, feeSatPerVbyte, signer)
}
