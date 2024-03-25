package nursery

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/utils"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
)

type Nursery struct {
	network *boltz.Network

	lightning lightning.LightningNode

	onchain  *onchain.Onchain
	boltz    *boltz.Boltz
	boltzWs  *boltz.BoltzWebsocket
	database *database.Database

	eventListeners     map[string]swapListener
	eventListenersLock sync.RWMutex
	globalListener     swapListener
	waitGroup          sync.WaitGroup
	stop               *utils.ChannelForwarder[bool]

	stopped bool
}

const retryInterval = 15

type SwapUpdate struct {
	Swap        *database.Swap
	ReverseSwap *database.ReverseSwap
	IsFinal     bool
}

type swapListener = *utils.ChannelForwarder[SwapUpdate]

func (nursery *Nursery) sendUpdate(id string, update SwapUpdate) {
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
	boltzClient *boltz.Boltz,
	database *database.Database,
) error {
	nursery.network = network
	nursery.lightning = lightning
	nursery.boltz = boltzClient
	nursery.database = database
	nursery.onchain = chain
	nursery.eventListeners = make(map[string]swapListener)
	nursery.globalListener = utils.ForwardChannel(make(chan SwapUpdate), 0, false)
	nursery.stop = utils.ForwardChannel(make(chan bool), 0, false)
	nursery.boltzWs = boltz.NewBoltzWebsocket(boltzClient.URL)

	logger.Info("Starting nursery")

	if err := nursery.boltzWs.Connect(); err != nil {
		return fmt.Errorf("could not connect to boltz websocket: %v", err)
	}

	nursery.startBlockListener(boltz.CurrencyBtc)
	nursery.startBlockListener(boltz.CurrencyLiquid)

	nursery.startSwapListener()

	return nursery.recoverPending()
}

func (nursery *Nursery) Stop() {
	nursery.stopped = true
	nursery.stop.Send(true)
	logger.Debugf("Sent stop signal to block listener")
	for id := range nursery.eventListeners {
		nursery.removeSwapListener(id)
	}
	nursery.globalListener.Close()
	logger.Debugf("Closed all event listeners")
	nursery.boltzWs.Close()
	nursery.waitGroup.Wait()
}

func (nursery *Nursery) registerSwap(id string) error {
	logger.Infof("Listening to events of Swap %s", id)
	nursery.eventListenersLock.Lock()
	defer nursery.eventListenersLock.Unlock()

	if err := nursery.boltzWs.Subscribe([]string{id}); err != nil {
		return err
	}

	updates := make(chan SwapUpdate)
	nursery.eventListeners[id] = utils.ForwardChannel(updates, 0, true)

	return nil
}

func (nursery *Nursery) recoverPending() error {
	logger.Info("Recovering pending Swaps")

	swaps, err := nursery.database.QueryPendingSwaps()
	if err != nil {
		return err
	}

	reverseSwaps, err := nursery.database.QueryPendingReverseSwaps()
	if err != nil {
		return err
	}

	var swapIds []string
	for _, swap := range swaps {
		swapIds = append(swapIds, swap.Id)
	}
	for _, reverseSwap := range reverseSwaps {
		swapIds = append(swapIds, reverseSwap.Id)
	}

	return nursery.boltzWs.Subscribe(swapIds)
}

func (nursery *Nursery) startSwapListener() {
	logger.Infof("Starting swap update listener")

	nursery.waitGroup.Add(1)

	go func() {
		for status := range nursery.boltzWs.Updates {
			logger.Infof("Swap %s status update: %s", status.Id, status.Status)

			swap, reverseSwap, err := nursery.database.QueryAnySwap(status.Id)
			if err != nil {
				logger.Errorf("Could not query swap %s: %v", status.Id, status.Status)
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
			}
		}
		nursery.waitGroup.Done()
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

func (nursery *Nursery) registerBlockListener(currency boltz.Currency) chan *onchain.BlockEpoch {
	logger.Infof("Connecting to block %s epoch stream", currency)
	blockNotifier := make(chan *onchain.BlockEpoch)
	stop := nursery.stop.Get()
	nursery.waitGroup.Add(1)
	go func() {
		defer func() {
			close(blockNotifier)
			nursery.waitGroup.Done()
			logger.Debugf("Closed block listener for %s", currency)
		}()
		for !nursery.stopped {
			listener := nursery.onchain.GetBlockListener(currency)
			if listener == nil {
				logger.Errorf("no block listener for %s", currency)
			} else {
				err := listener.RegisterBlockListener(blockNotifier, stop)
				if err != nil {
					logger.Errorf("Lost connection to %s block epoch stream: %s", currency, err.Error())
					logger.Infof("Retrying connection in " + strconv.Itoa(retryInterval) + " seconds")
				}
			}
			if nursery.stopped {
				return
			}
			select {
			case <-stop:
				return
			case <-time.After(retryInterval * time.Second):
			}
		}
	}()
	return blockNotifier
}

func (nursery *Nursery) getFeeEstimation(currency boltz.Currency) (float64, error) {
	return nursery.onchain.EstimateFee(currency, 2)
}

func (nursery *Nursery) createTransaction(currency boltz.Currency, outputs []boltz.OutputDetails, address string, feeSatPerVbyte float64) (string, uint64, error) {
	transaction, fee, err := boltz.ConstructTransaction(nursery.network, currency, outputs, address, feeSatPerVbyte, nursery.boltz)
	if err != nil {
		return "", 0, fmt.Errorf("construct transaction: %v", err)
	}

	response, err := nursery.boltz.BroadcastTransaction(transaction)
	if err != nil {
		return "", 0, fmt.Errorf("broadcast transaction: %v", err)
	}
	logger.Info("Broadcast transaction with Boltz API")

	return response.TransactionId, fee, nil
}
