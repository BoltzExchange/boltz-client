package nursery

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/BoltzExchange/boltz-client/onchain/wallet"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/BoltzExchange/boltz-client/utils"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
)

type Nursery struct {
	ctx    context.Context
	cancel func()

	network *boltz.Network

	lightning lightning.LightningNode

	onchain  *onchain.Onchain
	boltz    *boltz.Api
	boltzWs  *boltz.Websocket
	database *database.Database

	eventListeners     map[string]swapListener
	eventListenersLock sync.RWMutex
	globalListener     swapListener
	waitGroup          sync.WaitGroup

	MaxZeroConfAmount uint64

	BtcBlocks    *utils.ChannelForwarder[*onchain.BlockEpoch]
	LiquidBlocks *utils.ChannelForwarder[*onchain.BlockEpoch]
}

type SwapUpdate struct {
	Swap        *database.Swap
	ReverseSwap *database.ReverseSwap
	ChainSwap   *database.ChainSwap
	IsFinal     bool
}

type swapListener = *utils.ChannelForwarder[SwapUpdate]

func (nursery *Nursery) sendUpdate(id string, update SwapUpdate) {
	if update.IsFinal {
		nursery.boltzWs.Unsubscribe(id)
	}
	nursery.globalListener.Send(update)
	if listener, ok := nursery.eventListeners[id]; ok {
		listener.Send(update)
		logger.Debugf("Sent update for swap %s", id)

		if update.IsFinal {
			nursery.removeSwapListener(id)
		}
	} else {
		logger.Debugf("No listener for swap %s", id)
	}
}

func (nursery *Nursery) SwapUpdates(id string) (<-chan SwapUpdate, func()) {
	if listener, ok := nursery.eventListeners[id]; ok {
		updates := listener.Get()
		return updates, func() {
			if listener, ok := nursery.eventListeners[id]; ok {
				listener.Remove(updates)
			}
		}
	}
	return nil, nil
}

func (nursery *Nursery) GlobalSwapUpdates() (<-chan SwapUpdate, func()) {
	updates := nursery.globalListener.Get()
	return updates, func() {
		nursery.globalListener.Remove(updates)
	}
}

func (nursery *Nursery) Init(
	network *boltz.Network,
	lightning lightning.LightningNode,
	chain *onchain.Onchain,
	boltzClient *boltz.Api,
	database *database.Database,
) error {
	nursery.ctx, nursery.cancel = context.WithCancel(context.Background())
	nursery.network = network
	nursery.lightning = lightning
	nursery.boltz = boltzClient
	nursery.database = database
	nursery.onchain = chain
	nursery.eventListeners = make(map[string]swapListener)
	nursery.globalListener = utils.ForwardChannel(make(chan SwapUpdate), 0, false)
	nursery.boltzWs = boltzClient.NewWebsocket()

	logger.Info("Starting nursery")

	if err := nursery.boltzWs.Connect(); err != nil {
		return fmt.Errorf("could not connect to boltz websocket: %v", err)
	}

	nursery.BtcBlocks = nursery.startBlockListener(boltz.CurrencyBtc)
	nursery.LiquidBlocks = nursery.startBlockListener(boltz.CurrencyLiquid)

	nursery.startSwapListener()

	return nursery.recoverSwaps()
}

func (nursery *Nursery) Stop() {
	nursery.cancel()
	if err := nursery.boltzWs.Close(); err != nil {
		logger.Errorf("Could not close boltz websocket: %v", err)
	}
	nursery.waitGroup.Wait()
	for id := range nursery.eventListeners {
		nursery.removeSwapListener(id)
	}
	nursery.globalListener.Close()
	logger.Debugf("Closed all event listeners")
	nursery.onchain.Disconnect()
}

func (nursery *Nursery) registerSwaps(swapIds []string) error {
	nursery.eventListenersLock.Lock()
	defer nursery.eventListenersLock.Unlock()

	if err := nursery.boltzWs.Subscribe(swapIds); err != nil {
		return err
	}

	for _, id := range swapIds {
		updates := make(chan SwapUpdate)
		nursery.eventListeners[id] = utils.ForwardChannel(updates, 0, true)
	}

	return nil
}

