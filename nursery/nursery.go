package nursery

import (
	"errors"
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
	database *database.Database

	eventListenersGroup sync.WaitGroup
	eventListeners      map[string]swapListener
	eventListenersLock  sync.RWMutex
	stopBlockListeners  []chan bool
	blockListenerGroup  sync.WaitGroup

	stopped bool
}

const retryInterval = 15

type SwapUpdate struct {
	Swap        *database.Swap
	ReverseSwap *database.ReverseSwap
	IsFinal     bool
}

type swapListener struct {
	stop      chan bool
	updates   chan<- SwapUpdate
	forwarder *utils.ChannelForwarder[SwapUpdate]
}

func (listener *swapListener) close() {
	select {
	case listener.stop <- true:
		logger.Debug("Sent stop signal to listener")
	default:
		logger.Debug("Listener already stopped")
	}
}

// Map between Swap ids and a channel that tells its SSE event listeners to stop
func (nursery *Nursery) newListener(id string) (swapListener, func()) {
	nursery.eventListenersLock.Lock()
	defer nursery.eventListenersLock.Unlock()
	updates := make(chan SwapUpdate)
	listener := swapListener{
		stop:      make(chan bool),
		updates:   updates,
		forwarder: utils.ForwardChannel(updates, 0, true),
	}
	nursery.eventListeners[id] = listener
	logger.Debug("Creating listener for swap " + id)
	nursery.eventListenersGroup.Add(1)
	return listener, func() {
		nursery.eventListenersLock.Lock()
		defer nursery.eventListenersLock.Unlock()
		close(listener.updates)
		delete(nursery.eventListeners, id)
		nursery.eventListenersGroup.Done()
	}
}

func (nursery *Nursery) getSwapListener(id string) *swapListener {
	nursery.eventListenersLock.RLock()
	defer nursery.eventListenersLock.RUnlock()
	listener, ok := nursery.eventListeners[id]
	if ok {
		return &listener
	}
	return nil
}

func (nursery *Nursery) sendUpdate(id string, update SwapUpdate) {
	if listener := nursery.getSwapListener(id); listener != nil {
		listener.updates <- update
		logger.Debugf("Sent update for swap %s", id)

		if update.IsFinal {
			listener.close()
		}
	} else {
		logger.Debugf("No listener for swap %s", id)
	}
}

func (nursery *Nursery) SwapUpdates(id string) (<-chan SwapUpdate, func()) {
	listener := nursery.getSwapListener(id)
	if listener != nil {
		updates := listener.forwarder.Get()
		return updates, func() {
			if listener := nursery.getSwapListener(id); listener != nil {
				listener.forwarder.Remove(updates)
			}
		}
	}
	return nil, nil
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

	logger.Info("Starting nursery")

	if err := nursery.recoverSwaps(); err != nil {
		return err
	}
	nursery.startBlockListener(boltz.CurrencyBtc)
	nursery.startBlockListener(boltz.CurrencyLiquid)

	err := nursery.recoverReverseSwaps()

	return err
}

func (nursery *Nursery) Stop() {
	nursery.stopped = true
	for _, stop := range nursery.stopBlockListeners {
		select {
		case stop <- true:
			logger.Debugf("Sent stop signal to block listener")
		case <-time.After(1 * time.Second):
			logger.Debugf("block listener did not receive stop signal")
		}
	}
	logger.Debugf("Closed all block listeners")
	for _, listener := range nursery.eventListeners {
		listener.close()
	}
	logger.Debugf("Closed all event listeners")
	nursery.eventListenersGroup.Wait()
	nursery.blockListenerGroup.Wait()
}

func (nursery *Nursery) registerBlockListener(currency boltz.Currency) chan *onchain.BlockEpoch {
	logger.Infof("Connecting to block %s epoch stream", currency)
	blockNotifier := make(chan *onchain.BlockEpoch)
	stop := make(chan bool)
	nursery.stopBlockListeners = append(nursery.stopBlockListeners, stop)
	nursery.blockListenerGroup.Add(1)
	go func() {
		defer func() {
			close(blockNotifier)
			nursery.blockListenerGroup.Done()
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
	transaction, fee, err := nursery.boltz.ConstructTransaction(nursery.network, currency, outputs, address, feeSatPerVbyte)
	if err != nil {
		return "", 0, fmt.Errorf("construct transaction: %v", err)
	}

	id := transaction.Hash()
	err = nursery.broadcastTransaction(transaction, currency)
	if err != nil {
		return "", 0, fmt.Errorf("broadcast transaction: %v", err)
	}
	return id, fee, nil
}

func (nursery *Nursery) broadcastTransaction(transaction boltz.Transaction, currency boltz.Currency) error {
	transactionHex, err := transaction.Serialize()
	if err != nil {
		return errors.New("could not serialize transaction: " + err.Error())
	}

	_, err = nursery.boltz.BroadcastTransaction(transactionHex, currency)
	if err != nil {
		logger.Errorf("Could not broadcast transaction: %v\n%s", err, transactionHex)
		return errors.New("could not broadcast transaction: " + err.Error())
	}

	logger.Info("Broadcast transaction with Boltz API")

	return nil
}
