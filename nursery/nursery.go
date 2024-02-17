package nursery

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/utils"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
)

type Claimer struct {
	ExpiryTolerance time.Duration `long:"expiry-tolerance" description:"Time before a swap expires that it should be claimed" default:"1h"`
	DeferredSymbols []boltz.Currency
	ClaimInterval   time.Duration `long:"claim-interval" description:"Interval at which the claimer should check for deferred swaps" default:"5m"`

	claim         func([]*database.ReverseSwap) error
	deferredSwaps []*database.ReverseSwap
	database      *database.Database
}

func (nursery *Nursery) StartClaimer() {
	logger.Infof("Starting claimer")

	nursery.waitGroup.Add(1)
	go func() {
		stop := nursery.stop.Get()
		ticker := time.NewTicker(nursery.claimer.ClaimInterval)
		defer ticker.Stop()
		defer nursery.waitGroup.Done()

		for {
			select {
			case <-ticker.C:
				if err := nursery.Sweep(); err != nil {
					logger.Errorf("Error sweeping deferred swaps: %v", err)
				}
			case <-stop:
				return
			}
		}
	}()
}

func (claimer *Claimer) shouldDefer(reverseSwap *database.ReverseSwap, currentBlockHeight uint32) bool {
	if reverseSwap.State != boltzrpc.SwapState_PENDING {
		return false
	}

	claimer.deferredSwaps = append(claimer.deferredSwaps, reverseSwap)

	blocks := reverseSwap.TimeoutBlockHeight - currentBlockHeight
	timeout := time.Duration(boltz.BlocksToHours(blocks, reverseSwap.Pair.To) * float64(time.Hour))

	if timeout < claimer.ExpiryTolerance {
		return false
	}

	return true

}

func (nursery *Nursery) Sweep() error {
	logger.Infof("Starting Sweep")

	if err := nursery.claimReverseSwaps(nursery.claimer.deferredSwaps); err != nil {
		return err
	}
	nursery.claimer.deferredSwaps = nil
	return nil
}

type Nursery struct {
	network *boltz.Network

	lightning lightning.LightningNode

	onchain  *onchain.Onchain
	boltz    *boltz.Boltz
	boltzWs  *boltz.BoltzWebsocket
	database *database.Database

	eventListeners     map[string]swapListener
	eventListenersLock sync.RWMutex
	waitGroup          sync.WaitGroup
	stop               *utils.ChannelForwarder[bool]

	stopped bool
	claimer *Claimer
}

const retryInterval = 15

type SwapUpdate struct {
	Swap        *database.Swap
	ReverseSwap *database.ReverseSwap
	IsFinal     bool
}

type swapListener = *utils.ChannelForwarder[SwapUpdate]

func (nursery *Nursery) sendUpdate(id string, update SwapUpdate) {
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

func (nursery *Nursery) Init(
	network *boltz.Network,
	lightning lightning.LightningNode,
	chain *onchain.Onchain,
	boltzClient *boltz.Boltz,
	database *database.Database,
	claimer *Claimer,
) error {
	nursery.network = network
	nursery.lightning = lightning
	nursery.boltz = boltzClient
	nursery.database = database
	nursery.onchain = chain
	nursery.eventListeners = make(map[string]swapListener)
	nursery.stop = utils.ForwardChannel(make(chan bool), 0, false)
	nursery.boltzWs = boltz.NewBoltzWebsocket(boltzClient.URL)
	nursery.claimer = claimer

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
				logger.Warnf("Boltz could not find Swap %s: %s ", swap.Id, status.Error)
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

func (nursery *Nursery) createTransaction(currency boltz.Currency, outputs []boltz.OutputDetails, feeSatPerVbyte float64, signer boltz.Signer) (string, uint64, error) {
	transaction, fee, err := boltz.ConstructTransaction(nursery.network, currency, outputs, feeSatPerVbyte, signer)
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
