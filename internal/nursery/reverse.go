package nursery

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
)

func (nursery *Nursery) sendReverseSwapUpdate(reverseSwap database.ReverseSwap) {
	nursery.sendUpdate(reverseSwap.Id, SwapUpdate{
		ReverseSwap: &reverseSwap,
		IsFinal:     reverseSwap.State != boltzrpc.SwapState_PENDING,
	})
}

func (nursery *Nursery) RegisterReverseSwap(reverseSwap database.ReverseSwap) error {
	if err := nursery.registerSwaps([]string{reverseSwap.Id}); err != nil {
		return err
	}
	nursery.sendReverseSwapUpdate(reverseSwap)
	if err := nursery.payReverseSwap(&reverseSwap); err != nil {
		return err
	}
	return nil
}

func (nursery *Nursery) payReverseSwap(reverseSwap *database.ReverseSwap) error {
	if reverseSwap.ExternalPay {
		return nil
	}
	if nursery.lightning == nil {
		return fmt.Errorf("no lightning node available to pay invoice")
	}
	feeLimitPpm := nursery.maxRoutingFeePpm
	if reverseSwap.RoutingFeeLimitPpm != nil {
		feeLimitPpm = *reverseSwap.RoutingFeeLimitPpm
	}
	feeLimit, err := lightning.CalculateFeeLimit(reverseSwap.Invoice, nursery.network.Btc, feeLimitPpm)
	if err != nil {
		return err
	}

	status, err := nursery.lightning.PaymentStatus(reverseSwap.PreimageHash())
	if err == nil && status.State != lightning.PaymentFailed {
		logger.Debugf("Reverse Swap %s is already being paid", reverseSwap.Id)
		return nil
	}

	nursery.waitGroup.Add(1)
	go func() {
		defer nursery.waitGroup.Done()
		logger.Debugf("Paying invoice of Reverse Swap %s", reverseSwap.Id)
		payment, err := nursery.lightning.PayInvoice(nursery.ctx, reverseSwap.Invoice, feeLimit, 30, reverseSwap.ChanIds)
		if err != nil {
			if nursery.ctx.Err() == nil {
				logger.Errorf("Could not pay invoice %s: %v", reverseSwap.Invoice, err)

				if dbErr := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
					logger.Error("Could not update Reverse Swap state: " + dbErr.Error())
					return
				}
				reverseSwap, err := nursery.database.QueryReverseSwap(reverseSwap.Id)
				if err != nil {
					logger.Error("Could not query Reverse Swap: " + err.Error())
					return
				}
				nursery.sendReverseSwapUpdate(*reverseSwap)
			}
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
		outputArgs: onchain.OutputArgs{
			TransactionId:    reverseSwap.LockupTransactionId,
			Currency:         reverseSwap.Pair.To,
			Address:          lockupAddress,
			BlindingKey:      reverseSwap.BlindingKey,
			ExpectedAmount:   reverseSwap.OnchainAmount,
			RequireConfirmed: !reverseSwap.AcceptZeroConf,
		},
		setTransaction: func(transactionId string, fee uint64) error {
			if err := nursery.database.SetReverseSwapClaimTransactionId(reverseSwap, transactionId, fee); err != nil {
				return fmt.Errorf("could not set claim transaction id in database: %w", err)
			}
			return nil
		},
		setError: func(err error) {
			nursery.handleReverseSwapError(reverseSwap, err)
		},
	}
}

func (nursery *Nursery) handleReverseSwapError(reverseSwap *database.ReverseSwap, err error) {
	if dbErr := nursery.database.UpdateReverseSwapState(reverseSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
		logger.Error("Could not update Reverse Swap state: " + dbErr.Error())
	}
	logger.Errorf("Reverse Swap %s error: %s", reverseSwap.Id, err)
	nursery.sendReverseSwapUpdate(*reverseSwap)
}

