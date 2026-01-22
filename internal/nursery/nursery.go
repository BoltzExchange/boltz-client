package nursery

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"

	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"

	dbpkg "github.com/BoltzExchange/boltz-client/v2/internal/database"
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
	database *dbpkg.Database

	eventListeners     map[string]swapListener
	eventListenersLock sync.RWMutex
	globalListener     swapListener
	waitGroup          sync.WaitGroup
	maxZeroConfAmount  uint64
	maxRoutingFeePpm   uint64

	// updateLock is locked when a swap update is being processed.
	// it is used to prevent a race between a `ClaimSwaps` call where an update can be triggered
	// before the claim tx isnt properly broadcast and the swap is updated in the db.
	updateLock sync.Mutex

	BtcBlocks    *utils.ChannelForwarder[*onchain.BlockEpoch]
	LiquidBlocks *utils.ChannelForwarder[*onchain.BlockEpoch]

	FundingAddressUpdates *utils.ChannelForwarder[*dbpkg.FundingAddress]
}

func New(
	maxZeroConfAmount *uint64,
	maxRoutingFeePpm uint64,
	network *boltz.Network,
	lightning lightning.LightningNode,
	chain *onchain.Onchain,
	boltzClient *boltz.Api,
	database *dbpkg.Database,
) *Nursery {
	nursery := &Nursery{
		network:               network,
		lightning:             lightning,
		onchain:               chain,
		boltz:                 boltzClient,
		database:              database,
		eventListeners:        make(map[string]swapListener),
		globalListener:        utils.ForwardChannel(make(chan SwapUpdate), 0, false),
		boltzWs:               boltzClient.NewWebsocket(),
		maxRoutingFeePpm:      maxRoutingFeePpm,
		FundingAddressUpdates: utils.ForwardChannel[*dbpkg.FundingAddress](make(chan *dbpkg.FundingAddress), 0, false),
	}
	if maxZeroConfAmount != nil {
		nursery.maxZeroConfAmount = *maxZeroConfAmount
	}
	nursery.ctx, nursery.cancel = context.WithCancel(context.Background())
	return nursery
}