func (nursery *Nursery) recoverSwaps() error {
	logger.Info("Recovering Swaps")

	query := database.SwapQuery{
		// we also recover the ERROR state as this might be a temporary error and any swap will eventually be successfull or expired
		States: []boltzrpc.SwapState{boltzrpc.SwapState_PENDING, boltzrpc.SwapState_ERROR},
	}
	swaps, err := nursery.database.QuerySwaps(query)
	if err != nil {
		return err
	}

	reverseSwaps, err := nursery.database.QueryReverseSwaps(query)
	if err != nil {
		return err
	}

	chainSwaps, err := nursery.database.QueryChainSwaps(query)
	if err != nil {
		return err
	}

	var lockupTxs []string
	var spentTxs []string

	var swapIds []string
	for _, swap := range swaps {
		swapIds = append(swapIds, swap.Id)
		if swap.Pair.From == boltz.CurrencyLiquid {
			lockupTxs = append(lockupTxs, swap.LockupTransactionId)
		}
	}
	for _, reverseSwap := range reverseSwaps {
		if err := nursery.payReverseSwap(reverseSwap); err != nil {
			logger.Errorf("Could not initiate reverse swap payment %s: %v", reverseSwap.Id, err)
			continue
		}
		swapIds = append(swapIds, reverseSwap.Id)
	}
	for _, chainSwap := range chainSwaps {
		swapIds = append(swapIds, chainSwap.Id)
		if chainSwap.Pair.From == boltz.CurrencyLiquid {
			lockupTxs = append(lockupTxs, chainSwap.FromData.LockupTransactionId)
		}
	}

	for _, lockupTx := range lockupTxs {
		if lockupTx != "" {
			tx, err := nursery.onchain.GetTransaction(boltz.CurrencyLiquid, lockupTx, nil)
			if err != nil {
				return fmt.Errorf("could not get lockup transaction: %w", err)
			}
			for _, input := range tx.(*boltz.LiquidTransaction).Inputs {
				spentTxs = append(spentTxs, chainhash.Hash(input.Hash).String())
			}
		}
	}

	for _, anyWallet := range nursery.onchain.Wallets {
		if ownWallet, ok := anyWallet.(*wallet.Wallet); ok {
			ownWallet.SetSpentOutputs(spentTxs)
		}
	}

	return nursery.registerSwaps(swapIds)
}

func (nursery *Nursery) startSwapListener() {
	logger.Infof("Starting swap update listener")

	nursery.waitGroup.Add(2)

	go func() {
		for status := range nursery.boltzWs.Updates {
			logger.Debugf("Swap %s status update: %s", status.Id, status.Status)

			swap, reverseSwap, chainSwap, err := nursery.database.QueryAnySwap(status.Id)
			if err != nil {
				logger.Errorf("Could not query swap %s: %v", status.Id, err)
				continue
			}
			if status.Error != "" {
				logger.Warnf("Boltz could not find Swap %s: %s ", status.Id, status.Error)
				continue
			}
			if swap != nil {
				nursery.handleSwapStatus(swap, status.SwapStatusResponse)
			} else if reverseSwap != nil {
				nursery.handleReverseSwapStatus(reverseSwap, status.SwapStatusResponse)
			} else if chainSwap != nil {
				nursery.handleChainSwapStatus(chainSwap, status.SwapStatusResponse)
			}
		}
		nursery.waitGroup.Done()
	}()

	go func() {
		defer nursery.waitGroup.Done()
		notifier := wallet.TransactionNotifier.Get()
		defer wallet.TransactionNotifier.Remove(notifier)
		for {
			select {
			case notification := <-notifier:
				if err := nursery.checkExternalReverseSwaps(notification.Currency, notification.TxId); err != nil {
					logger.Errorf("Could not check external reverse swaps: %v", err)
				}
			case <-nursery.ctx.Done():
				return
			}
		}

	}()
}

func (nursery *Nursery) removeSwapListener(id string) {
	nursery.eventListenersLock.Lock()
	defer nursery.eventListenersLock.Unlock()
	if listener, ok := nursery.eventListeners[id]; ok {
		listener.Close()
		delete(nursery.eventListeners, id)
	}
}

