package nursery

import (
	"context"
	"fmt"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"sync"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
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
	claimer            *Claimer

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
	claimer *Claimer,
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
	nursery.claimer = claimer

	logger.Info("Starting nursery")

	if err := nursery.boltzWs.Connect(); err != nil {
		return fmt.Errorf("could not connect to boltz websocket: %v", err)
	}

	nursery.BtcBlocks = nursery.startBlockListener(boltz.CurrencyBtc)
	nursery.LiquidBlocks = nursery.startBlockListener(boltz.CurrencyLiquid)

	nursery.claimer.Init(nursery.onchain)
	nursery.startClaimer()

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

func (nursery *Nursery) createTransaction(currency boltz.Currency, outputs []*CheckedOutput) boltz.ConstructResult {
	details := make([]boltz.OutputDetails, 0, len(outputs))
	addresses := make(map[database.Id]string)
	for _, output := range outputs {
		if output.walletId != nil {
			walletId := *output.walletId
			address, ok := addresses[walletId]
			if ok {
				output.Address = address
			} else {
				addresses[walletId] = output.Address
			}
		} else if output.Address == "" {
			logger.Warnf("no address or wallet set for swap %s", output.SwapId)
		}
		details = append(details, *output.OutputDetails)
	}

	var result boltz.ConstructResult

	handleErr := func(err error) boltz.ConstructResult {
		result.Err = err
		for _, output := range outputs {
			details := result.SwapResult(output.SwapId)
			if details.Err != nil {
				output.setError(result.Err)
			} else {
				if err := output.setTransaction(result.TransactionId, details.Fee); err != nil {
					logger.Errorf("Could not set transaction id for %s swap %s: %s", output.SwapType, output.SwapId, err)
					continue
				}
			}
		}
		return result
	}

	feeSatPerVbyte, err := nursery.onchain.EstimateFee(currency, true)
	if err != nil {
		return handleErr(fmt.Errorf("could not get fee estimation: %w", err))
	}

	logger.Infof("Using fee of %v sat/vbyte for transaction", feeSatPerVbyte)

	result = boltz.ConstructTransaction(nursery.network, currency, details, feeSatPerVbyte, nursery.boltz)

	if result.Transaction != nil {
		_, err := nursery.onchain.BroadcastTransaction(result.Transaction)
		if err != nil {
			return handleErr(err)
		}
		logger.Infof("Broadcast transaction: %s", result.Transaction.Hash())
	}

	return handleErr(nil)
}

func (nursery *Nursery) CheckAmounts(swapType boltz.SwapType, pair boltz.Pair, sendAmount uint64, receiveAmount uint64, serviceFee boltz.Percentage) (err error) {
	fees := make(boltz.FeeEstimations)
	fees[boltz.CurrencyLiquid], err = nursery.onchain.EstimateFee(boltz.CurrencyLiquid, true)
	if err != nil {
		return err
	}
	fees[boltz.CurrencyBtc], err = nursery.onchain.EstimateFee(boltz.CurrencyBtc, true)
	if err != nil {
		return err
	}
	return boltz.CheckAmounts(swapType, pair, sendAmount, receiveAmount, serviceFee, fees, false)
}
