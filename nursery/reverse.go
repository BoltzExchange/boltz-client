package nursery

import (
	"encoding/hex"
	"encoding/json"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/btcsuite/btcutil"
	"github.com/google/logger"
	"github.com/r3labs/sse"
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
			return err
		}

		if status.Status != reverseSwap.Status.String() {
			logger.Info("Swap " + reverseSwap.Id + " status changed to: " + status.Status)
			nursery.handleReverseSwapStatus(&reverseSwap, *status, nil)

			isCompleted := false

			for _, completedStatus := range boltz.CompletedStatus {
				if reverseSwap.Status.String() == completedStatus {
					isCompleted = true
					break
				}
			}

			if !isCompleted {
				nursery.RegisterReverseSwap(reverseSwap)
			}

			continue
		}

		logger.Info("Reverse Swap " + reverseSwap.Id + " status did not change")
		nursery.RegisterReverseSwap(reverseSwap)
	}

	return nil
}

func (nursery *Nursery) RegisterReverseSwap(reverseSwap database.ReverseSwap) chan string {
	logger.Info("Listening to events of Reverse Swap " + reverseSwap.Id)

	claimTransactionIdChan := make(chan string)

	go func() {
		stopListening := make(chan bool)

		eventListenersLock.Lock()
		eventListeners[reverseSwap.Id] = stopListening
		eventListenersLock.Unlock()

		eventStream := make(chan *sse.Event)

		// TODO: handle disconnections gracefully
		go func() {
			_, err := nursery.boltz.StreamSwapStatus(reverseSwap.Id, eventStream)

			if err != nil {
				logger.Error("Could not listen to events of Reverse Swap " + reverseSwap.Id + ": " + err.Error())

				eventListenersLock.Lock()
				delete(eventListeners, reverseSwap.Id)
				eventListenersLock.Unlock()

				stopListening <- true
				return
			}
		}()

		for {
			select {
			case event := <-eventStream:
				var response boltz.SwapStatusResponse
				err := json.Unmarshal(event.Data, &response)

				if err == nil {
					logger.Info("Reverse Swap " + reverseSwap.Id + " status update: " + response.Status)
					nursery.handleReverseSwapStatus(&reverseSwap, response, claimTransactionIdChan)

					// The event listening can stop after the Reverse Swap has succeeded
					if reverseSwap.Status == boltz.InvoiceSettled {
						return
					}
				} else {
					logger.Error("Could not parse update event of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
				}

				break

			case <-stopListening:
				logger.Info("Stopping event listener of Reverse Swap: " + reverseSwap.Id)
				return
			}
		}
	}()

	return claimTransactionIdChan
}

// TODO: fail swap after "transaction.failed" event
func (nursery *Nursery) handleReverseSwapStatus(reverseSwap *database.ReverseSwap, event boltz.SwapStatusResponse, claimTransactionIdChan chan string) {
	parsedStatus := boltz.ParseEvent(event.Status)

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

		feeSatPerVbyte, err := nursery.getFeeEstimation()

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

		logger.Info("Constructed claim transaction: " + claimTransaction.TxHash().String())

		err = nursery.broadcastTransaction(claimTransaction)

		if err != nil {
			logger.Error("Could not finalize claim transaction: " + err.Error())
		}

		if claimTransactionIdChan != nil {
			claimTransactionIdChan <- claimTransaction.TxHash().String()
		}
	}

	err := nursery.database.UpdateReverseSwapStatus(reverseSwap, parsedStatus)
	if err != nil {
		logger.Error("Could not update status of Swap " + reverseSwap.Id + ": " + err.Error())
	}
}
