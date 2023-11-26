package nursery

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func sendSwapUpdate(swap database.Swap) {
	isFinal := swap.State == boltzrpc.SwapState_SUCCESSFUL || swap.State == boltzrpc.SwapState_REFUNDED
	if swap.LockupTransactionId == "" && swap.State != boltzrpc.SwapState_PENDING {
		isFinal = false
	}

	sendUpdate(swap.Id, SwapUpdate{
		Swap:    &swap,
		IsFinal: isFinal,
	})
}

// TODO: abstract interactions with chain (querying and broadcasting transactions) into interface to be able to switch between Boltz API and bitcoin core

func (nursery *Nursery) startBlockListener(pair boltz.Pair) error {
	blockNotifier := make(chan *onchain.BlockEpoch)
	err := nursery.registerBlockListener(pair, blockNotifier)
	if err != nil {
		return err
	}

	go func() {
		for newBlock := range blockNotifier {
			swapsToRefund, err := nursery.database.QueryRefundableSwaps(newBlock.Height, pair)

			if err != nil {
				logger.Error("Could not query refundable Swaps: " + err.Error())
				continue
			}

			if len(swapsToRefund) > 0 {
				logger.Info("Found " + strconv.Itoa(len(swapsToRefund)) + " Swaps to refund at height " + strconv.FormatUint(uint64(newBlock.Height), 10))

				if err != nil {
					logger.Error("Could not get new address: " + err.Error())
					continue
				}

				var refundedSwaps []database.Swap
				var refundOutputs []boltz.OutputDetails
				var refundAddress string

				for _, swapToRefund := range swapsToRefund {
					refundOutput := nursery.getRefundOutput(&swapToRefund)

					if refundOutput != nil {
						if swapToRefund.RefundAddress != "" {
							// we process all swaps that have an explicit refund address isolated
							refundedSwaps = []database.Swap{swapToRefund}
							refundOutputs = []boltz.OutputDetails{*refundOutput}
							refundAddress = swapToRefund.RefundAddress
							break
						}
						refundedSwaps = append(refundedSwaps, swapToRefund)
						refundOutputs = append(refundOutputs, *refundOutput)
					}
				}

				if refundAddress == "" {
					wallet, err := nursery.onchain.GetWallet(pair)
					if err != nil {
						message := "%d Swaps can not be refunded because they got no refund address and no wallet for pair %s is available! Set up a wallet to refund"
						logger.Warnf(message, len(refundedSwaps), pair)
						continue
					}
					refundAddress, err = wallet.NewAddress()
					if err != nil {
						logger.Warnf("%d Swaps can not be refunded because they got no refund address and wallet failed to generate address: %v", len(refundedSwaps), err)
						continue
					}
				}

				if len(refundOutputs) == 0 {
					logger.Info("Did not find any outputs to refund")
					continue
				}

				// TODO: make sure that all refund swaps are from the same pair
				pair := refundedSwaps[0].PairId
				feeSatPerVbyte, err := nursery.getFeeEstimation(pair)

				if err != nil {
					logger.Error("Could not get fee estimation: " + err.Error())
					continue
				}

				logger.Info(fmt.Sprintf("Using fee of %v sat/vbyte for refund transaction", feeSatPerVbyte))

				refundTransaction, totalRefundFee, err := boltz.ConstructTransaction(
					pair,
					nursery.network,
					refundOutputs,
					refundAddress,
					feeSatPerVbyte,
				)

				if err != nil {
					logger.Error("Could not construct refund transaction: " + err.Error())
					continue
				}

				refundTransactionId := refundTransaction.Hash()
				logger.Infof("Constructed refund transaction for %d swaps: %s", len(refundOutputs), refundTransactionId)

				// TODO: right pair?
				err = nursery.broadcastTransaction(refundTransaction, utils.CurrencyFromPair(pair))

				if err != nil {
					logger.Error("Could not finalize refund transaction: " + err.Error())
					continue
				}

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
					}

					sendSwapUpdate(refundedSwap)
				}
			}
		}
	}()

	return nil
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

	lockupTransaction, err := boltz.NewTxFromHex(swapTransactionResponse.TransactionHex, swap.BlindingKey)

	if err != nil {
		logger.Error("Could not parse lockup transaction from Boltz: " + err.Error())
		return nil
	}

	logger.Info("Got lockup transaction of Swap " + swap.Id + " from Boltz: " + lockupTransaction.Hash())

	lockupVout, _, err := lockupTransaction.FindVout(nursery.network, swap.Address)

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

