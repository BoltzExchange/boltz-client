package nursery

import (
	"encoding/hex"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/btcsuite/btcd/btcutil"
	"strconv"
)

func (nursery *Nursery) recoverReverseSwaps() error {
	logger.Info("Recovering pending Reverse Swaps")

	reverseSwaps, err := nursery.database.QueryPendingReverseSwaps()

	if err != nil {
		return err
	}

	for _, reverseSwap := range reverseSwaps {
		logger.Info("Recovering Reverse Swap " + reverseSwap.Id + " at state: " + reverseSwap.Status.String())

		// TODO: handle race condition when status is updated between the POST request and the time the streaming starts
		status, err := nursery.boltz.SwapStatus(reverseSwap.Id)

		if err != nil {
			logger.Warning("Boltz could not find Reverse Swap " + reverseSwap.Id + ": " + err.Error())
			continue
		}

		if status.Status != reverseSwap.Status.String() {
			logger.Info("Swap " + reverseSwap.Id + " status changed to: " + status.Status)
			nursery.handleReverseSwapStatus(&reverseSwap, *status, nil)

			if reverseSwap.State == boltzrpc.SwapState_PENDING {
				nursery.RegisterReverseSwap(reverseSwap, nil)
			}

			continue
		}

		logger.Info("Reverse Swap " + reverseSwap.Id + " status did not change")
		nursery.RegisterReverseSwap(reverseSwap, nil)
	}

	return nil
}

func (nursery *Nursery) RegisterReverseSwap(reverseSwap database.ReverseSwap, claimTransactionIdChan chan string) chan string {
	logger.Info("Listening to events of Reverse Swap " + reverseSwap.Id)

	go func() {
		stopListening := make(chan bool)
		stopHandler := make(chan bool)

		eventListenersLock.Lock()
		eventListeners[reverseSwap.Id] = stopListening
		eventListenersLock.Unlock()

		eventStream := make(chan *boltz.SwapStatusResponse)

		nursery.streamSwapStatus(reverseSwap.Id, "Reverse Swap", eventStream, stopListening, stopHandler)

		for {
			select {
			case event := <-eventStream:
				logger.Info("Reverse Swap " + reverseSwap.Id + " status update: " + event.Status)
				nursery.handleReverseSwapStatus(&reverseSwap, *event, claimTransactionIdChan)

				// The event listening can stop after the Reverse Swap has succeeded
				if reverseSwap.Status == boltz.InvoiceSettled {
					stopListening <- true
				}

				break

			case <-stopHandler:
				return
			}
		}
	}()

	return claimTransactionIdChan
}

// TODO: fail swap after "transaction.failed" event
func (nursery *Nursery) handleReverseSwapStatus(reverseSwap *database.ReverseSwap, event boltz.SwapStatusResponse, claimTransactionIdChan chan string) {
	parsedStatus := boltz.ParseEvent(event.Status)

	if parsedStatus == reverseSwap.Status {
		logger.Info("Status of Reverse Swap " + reverseSwap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	switch parsedStatus {
	case boltz.TransactionMempool:
		fallthrough

	case boltz.TransactionConfirmed:
		if parsedStatus == boltz.TransactionMempool && !reverseSwap.AcceptZeroConf {
			break
		}

		lockupTransactionRaw, err := hex.DecodeString(event.Transaction.Hex)

		if err != nil {
			logger.Error("Could not decode lockup transaction: " + err.Error())
			return
		}

		lockupTransaction, err := btcutil.NewTxFromBytes(lockupTransactionRaw)

		if err != nil {
			logger.Error("Could not parse lockup transaction: " + err.Error())
			return
		}

		err = nursery.database.SetReverseSwapLockupTransactionId(reverseSwap, lockupTransaction.Hash().String())

		if err != nil {
			logger.Error("Could not set lockup transaction id in database: " + err.Error())
			return
		}

		lockupAddress, err := boltz.WitnessScriptHashAddress(nursery.chainParams, reverseSwap.RedeemScript)

		if err != nil {
			logger.Error("Could not derive lockup address: " + err.Error())
			return
		}

		lockupVout, err := nursery.findLockupVout(lockupAddress, lockupTransaction.MsgTx().TxOut)

		if err != nil {
			logger.Error("Could not find lockup vout of Reverse Swap " + reverseSwap.Id)
			return
		}

		if lockupTransaction.MsgTx().TxOut[lockupVout].Value < int64(reverseSwap.OnchainAmount) {
			logger.Warning("Boltz locked up less onchain coins than expected. Abandoning Reverse Swap")
			return
		}

		logger.Info("Constructing claim transaction for Reverse Swap " + reverseSwap.Id + " with output: " + lockupTransaction.Hash().String() + ":" + strconv.Itoa(int(lockupVout)))

		claimAddress, err := btcutil.DecodeAddress(reverseSwap.ClaimAddress, nursery.chainParams)

		if err != nil {
			logger.Error("Could not decode claim address of Reverse Swap: " + err.Error())
			return
		}

		feeSatPerVbyte, err := nursery.mempool.GetFeeEstimation()

		if err != nil {
			logger.Error("Could not get LND fee estimation: " + err.Error())
			return
		}

		logger.Info("Using fee of " + strconv.FormatInt(feeSatPerVbyte, 10) + " sat/vbyte for claim transaction")

		claimTransaction, err := boltz.ConstructTransaction(
			[]boltz.OutputDetails{
				{
					LockupTransaction: lockupTransaction,
					Vout:              lockupVout,
					OutputType:        boltz.SegWit,
					RedeemScript:      reverseSwap.RedeemScript,
					PrivateKey:        reverseSwap.PrivateKey,
					Preimage:          reverseSwap.Preimage,
				},
			},
			claimAddress,
			feeSatPerVbyte,
		)

		if err != nil {
			logger.Error("Could not construct claim transaction: " + err.Error())
			return
		}

		claimTransactionId := claimTransaction.TxHash().String()
		logger.Info("Constructed claim transaction: " + claimTransactionId)

		err = nursery.broadcastTransaction(claimTransaction)

		if err != nil {
			logger.Error("Could not finalize claim transaction: " + err.Error())
			return
		}

		err = nursery.database.SetReverseSwapClaimTransactionId(reverseSwap, claimTransactionId)

		if err != nil {
			logger.Error("Could not set claim transaction id in database: " + err.Error())
			return
		}

		if claimTransactionIdChan != nil {
			claimTransactionIdChan <- claimTransactionId
		}
	}

	err := nursery.database.UpdateReverseSwapStatus(reverseSwap, parsedStatus)

	if err != nil {
		logger.Error("Could not update status of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
	}

	if parsedStatus.IsCompletedStatus() {
		err = nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_SUCCESSFUL, "")
	} else if parsedStatus.IsFailedStatus() {
		if reverseSwap.State == boltzrpc.SwapState_PENDING {
			err = nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_SERVER_ERROR, "")
		}
	}

	if err != nil {
		logger.Error("Could not update state of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
	}
}