// TODO: fail swap after "transaction.failed" event
func (nursery *Nursery) handleReverseSwapStatus(reverseSwap *database.ReverseSwap, event boltz.SwapStatusResponse) {
	parsedStatus := boltz.ParseEvent(event.Status)

	if parsedStatus == reverseSwap.Status {
		logger.Debugf("Status of Reverse Swap %s is %s already", reverseSwap.Id, parsedStatus)
		return
	}

	logger.Infof("Status of Reverse Swap %s changed to: %s", reverseSwap.Id, parsedStatus)

	handleError := func(err string) {
		nursery.handleReverseSwapError(reverseSwap, errors.New(err))
	}

	switch parsedStatus {
	case boltz.TransactionDirect:
		var blindingKey *btcec.PrivateKey
		var wallet onchain.Wallet
		if reverseSwap.Pair.To == boltz.CurrencyLiquid && reverseSwap.WalletId != nil {
			var err error
			wallet, err = nursery.onchain.GetAnyWallet(onchain.WalletChecker{Id: reverseSwap.WalletId, AllowReadonly: true})
			if err != nil {
				handleError("Could not get wallet: " + err.Error())
				return
			}
			if liquid_wallet, ok := wallet.(*liquid_wallet.Wallet); ok {
				blindingKey, err = liquid_wallet.DeriveBlindingKey(reverseSwap.ClaimAddress)
				if err != nil {
					handleError("Could not derive blinding key: " + err.Error())
					return
				}
			} else {
				// gdk handles direct transactions via its own transaction notifier
				break
			}
		}
		directTransaction, err := boltz.NewTxFromHex(reverseSwap.Pair.To, event.Transaction.Hex, blindingKey)
		if err != nil {
			handleError("Could not parse direct transaction: " + err.Error())
			return
		}
		_, value, err := directTransaction.FindVout(nursery.network, reverseSwap.ClaimAddress)
		if err != nil {
			handleError("Could not find vout: " + err.Error())
			return
		}
		_, err = nursery.onchain.BroadcastTransaction(directTransaction)
		if err != nil {
			msg := strings.ToLower(err.Error())
			errors := []string{"transaction already in block chain", "transaction outputs already in utxo set"}
			if !slices.Contains(errors, msg) {
				handleError("Could not broadcast transaction: " + err.Error())
				return
			}
		}

		if wallet != nil {
			if err := wallet.ApplyTransaction(event.Transaction.Hex); err != nil {
				handleError("Could not apply transaction to wallet: " + err.Error())
				return
			}
		}

		nursery.handleReverseSwapDirectPayments(reverseSwap, []*onchain.Output{
			{
				TxId:  event.Transaction.Id,
				Value: value,
			},
		})
		return
	case boltz.TransactionMempool:
		if err := nursery.database.SetReverseSwapPaidAt(reverseSwap, time.Now()); err != nil {
			handleError("Could not set paid at in database: " + err.Error())
			return
		}

		fallthrough

	case boltz.TransactionConfirmed:
		// already broadcasted on transaction.mempool
		if reverseSwap.ClaimTransactionId != "" {
			break
		}

		err := nursery.database.SetReverseSwapLockupTransactionId(reverseSwap, event.Transaction.Id)

		if err != nil {
			handleError("Could not set lockup transaction id in database: " + err.Error())
			return
		}

		if parsedStatus == boltz.TransactionMempool && !reverseSwap.AcceptZeroConf {
			break
		}

		logger.Infof("Constructing claim transaction for Reverse Swap %s", reverseSwap.Id)

		output := nursery.getReverseSwapClaimOutput(reverseSwap)

		if _, err := nursery.createTransaction(reverseSwap.Pair.To, []*Output{output}); err != nil {
			logger.Info("Could not claim: " + err.Error())
			return
		}
	}

	if err := nursery.database.UpdateReverseSwapStatus(reverseSwap, parsedStatus); err != nil {
		handleError("Could not update status of Reverse Swap " + reverseSwap.Id + ": " + err.Error())
		return
	}

	if parsedStatus.IsCompletedStatus() {
		if nursery.lightning != nil && !reverseSwap.ExternalPay {
			status, err := nursery.lightning.PaymentStatus(reverseSwap.PreimageHash())
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

		invoiceAmount := int64(reverseSwap.InvoiceAmount)
		serviceFee := boltz.CalculatePercentage(reverseSwap.ServiceFeePercent, invoiceAmount)
		boltzOnchainFee := invoiceAmount - int64(reverseSwap.OnchainAmount) - serviceFee
		if boltzOnchainFee < 0 {
			logger.Warnf("Reverse Swap %s has negative boltz onchain fee: %dsat", reverseSwap.Id, boltzOnchainFee)
			boltzOnchainFee = 0
		}

		logger.Infof("Reverse Swap service fee: %dsat; boltz onchain fee: %dsat", serviceFee, boltzOnchainFee)

		if err := nursery.database.SetReverseSwapServiceFee(reverseSwap, serviceFee, uint64(boltzOnchainFee)); err != nil {
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

func (nursery *Nursery) chooseDirectOutput(swap *database.ReverseSwap, feeEstimations boltz.FeeEstimations, outputs []*onchain.Output) (*onchain.Output, error) {
	slices.SortFunc(outputs, func(a, b *onchain.Output) int {
		return cmp.Compare(a.Value, b.Value)
	})

	for _, output := range outputs {
		found, err := nursery.database.QueryReverseSwapByClaimTransaction(output.TxId)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("could not query reverse swap by claim transaction: %s", err)
		}
		if found != nil && found.Id != swap.Id {
			logger.Infof("Direct Transaction is claim of %s", found.Id)
			continue
		}

		if output.Value < swap.OnchainAmount {
			if err := boltz.CheckAmounts(
				boltz.ReverseSwap,
				swap.Pair,
				swap.InvoiceAmount,
				output.Value,
				swap.ServiceFeePercent,
				feeEstimations,
				true,
			); err != nil {
				logger.Infof("Output from %s does not pass amount checks: %s", output.TxId, err)
				continue
			}
		}

		return output, nil
	}
	return nil, nil
}

func (nursery *Nursery) handleReverseSwapDirectPayments(swap *database.ReverseSwap, outputs []*onchain.Output) {
	err := nursery.database.RunTx(func(tx *database.Transaction) error {
		logger.Debugf("Found %d direct payments to Reverse Swap %s", len(outputs), swap.Id)
		feeEstimations, err := nursery.GetFeeEstimations()
		if err != nil {
			return fmt.Errorf("get fee estimations: %s", err)
		}

		output, err := nursery.chooseDirectOutput(swap, feeEstimations, outputs)
		if err != nil {
			return err
		}
		if output == nil {
			return nil
		}

		if swap.Pair.To == boltz.CurrencyBtc || output.Value > nursery.maxZeroConfAmount {
			confirmed, err := nursery.onchain.IsTransactionConfirmed(swap.Pair.To, output.TxId, true)
			if !errors.Is(err, errors.ErrUnsupported) {
				if err != nil {
					return fmt.Errorf("is transaction confirmed: %s", err)
				}
				if !confirmed {
					logger.Infof("Rejecting zero conf for direct payment of swap %s: %s", swap.Id, output.TxId)
					if err := tx.UpdateReverseSwapStatus(swap, boltz.TransactionDirectMempool); err != nil {
						return fmt.Errorf("set reverse swap status: %s", err)
					}
					return nil
				}
			}
		}

		if err := tx.SetReverseSwapClaimTransactionId(swap, output.TxId, 0); err != nil {
			return fmt.Errorf("set paid at: %s", err)
		}
		if err := tx.SetReverseSwapPaidAt(swap, time.Now()); err != nil {
			return fmt.Errorf("set paid at: %s", err)
		}
		if err := tx.UpdateReverseSwapState(swap, boltzrpc.SwapState_SUCCESSFUL, ""); err != nil {
			return fmt.Errorf("could not update swap: %s", err)
		}
		if err := tx.UpdateReverseSwapStatus(swap, boltz.TransactionDirect); err != nil {
			return fmt.Errorf("set reverse swap status: %s", err)
		}
		logger.Infof("Reverse Swap %s succeeded through direct payment: %s", swap.Id, swap.ClaimTransactionId)
		return nil
	})
	if err != nil {
		nursery.handleReverseSwapError(swap, fmt.Errorf("direct payment: %w", err))
	} else {
		nursery.sendReverseSwapUpdate(*swap)
	}
}

func (nursery *Nursery) checkExternalReverseSwaps(currency boltz.Currency) error {
	reverseSwaps, err := nursery.database.QueryReverseSwaps(database.SwapQuery{
		To:     &currency,
		States: []boltzrpc.SwapState{boltzrpc.SwapState_PENDING, boltzrpc.SwapState_ERROR},
	})
	if err != nil {
		return err
	}
	for _, swap := range reverseSwaps {
		if !swap.ExternalPay || swap.WalletId == nil || swap.ClaimAddress == "" {
			continue
		}
		swapWallet, err := nursery.onchain.GetAnyWallet(onchain.WalletChecker{Id: swap.WalletId, AllowReadonly: true})
		if err != nil {
			return err
		}
		outputs, err := swapWallet.GetOutputs(swap.ClaimAddress)
		if err != nil {
			if errors.Is(err, errors.ErrUnsupported) && currency == boltz.CurrencyBtc {
				outputs, err = nursery.onchain.GetUnspentOutputs(currency, swap.ClaimAddress)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if len(outputs) > 0 {
			nursery.handleReverseSwapDirectPayments(swap, outputs)
		}
	}
	return nil
}