func (nursery *Nursery) recoverSwaps() error {
	logger.Info("Recovering pending Swaps")

	swaps, err := nursery.database.QueryPendingSwaps()

	if err != nil {
		return err
	}

	for _, swap := range swaps {
		logger.Info("Recovering Swap" + " " + swap.Id + " at state: " + swap.Status.String())

		// TODO: handle race condition when status is updated between the POST request and the time the streaming starts
		status, err := nursery.boltz.SwapStatus(swap.Id)

		if err != nil {
			logger.Warn("Boltz could not find Swap " + swap.Id + ": " + err.Error())
			continue
		}

		if status.Status != swap.Status.String() {
			logger.Info("Swap " + swap.Id + " status changed to: " + status.Status)
			nursery.handleSwapStatus(&swap, *status)

			if swap.State == boltzrpc.SwapState_PENDING {
				nursery.RegisterSwap(swap)
			}

			continue
		}

		logger.Info("Swap " + swap.Id + " status did not change")
		nursery.RegisterSwap(swap)
	}
	return nil
}

func (nursery *Nursery) RegisterSwap(swap database.Swap) {
	logger.Info("Listening to events of Swap " + swap.Id)

	go func() {
		listener, remove := newListener(swap.Id)
		defer remove()

		sendSwapUpdate(swap)

		eventStream := make(chan *boltz.SwapStatusResponse)
		nursery.streamSwapStatus(swap.Id, "Swap", eventStream, listener.stop)

		for event := range eventStream {
			logger.Info("Swap " + swap.Id + " status update: " + event.Status)
			nursery.handleSwapStatus(&swap, *event)
		}
	}()
}

func (nursery *Nursery) handleSwapStatus(swap *database.Swap, status boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(status.Status)

	if parsedStatus == swap.Status {
		logger.Info("Status of Swap " + swap.Id + " is " + parsedStatus.String() + " already")
		return
	}

	handleError := func(err string) {
		if dbErr := nursery.database.UpdateSwapState(swap, boltzrpc.SwapState_ERROR, err); dbErr != nil {
			logger.Error("Could not update Reverse Swap state: " + dbErr.Error())
		}
		logger.Error(err)
		sendSwapUpdate(*swap)
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
			lockupTransaction, err := boltz.NewTxFromHex(swapTransactionResponse.TransactionHex, swap.BlindingKey)
			if err != nil {
				handleError("Could not decode lockup transaction: " + err.Error())
				return
			}

			logger.Info("Got lockup transaction of Swap " + swap.Id + " from Boltz: " + lockupTransaction.Hash())

			if err := nursery.database.SetSwapLockupTransactionId(swap, lockupTransaction.Hash()); err != nil {
				handleError("Could not set lockup transaction in database: " + err.Error())
				return
			}

			if swap.AutoSend {
				fee, err := nursery.onchain.GetTransactionFee(swap.PairId, swap.LockupTransactionId)
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
		// Connect to the LND node of Boltz to allow for channels to be opened and to gossip our channels
		// to increase the chances that the provided invoice can be paid
		_, _ = utils.ConnectBoltz(nursery.lightning, nursery.boltz)

		// Set the invoice of Swaps that were created with only a preimage hash
		if swap.Invoice != "" {
			break
		}

		swapRates, err := nursery.boltz.SwapRates(boltz.SwapRatesRequest{
			Id: swap.Id,
		})
		if err != nil {
			handleError("Could not query Swap rates of Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Info("Found output for Swap " + swap.Id + " of " + strconv.FormatUint(swapRates.OnchainAmount, 10) + " satoshis")

		if err := nursery.database.SetSwapExpectedAmount(swap, swapRates.OnchainAmount); err != nil {
			logger.Error("Could not set expected amount in database: " + err.Error())
			return
		}

		blockHeight, err := nursery.onchain.GetBlockHeight(swap.PairId)

		if err != nil {
			handleError("Could not get block height: " + err.Error())
			return
		}

		invoice, err := nursery.lightning.CreateInvoice(
			int64(swapRates.SubmarineSwap.InvoiceAmount),
			swap.Preimage,
			utils.CalculateInvoiceExpiry(swap.TimoutBlockHeight-blockHeight, utils.GetBlockTime(swap.PairId)),
			utils.GetSwapMemo(utils.CurrencyFromPair(swap.PairId)),
		)

		if err != nil {
			handleError("Could not get new invoice for Swap " + swap.Id + ": " + err.Error())
			return
		}

		logger.Info("Generated new invoice for Swap " + swap.Id + " for " + strconv.FormatUint(swapRates.SubmarineSwap.InvoiceAmount, 10) + " satoshis")

		_, err = nursery.boltz.SetInvoice(boltz.SetInvoiceRequest{
			Id:      swap.Id,
			Invoice: invoice.PaymentRequest,
		})

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
		// Verify that the invoice was actually paid
		decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.network.Btc)

		if err != nil {
			handleError("Could not decode invoice: " + err.Error())
			return
		}

		paid, err := nursery.lightning.CheckInvoicePaid(decodedInvoice.PaymentHash[:])

		if err != nil {
			handleError("Could not get invoice information from LND: " + err.Error())
			return
		}

		if !paid {
			logger.Warn("Swap " + swap.Id + " was not actually settled. Refunding at block " + strconv.FormatUint(uint64(swap.TimoutBlockHeight), 10))
			return
		}

		logger.Info("Swap " + swap.Id + " succeeded")
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
	}

	sendSwapUpdate(*swap)
}
