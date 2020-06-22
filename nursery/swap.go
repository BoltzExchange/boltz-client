package nursery

import (
	"encoding/hex"
	"encoding/json"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/btcsuite/btcutil"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/r3labs/sse"
	"strconv"
	"strings"
)

// TODO: abstract interactions with chain (querying and broadcasting transactions) into interface to be able to switch between Boltz API and bitcoin core

func (nursery *Nursery) startBlockListener(blockNotifier chan *chainrpc.BlockEpoch) {
	go func() {
		for {
			newBlock := <-blockNotifier

			swapsToRefund, err := nursery.database.QueryRefundableSwaps(newBlock.Height)

			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swapsToRefund) > 0 {
				logger.Info("Found " + strconv.Itoa(len(swapsToRefund)) + " Swaps to refund at height " + strconv.FormatUint(uint64(newBlock.Height), 10))

				addressString, err := nursery.lnd.NewAddress()

				if err != nil {
					logger.Error("Could not get new address from LND: " + err.Error())
					continue
				}

				address, err := btcutil.DecodeAddress(addressString, nursery.chainParams)

				if err != nil {
					logger.Error("Could not decode destination address from LND: " + err.Error())
					continue
				}

				var refundOutputs []boltz.OutputDetails

				for _, swapToRefund := range swapsToRefund {
					eventListenersLock.RLock()
					stopListening, hasListener := eventListeners[swapToRefund.Id]
					eventListenersLock.RUnlock()

					if hasListener {
						stopListening <- true

						eventListenersLock.Lock()
						delete(eventListeners, swapToRefund.Id)
						eventListenersLock.Unlock()
					}

					refundOutput := nursery.getRefundOutput(swapToRefund)

					if refundOutput != nil {
						refundOutputs = append(refundOutputs, *refundOutput)
					}
				}

				if len(refundOutputs) == 0 {
					logger.Info("Did not find any outputs to refund")
					continue
				}

				feeSatPerVbyte, err := nursery.getFeeEstimation()

				if err != nil {
					logger.Error("Could not get LND fee estimation: " + err.Error())
					continue
				}

				logger.Info("Using fee of " + strconv.FormatInt(feeSatPerVbyte, 10) + " sat/vbyte for refund transaction")

				refundTransaction, err := boltz.ConstructTransaction(
					refundOutputs,
					address,
					feeSatPerVbyte,
				)

				if err != nil {
					logger.Error("Could not construct refund transaction: " + err.Error())
					continue
				}

				logger.Info("Constructed refund transaction: " + refundTransaction.TxHash().String())

				err = nursery.broadcastTransaction(refundTransaction)

				if err != nil {
					logger.Error("Could not finalize refund transaction: " + err.Error())
				}
			}
		}
	}()
}

