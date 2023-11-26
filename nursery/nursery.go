package nursery

import (
	"errors"
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
var eventListeners = make(map[string]swapListener)
var eventListenersLock sync.RWMutex
var eventListenersGroup sync.WaitGroup

func newListener(id string) (swapListener, func()) {
	eventListenersLock.Lock()
	defer eventListenersLock.Unlock()
	updates := make(chan SwapUpdate)
	listener := swapListener{
		stop:      make(chan bool),
		updates:   updates,
		forwarder: utils.ForwardChannel(updates, 0, true),
	}
	eventListeners[id] = listener
	logger.Debug("Creating listener for swap " + id)
	eventListenersGroup.Add(1)
	return listener, func() {
		eventListenersLock.Lock()
		defer eventListenersLock.Unlock()
		close(listener.stop)
		close(listener.updates)
		delete(eventListeners, id)
		eventListenersGroup.Done()
	}
}

func getSwapListener(id string) *swapListener {
	eventListenersLock.RLock()
	defer eventListenersLock.RUnlock()
	listener, ok := eventListeners[id]
	if ok {
		return &listener
	}
	return nil
}

func sendUpdate(id string, update SwapUpdate) {
	if listener := getSwapListener(id); listener != nil {
		listener.updates <- update
		logger.Debugf("Sent update for swap %s", id)

		if update.IsFinal {
			listener.close()
		}
	}
}

func (nursery *Nursery) SwapUpdates(id string) (<-chan SwapUpdate, func()) {
	listener := getSwapListener(id)
	if listener != nil {
		updates := listener.forwarder.Get()
		return updates, func() {
			if listener := getSwapListener(id); listener != nil {
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

	logger.Info("Starting nursery")

	if err := nursery.recoverSwaps(); err != nil {
		return err
	}

	if err := nursery.startBlockListener(boltz.PairBtc); err != nil {
		return err
	}

	if err := nursery.startBlockListener(boltz.PairLiquid); err != nil {
		return err
	}

	err := nursery.recoverReverseSwaps()

	return err
}

func (nursery *Nursery) Stop() {
	for _, listener := range eventListeners {
		listener.close()
	}
	eventListenersGroup.Wait()
}

func (nursery *Nursery) registerBlockListener(pair boltz.Pair, blockNotifier chan *onchain.BlockEpoch) error {
	logger.Info("Connecting to block epoch stream")
	go func() {
		for {
			listener := nursery.onchain.GetBlockListener(pair)
			if listener == nil {
				logger.Errorf("no block listener for %s", pair)
				time.Sleep(retryInterval * time.Second)
			} else {
				err := listener.RegisterBlockListener(blockNotifier)
				if err != nil {
					logger.Errorf("Lost connection to %s block epoch stream: %s", utils.CurrencyFromPair(pair), err.Error())
					logger.Infof("Retrying connection in " + strconv.Itoa(retryInterval) + " seconds")

					time.Sleep(retryInterval * time.Second)
				}
			}
		}
	}()
	return nil
}

func (nursery *Nursery) getFeeEstimation(pair boltz.Pair) (float64, error) {
	return nursery.onchain.EstimateFee(pair, 2)
}

func (nursery *Nursery) broadcastTransaction(transaction boltz.Transaction, currency string) error {
	transactionHex, err := transaction.Serialize()
	if err != nil {
		return errors.New("could not serialize transaction: " + err.Error())
	}

	_, err = nursery.boltz.BroadcastTransaction(transactionHex, currency)

	if err != nil {
		return errors.New("could not broadcast transaction: " + err.Error())
	}

	logger.Info("Broadcast transaction with Boltz API")

	return nil
}
