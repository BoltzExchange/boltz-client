package nursery

import (
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/zpay32"
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

				addressString, err := nursery.lightning.NewAddress()

				if err != nil {
					logger.Error("Could not get new address from LND: " + err.Error())
					continue
				}

				address, err := btcutil.DecodeAddress(addressString, nursery.chainParams)

				if err != nil {
					logger.Error("Could not decode destination address from LND: " + err.Error())
					continue
				}

				var refundedSwaps []database.Swap
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

					refundOutput := nursery.getRefundOutput(&swapToRefund)

					if refundOutput != nil {
						refundedSwaps = append(refundedSwaps, swapToRefund)
						refundOutputs = append(refundOutputs, *refundOutput)
					}
				}

				if len(refundOutputs) == 0 {
					logger.Info("Did not find any outputs to refund")
					continue
				}

				feeSatPerVbyte, err := nursery.mempool.GetFeeEstimation()

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

				refundTransactionId := refundTransaction.TxHash().String()
				logger.Info("Constructed refund transaction: " + refundTransactionId)

				// TODO: right pair?
				err = nursery.broadcastTransaction(refundTransaction, "BTC")

				if err != nil {
					logger.Error("Could not finalize refund transaction: " + err.Error())
					continue
				}

				for _, refundedSwap := range refundedSwaps {
					err = nursery.database.SetSwapRefundTransactionId(&refundedSwap, refundTransactionId)

					if err != nil {
						logger.Error("Could not set refund transaction id in database: " + err.Error())
					}
				}
			}
		}
	}()
}

func (nursery *Nursery) getRefundOutput(swap *database.Swap) *boltz.OutputDetails {
	swapTransactionResponse, err := nursery.boltz.GetSwapTransaction(swap.Id)

	if err != nil {
		logger.Error("Could not get lockup transaction from Boltz: " + err.Error())
		err := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_ABANDONED, "")

		if err != nil {
			logger.Error("Could not update state of Swap " + swap.Id + ": " + err.Error())
		}

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

	err = nursery.database.SetSwapLockupTransactionId(swap, lockupTransaction.Hash().String())

	if err != nil {
		logger.Error("Could not set lockup transaction id in database: " + err.Error())
		return nil
	}

	lockupVout, err := nursery.findLockupVout(swap.Address, lockupTransaction.MsgTx().TxOut)

	if err != nil {
		logger.Error("Could not find lockup vout of Swap " + swap.Id)
		return nil
	}

	return &boltz.OutputDetails{
		LockupTransaction:  lockupTransaction,
		Vout:               lockupVout,
		OutputType:         boltz.Compatibility,
		RedeemScript:       swap.RedeemScript,
		PrivateKey:         swap.PrivateKey,
		Preimage:           []byte{},
		TimeoutBlockHeight: swap.TimoutBlockHeight,
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
			logger.Warning("Boltz could not find Swap " + swap.Id + ": " + err.Error())
			continue
		}

		if status.Status != swap.Status.String() {
			logger.Info(swapType + " " + swap.Id + " status changed to: " + status.Status)
			nursery.handleSwapStatus(&swap, channelCreation, *status)

			if swap.State == boltzrpc.SwapState_PENDING {
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
		stopInvoiceSubscription = nursery.subscribeChannelCreationInvoice(*swap, channelCreation)
	}

	go func() {
		stopListening := make(chan bool)
		stopHandler := make(chan bool)

		eventListenersLock.Lock()
		eventListeners[swap.Id] = stopListening
		eventListenersLock.Unlock()

		eventStream := make(chan *boltz.SwapStatusResponse)

		nursery.streamSwapStatus(swap.Id, swapType, eventStream, stopListening, stopHandler)

		for {
			select {
			case event := <-eventStream:
				logger.Info(swapType + " " + swap.Id + " status update: " + event.Status)
				nursery.handleSwapStatus(swap, channelCreation, *event)

				// The event listening can stop after the Swap has succeeded
				if swap.Status == boltz.TransactionClaimed {
					stopListening <- true
				}

				break

			case <-stopHandler:
				if stopInvoiceSubscription != nil {
					stopInvoiceSubscription <- true
				}

				return
			}
		}
	}()
}

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, channelCreation *database.ChannelCreation, status boltz.SwapStatusResponse) {
	isChannelCreation := channelCreation != nil
	swapType := getSwapType(isChannelCreation)

	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Info("Status of " + swapType + " " + swap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	switch parsedStatus {
	case boltz.TransactionMempool:
		fallthrough

	case boltz.TransactionConfirmed:
		// Connect to the LND node of Boltz to allow for channels to be opened and to gossip our channels
		// to increase the chances that the provided invoice can be paid
		_, _ = utils.ConnectBoltzLnd(nursery.lnd, nursery.boltz)

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

		logger.Info("Found output for Swap " + swap.Id + " of " + strconv.FormatUint(swapRates.OnchainAmount, 10) + " satoshis")

		info, err := nursery.lightning.GetInfo()

		if err != nil {
			logger.Error("Could not get LND info: " + err.Error())
			return
		}

		invoice, err := nursery.lightning.CreateInvoice(
			int64(swapRates.SubmarineSwap.InvoiceAmount),
			swap.Preimage,
			utils.CalculateInvoiceExpiry(swap.TimoutBlockHeight-info.BlockHeight, utils.GetBlockTime(swap.PairId)),
			utils.GetSwapMemo(swap.PairId),
		)

		if err != nil {
			logger.Error("Could not get new invoice for Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Info("Generated new invoice for Swap " + swap.Id + " for " + strconv.FormatUint(swapRates.SubmarineSwap.InvoiceAmount, 10) + " satoshis")

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
		if !isChannelCreation {
			break
		}

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
			logger.Warning(swapType + " " + swap.Id + " was not actually settled. Refunding at block " + strconv.FormatUint(uint64(swap.TimoutBlockHeight), 10))
			return
		}

		logger.Info(swapType + " " + swap.Id + " succeeded")
	}

	err := nursery.database.UpdateSwapStatus(swap, parsedStatus)

	if err != nil {
		logger.Error("Could not update status of " + swapType + " " + swap.Id + ": " + err.Error())
	}

	if parsedStatus.IsCompletedStatus() {
		err = nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, "")
	} else if parsedStatus.IsFailedStatus() {
		if swap.State == boltzrpc.SwapState_PENDING {
			err = nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_SERVER_ERROR, "")
		}
	}

	if err != nil {
		logger.Error("Could not update state of " + swapType + " " + swap.Id + ": " + err.Error())
	}
}

func parseChannelPoint(channelPoint string) (string, uint32, error) {
	split := strings.Split(channelPoint, ":")
	vout, err := strconv.Atoi(split[1])

	if err != nil {
		return "", 0, err
	}

	return split[0], uint32(vout), nil
}

func getSwapType(isChannelCreation bool) string {
	swapType := "Swap"

	if isChannelCreation {
		swapType = "Channel Creation"
	}

	return swapType
}