// TODO: add channel creation wording
func (nursery *Nursery) getRefundOutput(swap database.Swap) *boltz.OutputDetails {
	swapTransactionResponse, err := nursery.boltz.GetSwapTransaction(swap.Id)

	if err != nil {
		logger.Error("Could not get lockup transaction from Boltz: " + err.Error())
		nursery.handleSwapStatus(&swap, nil, boltz.SwapStatusResponse{
			Status: boltz.SwapAbandoned.String(),
		})

		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(swapTransactionResponse.TransactionHex)

	if err != nil {
		logger.Error("Could not decode lockup transaction from Boltz: " + err.Error())
		return nil
	}

	lockupTransaction, err := btcutil.NewTxFromBytes(lockupTransactionRaw)

	if err != nil {
		logger.Error("Could not parse lockup transaction from Boltz: " + err.Error())
		return nil
	}

	logger.Info("Got lockup transaction of Swap " + swap.Id + " from Boltz: " + lockupTransaction.Hash().String())
	lockupVout, err := nursery.findLockupVout(swap.Address, lockupTransaction.MsgTx().TxOut)

	if err != nil {
		logger.Error("Could not find lockup vout of Swap " + swap.Id)
		return nil
	}

	// TODO: do this after refund is successful
	nursery.handleSwapStatus(&swap, nil, boltz.SwapStatusResponse{
		Status: boltz.SwapRefunded.String(),
	})

	return &boltz.OutputDetails{
		LockupTransaction:  lockupTransaction,
		Vout:               lockupVout,
		OutputType:         boltz.Compatibility,
		RedeemScript:       swap.RedeemScript,
		PrivateKey:         swap.PrivateKey,
		Preimage:           []byte{},
		TimeoutBlockHeight: uint32(swap.TimoutBlockHeight),
	}
}

func (nursery *Nursery) recoverSwaps(blockNotifier chan *chainrpc.BlockEpoch) error {
	logger.Info("Recovering pending Swaps and Channel Creations")

	swaps, err := nursery.database.QueryPendingSwaps()

	if err != nil {
		return err
	}

	for _, swap := range swaps {
		channelCreation, err := nursery.database.QueryChannelCreation(swap.Id)
		isChannelCreation := err == nil

		swapType := getSwapType(isChannelCreation)

		logger.Info("Recovering " + swapType + " " + swap.Id + " at state: " + swap.Status.String())

		// TODO: handle race condition when status is updated between the POST request and the time the streaming starts
		status, err := nursery.boltz.SwapStatus(swap.Id)

		if err != nil {
			return err
		}

		if status.Status != swap.Status.String() {
			logger.Info(swapType + " " + swap.Id + " status changed to: " + status.Status)
			nursery.handleSwapStatus(&swap, channelCreation, *status)

			isCompleted := false

			for _, completedStatus := range boltz.CompletedStatus {
				if swap.Status.String() == completedStatus {
					isCompleted = true
					break
				}
			}

			if !isCompleted {
				nursery.RegisterSwap(&swap, channelCreation)
			}

			continue
		}

		logger.Info(swapType + " " + swap.Id + " status did not change")
		nursery.RegisterSwap(&swap, channelCreation)
	}

	nursery.startBlockListener(blockNotifier)

	return nil
}

func (nursery *Nursery) RegisterSwap(swap *database.Swap, channelCreation *database.ChannelCreation) {
	isChannelCreation := channelCreation != nil
	swapType := getSwapType(isChannelCreation)

	logger.Info("Listening to events of " + swapType + " " + swap.Id)

	var stopInvoiceSubscription chan bool

	if isChannelCreation {
		var err error
		stopInvoiceSubscription, err = nursery.subscribeChannelCreationInvoice(*swap, channelCreation)

		if err != nil {
			logger.Error("Could not subscribe to invoice events: " + err.Error())
			return
		}
	}

	go func() {
		stopListening := make(chan bool)

		eventListenersLock.Lock()
		eventListeners[swap.Id] = stopListening
		eventListenersLock.Unlock()

		eventStream := make(chan *sse.Event)

		// TODO: handle disconnections gracefully
		go func() {
			_, err := nursery.boltz.StreamSwapStatus(swap.Id, eventStream)

			if err != nil {
				logger.Error("Could not listen to events of " + swapType + " " + swap.Id + ": " + err.Error())

				eventListenersLock.Lock()
				delete(eventListeners, swap.Id)
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
					logger.Info(swapType + " " + swap.Id + " status update: " + response.Status)
					nursery.handleSwapStatus(swap, channelCreation, response)

					// The event listening can stop after the Swap has succeeded
					if swap.Status == boltz.TransactionClaimed {
						return
					}
				} else {
					logger.Error("Could not parse update event of " + swapType + " " + swap.Id + ": " + err.Error())
				}

				break

			case <-stopListening:
				if stopInvoiceSubscription != nil {
					stopInvoiceSubscription <- true
				}

				logger.Info("Stopping event listener of " + swapType + " " + swap.Id)
				return
			}
		}
	}()
}

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, channelCreation *database.ChannelCreation, status boltz.SwapStatusResponse) {
	isChannelCreation := channelCreation != nil
	swapType := getSwapType(isChannelCreation)

	parsedStatus := boltz.ParseEvent(status.Status)

	switch parsedStatus {
	case boltz.TransactionMempool:
		fallthrough

	case boltz.TransactionConfirmed:
		// Set the invoice of Swaps that were created with only a preimage hash
		if swap.Invoice != "" {
			break
		}

		swapRates, err := nursery.boltz.SwapRates(boltz.SwapRatesRequest{
			Id: swap.Id,
		})

		if err != nil {
			logger.Error("Could not query Swap rates of Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Info("Found output for Swap " + swap.Id + " of " + strconv.Itoa(swapRates.OnchainAmount) + " satoshis")

		invoice, err := nursery.lnd.AddInvoice(int64(swapRates.SubmarineSwap.InvoiceAmount), swap.Preimage, utils.GetSwapMemo(nursery.symbol))

		if err != nil {
			logger.Error("Could not get new invoice for Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Info("Generated new invoice for Swap " + swap.Id + " for " + strconv.Itoa(swapRates.SubmarineSwap.InvoiceAmount) + " satoshis")

		_, err = nursery.boltz.SetInvoice(boltz.SetInvoiceRequest{
			Id:      swap.Id,
			Invoice: invoice.PaymentRequest,
		})

		if err != nil {
			logger.Error("Could not set invoice of Swap: " + err.Error())
			return
		}

		err = nursery.database.SetSwapInvoice(swap, invoice.PaymentRequest)

		if err != nil {
			logger.Error("Could not set invoice of Swap in database: " + err.Error())
			return
		}

	case boltz.ChannelCreated:
		// Verify the capacity of the channel
		pendingChannels, err := nursery.lnd.PendingChannels()

		if err != nil {
			logger.Error("Could not get pending channels: " + err.Error())
			return
		}

		for _, pendingChannel := range pendingChannels.PendingOpenChannels {
			id, vout, err := parseChannelPoint(pendingChannel.Channel.ChannelPoint)

			if err != nil {
				logger.Error("Could not parse funding channel point: " + err.Error())
				return
			}

			if pendingChannel.Channel.RemoteNodePub == nursery.boltzPubKey &&
				id == status.Channel.FundingTransactionId &&
				vout == status.Channel.FundingTransactionVout {

				decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.chainParams)

				if err != nil {
					logger.Error("Could not decode invoice: " + err.Error())
					return
				}

				invoiceAmount := decodedInvoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi)
				expectedCapacity := calculateChannelCreationCapacity(invoiceAmount, channelCreation.InboundLiquidity)

				if pendingChannel.Channel.Capacity < expectedCapacity {
					logger.Error("Channel capacity of " + swapType + " " + swap.Id + " " +
						strconv.FormatInt(pendingChannel.Channel.Capacity, 10) +
						" is less than than expected " + strconv.FormatInt(expectedCapacity, 10) + " satoshis")
					return
				}

				// TODO: how to verify public / private channel?

				err = nursery.database.SetChannelFunding(channelCreation, id, vout)

				if err != nil {
					logger.Error("Could not update state of " + swapType + " " + swap.Id + ": " + err.Error())
					return
				}

				logger.Info("Channel for " + swapType + " " + swap.Id + " was opened: " + pendingChannel.Channel.ChannelPoint)

				break
			}
		}

	case boltz.TransactionClaimed:
		// Verify that the invoice was actually paid
		decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.chainParams)

		if err != nil {
			logger.Error("Could not decode invoice: " + err.Error())
			return
		}

		invoiceInfo, err := nursery.lnd.LookupInvoice(decodedInvoice.PaymentHash[:])

		if err != nil {
			logger.Error("Could not get invoice information from LND: " + err.Error())
			return
		}

		if invoiceInfo.State != lnrpc.Invoice_SETTLED {
			logger.Warning(swapType + " " + swap.Id + " was not actually settled. Refunding at block " + strconv.Itoa(swap.TimoutBlockHeight))
			return
		}

		logger.Info(swapType + " " + swap.Id + " succeeded")
	}

	err := nursery.database.UpdateSwapStatus(swap, parsedStatus)
	if err != nil {
		logger.Error("Could not update status of " + swapType + " " + swap.Id + ": " + err.Error())
	}
}

func parseChannelPoint(channelPoint string) (string, int, error) {
	split := strings.Split(channelPoint, ":")
	vout, err := strconv.Atoi(split[1])

	if err != nil {
		return "", 0, err
	}

	return split[0], vout, nil
}

func getSwapType(isChannelCreation bool) string {
	swapType := "Swap"

	if isChannelCreation {
		swapType = "Channel Creation"
	}

	return swapType
}