func (nursery *Nursery) createTransaction(currency boltz.Currency, outputs []*Output) (id string, err error) {
	outputs, details := nursery.populateOutputs(outputs)
	if len(details) == 0 {
		return "", errors.New("all outputs invalid")
	}

	results := make(boltz.Results)

	handleErr := func(err error) (string, error) {
		for _, output := range outputs {
			results.SetErr(output.SwapId, err)
			if err := results[output.SwapId].Err; err != nil {
				output.setError(results[output.SwapId].Err)
			}
		}
		return id, err
	}

	feeSatPerVbyte, err := nursery.onchain.EstimateFee(currency, true)
	if err != nil {
		return handleErr(fmt.Errorf("could not get fee estimation: %w", err))
	}

	logger.Infof("Using fee of %v sat/vbyte for transaction", feeSatPerVbyte)

	transaction, results, err := boltz.ConstructTransaction(nursery.network, currency, details, feeSatPerVbyte, nursery.boltz)
	if err != nil {
		return handleErr(fmt.Errorf("construct: %w", err))
	}

	id, err = nursery.onchain.BroadcastTransaction(transaction)
	if err != nil {
		return handleErr(fmt.Errorf("broadcast: %w", err))
	}
	logger.Infof("Broadcast transaction: %s", id)

	for _, output := range outputs {
		result := results[output.SwapId]
		if result.Err == nil {
			if err := output.setTransaction(id, result.Fee); err != nil {
				logger.Errorf("Could not set transaction id for %s swap %s: %s", output.SwapType, output.SwapId, err)
				continue
			}
		}
	}

	return handleErr(nil)
}

func (nursery *Nursery) populateOutputs(outputs []*Output) (valid []*Output, details []boltz.OutputDetails) {
	addresses := make(map[database.Id]string)
	for _, output := range outputs {
		handleErr := func(err error) {
			verb := "claim"
			if output.IsRefund() {
				verb = "refund"
			}
			logger.Warnf("swap %s can not be %sed automatically: %s", output.SwapId, verb, err)
			output.setError(err)
		}
		if output.Address == "" {
			if output.walletId == nil {
				handleErr(errors.New("no address or wallet set"))
				continue
			}
			walletId := *output.walletId
			address, ok := addresses[walletId]
			if !ok {
				wallet, err := nursery.onchain.GetAnyWallet(onchain.WalletChecker{Id: &walletId, AllowReadonly: true})
				if err != nil {
					handleErr(fmt.Errorf("wallet with id %d could not be found", walletId))
					continue
				}
				address, err = wallet.NewAddress()
				if err != nil {
					handleErr(fmt.Errorf("could not get address from wallet %s: %w", wallet.GetWalletInfo().Name, err))
					continue
				}
				addresses[walletId] = address
			}
			output.Address = address
		}
		var err error
		output.LockupTransaction, output.Vout, _, err = nursery.findVout(output.voutInfo)
		if err != nil {
			handleErr(err)
			continue
		}
		valid = append(valid, output)
		details = append(details, *output.OutputDetails)
	}
	return
}

type voutInfo struct {
	transactionId    string
	currency         boltz.Currency
	address          string
	blindingKey      *btcec.PrivateKey
	expectedAmount   uint64
	requireConfirmed bool
}

var ErrNotConfirmed = errors.New("lockup transaction not confirmed")

func (nursery *Nursery) findVout(info voutInfo) (boltz.Transaction, uint32, uint64, error) {
	lockupTransaction, err := nursery.onchain.GetTransaction(info.currency, info.transactionId, info.blindingKey)
	if err != nil {
		return nil, 0, 0, errors.New("Could not decode lockup transaction: " + err.Error())
	}

	vout, value, err := lockupTransaction.FindVout(nursery.network, info.address)
	if err != nil {
		return nil, 0, 0, err
	}

	if info.expectedAmount != 0 && value < info.expectedAmount {
		return nil, 0, 0, fmt.Errorf("locked up less onchain coins than expected: %d < %d", value, info.expectedAmount)
	}
	if info.requireConfirmed {
		confirmed, err := nursery.onchain.IsTransactionConfirmed(info.currency, info.transactionId)
		if err != nil {
			return nil, 0, 0, errors.New("Could not check if lockup transaction is confirmed: " + err.Error())
		}
		if !confirmed {
			return nil, 0, 0, ErrNotConfirmed
		}
	}

	return lockupTransaction, vout, value, nil
}

func (nursery *Nursery) ClaimSwaps(currency boltz.Currency, reverseSwaps []*database.ReverseSwap, chainSwaps []*database.ChainSwap) (string, error) {
	var outputs []*Output
	for _, swap := range reverseSwaps {
		outputs = append(outputs, nursery.getReverseSwapClaimOutput(swap))
	}
	for _, swap := range chainSwaps {
		outputs = append(outputs, nursery.getChainSwapClaimOutput(swap))
	}
	return nursery.createTransaction(currency, outputs)
}