type SwapUpdate struct {
	Swap        *dbpkg.Swap
	ReverseSwap *dbpkg.ReverseSwap
	ChainSwap   *dbpkg.ChainSwap
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

func (nursery *Nursery) Init() error {
	logger.Info("Starting nursery")

	if nursery.maxZeroConfAmount == 0 {
		pairs, err := nursery.boltz.GetSubmarinePairs()
		if err != nil {
			return fmt.Errorf("could not get submarine pairs: %v", err)
		}
		pair, err := boltz.FindPair(boltz.Pair{From: boltz.CurrencyLiquid, To: boltz.CurrencyBtc}, pairs)
		if err != nil {
			return fmt.Errorf("could not find submarine pair: %v", err)
		}
		nursery.maxZeroConfAmount = pair.Limits.MaximalZeroConfAmount
		logger.Infof("No maximal zero conf amount set, using same value as boltz: %v", nursery.maxZeroConfAmount)
	}

	if err := nursery.boltzWs.Connect(); err != nil {
		return fmt.Errorf("could not connect to boltz websocket: %v", err)
	}

	nursery.BtcBlocks = nursery.startBlockListener(boltz.CurrencyBtc)
	nursery.LiquidBlocks = nursery.startBlockListener(boltz.CurrencyLiquid)

	nursery.startSwapListener()
	nursery.startFundingListener()

	if err := nursery.recoverSwaps(); err != nil {
		return err
	}

	return nursery.recoverFundingAddresses()
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
	nursery.FundingAddressUpdates.Close()
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

	query := dbpkg.SwapQuery{
		// we also recover the ERROR state as this might be a temporary error and any swap will eventually be successful or expired
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

func (nursery *Nursery) processUpdate(status boltz.SwapUpdate) error {
	nursery.updateLock.Lock()
	defer nursery.updateLock.Unlock()

	swap, reverseSwap, chainSwap, err := nursery.database.QueryAnySwap(status.Id)
	if err != nil {
		return fmt.Errorf("could not query swap %s: %v", status.Id, err)
	}
	if status.Error != "" {
		return fmt.Errorf("boltz could not find Swap %s: %s", status.Id, status.Error)
	}
	if swap != nil {
		nursery.handleSwapStatus(swap, status.SwapStatusResponse)
	} else if reverseSwap != nil {
		nursery.handleReverseSwapStatus(reverseSwap, status.SwapStatusResponse)
	} else if chainSwap != nil {
		nursery.handleChainSwapStatus(chainSwap, status.SwapStatusResponse)
	}
	return nil
}

func (nursery *Nursery) startSwapListener() {
	logger.Infof("Starting swap update listener")

	nursery.waitGroup.Add(2)

	go func() {
		for status := range nursery.boltzWs.Updates {
			logger.Debugf("Swap %s status update: %s", status.Id, status.Status)
			if err := nursery.processUpdate(status); err != nil {
				logger.Errorf("Could not process swap update: %v", err)
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
				if err := nursery.checkExternalReverseSwaps(notification.Currency); err != nil {
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
	logger.Debugf("Creating tx for %s and %d outputs", currency, len(outputs))

	outputs, details := nursery.populateOutputs(outputs)
	if len(details) == 0 {
		return "", errors.New("all outputs invalid")
	}

	logger.Debugf("Got %d valid outputs", len(outputs))

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

	transaction, results, err := boltz.ConstructTransaction(nursery.network, currency, details, boltz.Fee{SatsPerVbyte: &feeSatPerVbyte}, nursery.boltz)
	if err != nil {
		return handleErr(fmt.Errorf("construct: %w", err))
	}

	logger.Debugf("Constructed tx, broadcasting")

	id, err = nursery.onchain.BroadcastTransaction(transaction)
	if err != nil {
		return handleErr(fmt.Errorf("broadcast: %w", err))
	}
	logger.Infof("Broadcast transaction: %s", id)

	txHex, err := transaction.Serialize()
	if err != nil {
		return handleErr(fmt.Errorf("serialize: %w", err))
	}

	for _, output := range outputs {
		result := results[output.SwapId]
		if result.Err == nil {
			if err := output.setTransaction(id, result.Fee); err != nil {
				logger.Errorf("Could not set transaction id for %s swap %s: %s", output.SwapType, output.SwapId, err)
				continue
			}
			if output.walletId != nil {
				wallet, err := nursery.onchain.GetAnyWallet(onchain.WalletChecker{Id: output.walletId, AllowReadonly: true})
				if err != nil {
					results.SetErr(output.SwapId, fmt.Errorf("wallet with id %d could not be found", *output.walletId))
					continue
				}
				if err := wallet.ApplyTransaction(txHex); err != nil {
					results.SetErr(output.SwapId, fmt.Errorf("could not apply transaction to wallet %s: %w", wallet.GetWalletInfo().Name, err))
					continue
				}
			}
		}
	}

	return handleErr(nil)
}

func (nursery *Nursery) populateOutputs(outputs []*Output) (valid []*Output, details []boltz.OutputDetails) {
	addresses := make(map[dbpkg.Id]string)
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

func (nursery *Nursery) GetFeeEstimations(swapType boltz.SwapType, pair boltz.Pair) (boltz.FeeEstimations, error) {
	fees := make(boltz.FeeEstimations)
	var err error
	for _, currency := range boltz.RequiredEstimations(swapType, pair) {
		fees[currency], err = nursery.onchain.EstimateFee(currency)
		if err != nil {
			return nil, err
		}
	}
	return fees, nil
}

func (nursery *Nursery) CheckAmounts(swapType boltz.SwapType, pair boltz.Pair, sendAmount uint64, receiveAmount uint64, serviceFee boltz.Percentage) (err error) {
	fees, err := nursery.GetFeeEstimations(swapType, pair)
	if err != nil {
		return err
	}
	return boltz.CheckAmounts(swapType, pair, sendAmount, receiveAmount, serviceFee, fees, false)
}

func (nursery *Nursery) ClaimSwaps(currency boltz.Currency, reverseSwaps []*dbpkg.ReverseSwap, chainSwaps []*dbpkg.ChainSwap) (string, error) {
	nursery.updateLock.Lock()
	defer nursery.updateLock.Unlock()

	return nursery.claimSwaps(currency, reverseSwaps, chainSwaps)
}

func (nursery *Nursery) claimSwaps(currency boltz.Currency, reverseSwaps []*dbpkg.ReverseSwap, chainSwaps []*dbpkg.ChainSwap) (string, error) {
	var outputs []*Output
	for _, swap := range reverseSwaps {
		outputs = append(outputs, nursery.getReverseSwapClaimOutput(swap))
	}
	for _, swap := range chainSwaps {
		outputs = append(outputs, nursery.getChainSwapClaimOutput(swap))
	}
	return nursery.createTransaction(currency, outputs)
}

func (nursery *Nursery) QueryClaimableSwaps(tenantId *dbpkg.Id, currency boltz.Currency) (
	[]*dbpkg.ReverseSwap, []*dbpkg.ChainSwap, error,
) {
	reverseSwaps, chainSwaps, err := nursery.database.QueryAllClaimableSwaps(tenantId, currency)
	if err != nil {
		return nil, nil, err
	}

	reverseSwaps = slices.DeleteFunc(reverseSwaps, func(reverseSwap *dbpkg.ReverseSwap) bool {
		if !reverseSwap.AcceptZeroConf {
			confirmed, err := nursery.onchain.IsTransactionConfirmed(currency, reverseSwap.LockupTransactionId, false)
			if err != nil {
				logger.Errorf("Could not check if reverse swap lockup %s is confirmed: %v", reverseSwap.Id, err)
				return true
			}
			return !confirmed
		}
		return false
	})

	chainSwaps = slices.DeleteFunc(chainSwaps, func(chainSwap *dbpkg.ChainSwap) bool {
		if !chainSwap.AcceptZeroConf {
			confirmed, err := nursery.onchain.IsTransactionConfirmed(currency, chainSwap.ToData.LockupTransactionId, false)
			if err != nil {
				logger.Errorf("Could not check if chain swap lockup %s is confirmed: %v", chainSwap.Id, err)
				return true
			}
			return !confirmed
		}
		return false
	})

	return reverseSwaps, chainSwaps, nil
}

func (nursery *Nursery) RegisterFundingAddress(fa *dbpkg.FundingAddress) error {
	return nursery.boltzWs.SubscribeFunding([]string{fa.Id})
}

func (nursery *Nursery) startFundingListener() {
	logger.Infof("Starting funding address update listener")

	nursery.waitGroup.Add(1)

	go func() {
		defer nursery.waitGroup.Done()
		for update := range nursery.boltzWs.FundingUpdates {
			logger.Debugf("Funding address %s status update: %s", update.Id, update.Status)
			if err := nursery.processFundingUpdate(update); err != nil {
				logger.Errorf("Could not process funding update: %v", err)
			}
		}
	}()
}

func (nursery *Nursery) processFundingUpdate(update boltz.FundingUpdate) error {
	logger.Debugf("Processing funding update: %s", update.Id)
	event := boltz.ParseFundingEvent(update.Status)
	fa, err := nursery.database.QueryFundingAddress(update.Id)
	if err != nil {
		return fmt.Errorf("could not query funding address %s: %v", update.Id, err)
	}

	// Update status
	if err := nursery.database.UpdateFundingAddressStatus(fa, update.Status); err != nil {
		return fmt.Errorf("could not update funding address status: %v", err)
	}

	// Update lockup transaction ID if present
	if update.Transaction != nil && update.Transaction.Id != "" {
		if err := nursery.database.SetFundingAddressLockupTransactionId(fa, update.Transaction.Id); err != nil {
			return fmt.Errorf("could not update funding address lockup transaction: %v", err)
		}
	}

	// Update swap ID if present
	if update.SwapId != "" {
		if err := nursery.database.SetFundingAddressSwapId(fa, update.SwapId); err != nil {
			return fmt.Errorf("could not update funding address swap id: %v", err)
		}
	}

	// Send update to stream listeners
	nursery.FundingAddressUpdates.Send(fa)

	// Check if this is a final status (spent, expired, refunded)
	if event.IsFinalStatus() {
		nursery.boltzWs.UnsubscribeFunding(update.Id)
	}

	return nil
}

func (nursery *Nursery) SignFundingAddress(fundingAddress *dbpkg.FundingAddress) error {
	// Get signing details from Boltz API
	signingDetails, err := nursery.boltz.GetFundingAddressSigningDetails(fundingAddress.Id, fundingAddress.SwapId)
	if err != nil {
		return fmt.Errorf("failed to get signing details from boltz: %v", err)
	}

	fundingTree, err := fundingAddress.GetFundingTree()
	if err != nil {
		return fmt.Errorf("failed to create funding tree: %v", err)
	}
	// Create signing session
	session, err := boltz.NewFundingSigningSession(fundingTree)
	if err != nil {
		return fmt.Errorf("failed to create signing session: %v", err)
	}

	// Sign the transaction hash
	partial, err := session.Sign(signingDetails)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send our partial signature to Boltz
	if err := nursery.boltz.SendFundingAddressSignature(fundingAddress.Id, partial); err != nil {
		return fmt.Errorf("failed to send signature to boltz: %v", err)
	}

	// Update funding address with the swap ID
	if err := nursery.database.SetFundingAddressSwapId(fundingAddress, fundingAddress.SwapId); err != nil {
		logger.Warnf("Failed to update funding address %s with swap ID: %v", fundingAddress.Id, err)
	}

	return nil
}

func (nursery *Nursery) recoverFundingAddresses() error {
	addresses, err := nursery.database.QueryPendingFundingAddresses()
	if err != nil {
		return err
	}

	if len(addresses) == 0 {
		return nil
	}

	var fundingIds []string
	for _, fa := range addresses {
		fundingIds = append(fundingIds, fa.Id)
	}

	return nursery.boltzWs.SubscribeFunding(fundingIds)
}
