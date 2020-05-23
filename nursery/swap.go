package nursery

import (
	"encoding/json"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/r3labs/sse"
	"strconv"
)

func (nursery *Nursery) recoverSwaps() error {
	logger.Info("Recovering pending Swaps")

	swaps, err := nursery.database.QueryPendingSwaps()

	if err != nil {
		return err
	}

	for _, swap := range swaps {
		logger.Info("Recovering Swap " + swap.Id + " at state: " + swap.Status.String())

		// TODO: handle race condition when status is updated between the POST request and the time the streaming starts
		status, err := nursery.boltz.SwapStatus(swap.Id)

		if err != nil {
			return err
		}

		if status.Status != swap.Status.String() {
			logger.Info("Swap " + swap.Id + " status changed to: " + status.Status)
			nursery.handleSwapStatus(&swap, "transaction.claimed")

			go func(swap database.Swap) {
				nursery.RegisterSwap(swap)
			}(swap)
			continue
		}

		logger.Info("Swap " + swap.Id + " status did not change")

		go func(swap database.Swap) {
			nursery.RegisterSwap(swap)
		}(swap)
	}

	return nil
}

func (nursery *Nursery) RegisterSwap(swap database.Swap) {
	logger.Info("Listening to events of Swap " + swap.Id)

	eventStream := make(chan *sse.Event)
	// TODO: handle disconnections gracefully
	client, err := nursery.boltz.StreamSwapStatus(swap.Id, eventStream)

	if err != nil {
		logger.Error("Could not listen to events of Swap " + swap.Id + ": " + err.Error())
		return
	}

	for {
		event := <-eventStream

		var response boltz.SwapStatusResponse
		err := json.Unmarshal(event.Data, &response)

		if err == nil {
			logger.Info("Swap " + swap.Id + " status update: " + response.Status)
			nursery.handleSwapStatus(&swap, response.Status)

			// The event listening can stop after the Swap has succeeded
			if swap.Status == boltz.TransactionClaimed {
				client.Unsubscribe(eventStream)
				break
			}
		} else {
			logger.Error("Could not parse update event of Swap " + swap.Id + ": " + err.Error())
		}
	}
}

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, status string) {
	parsedStatus := boltz.ParseEvent(status)

	switch parsedStatus {
	case boltz.TransactionClaimed:
		decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.chainParams)

		if err != nil {
			logger.Warning("Could not decode invoice: " + err.Error())
			return
		}

		invoiceInfo, err := nursery.lnd.LookupInvoice(decodedInvoice.PaymentHash[:])

		if err != nil {
			logger.Warning("Could not get invoice information from LND: " + err.Error())
			return
		}

		if invoiceInfo.State != lnrpc.Invoice_SETTLED {
			logger.Warning("Swap " + swap.Id + " was not actually settled. Refunding at block " + strconv.Itoa(swap.TimoutBlockHeight))
			return
		}

		logger.Info("Swap " + swap.Id + " succeeded")
	}

	err := nursery.database.UpdateSwapStatus(swap, parsedStatus)
	if err != nil {
		logger.Error("Could not update status of Swap " + swap.Id + ": " + err.Error())
	}
}
