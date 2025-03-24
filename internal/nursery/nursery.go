package nursery

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"

	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
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

	var swapIds []string
	for _, swap := range swaps {
		swapIds = append(swapIds, swap.Id)
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
	}

	return nursery.registerSwaps(swapIds)
}

func (nursery *Nursery) setSomeSwapError(swap *database.SomeSwap, err error) {
	var dbErr error
	if swap.Normal != nil {
		dbErr = nursery.database.UpdateSwapState(swap.Normal, boltzrpc.SwapState_ERROR, err.Error())
		logger.Errorf("Submarine Swap %s error: %v", swap.Normal.Id, err)
	}
	if swap.Reverse != nil {
		dbErr = nursery.database.UpdateReverseSwapState(swap.Reverse, boltzrpc.SwapState_ERROR, err.Error())
		logger.Errorf("Reverse Swap %s error: %v", swap.Reverse.Id, err)
	}
	if swap.Chain != nil {
		dbErr = nursery.database.UpdateChainSwapState(swap.Chain, boltzrpc.SwapState_ERROR, err.Error())
		logger.Errorf("Chain Swap %s error: %v", swap.Chain.Id, err)
	}
	if dbErr != nil {
		logger.Error(dbErr.Error())
	}
}

func (nursery *Nursery) sendSomeSwapUpdate(swapId string) {
	swap, err := nursery.database.QueryAnySwap(swapId)
	if err != nil {
		logger.Errorf("Could not query swap %s: %v", swapId, err)
		return
	}
	if swap.Normal != nil {
		nursery.sendSwapUpdate(*swap.Normal)
	}
	if swap.Chain != nil {
		nursery.sendChainSwapUpdate(*swap.Chain)
	}
	if swap.Reverse != nil {
		nursery.sendReverseSwapUpdate(*swap.Reverse)
	}
}

func (nursery *Nursery) startSwapListener() {
	logger.Infof("Starting swap update listener")

	nursery.waitGroup.Add(2)

	go func() {
		for status := range nursery.boltzWs.Updates {
			logger.Debugf("Swap %s status update: %s", status.Id, status.Status)

			someSwap, err := nursery.database.QueryAnySwap(status.Id)
			if err != nil {
				logger.Errorf("Could not query swap %s: %v", status.Id, err)
				continue
			}
			if status.Error != "" {
				logger.Warnf("Boltz could not find Swap %s: %s ", status.Id, status.Error)
				continue
			}
			err = nursery.database.RunTx(func(tx *database.Transaction) error {
				if someSwap.Normal != nil {
					return nursery.handleSwapStatus(tx, someSwap.Normal, status.SwapStatusResponse)
				} else if someSwap.Reverse != nil {
					return nursery.handleReverseSwapStatus(tx, someSwap.Reverse, status.SwapStatusResponse)
				} else if someSwap.Chain != nil {
					return nursery.handleChainSwapStatus(tx, someSwap.Chain, status.SwapStatusResponse)
				}
				return nil
			})
			if err != nil {
				nursery.setSomeSwapError(someSwap, err)
			}
			nursery.sendSomeSwapUpdate(status.Id)
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

	feeSatPerVbyte, err := nursery.onchain.EstimateFee(currency)
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
		result, err := nursery.onchain.FindOutput(output.outputArgs)
		if err != nil {
			handleErr(err)
			continue
		}
		output.LockupTransaction = result.Transaction
		output.Vout = result.Vout
		valid = append(valid, output)
		details = append(details, *output.OutputDetails)
	}
	return
}

func (nursery *Nursery) CheckAmounts(swapType boltz.SwapType, pair boltz.Pair, sendAmount uint64, receiveAmount uint64, serviceFee boltz.Percentage) (err error) {
	fees := make(boltz.FeeEstimations)
	fees[boltz.CurrencyLiquid], err = nursery.onchain.EstimateFee(boltz.CurrencyLiquid)
	if err != nil {
		return err
	}
	fees[boltz.CurrencyBtc], err = nursery.onchain.EstimateFee(boltz.CurrencyBtc)
	if err != nil {
		return err
	}
	return boltz.CheckAmounts(swapType, pair, sendAmount, receiveAmount, serviceFee, fees, false)
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
