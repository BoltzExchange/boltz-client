package onchain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/vulpemventures/go-elements/confidential"
)

type Id = uint64

type BlockEpoch struct {
	Height uint32
}

type WalletChecker struct {
	Id            *Id
	Name          *string
	Currency      boltz.Currency
	AllowReadonly bool
	TenantId      *Id
}

type Output struct {
	TxId  string
	Value uint64
}

type ChainProvider interface {
	EstimateFee() (float64, error)
	GetBlockHeight() (uint32, error)
	GetRawTransaction(txId string) (string, error)
	BroadcastTransaction(txHex string) (string, error)
	IsTransactionConfirmed(txId string) (bool, error)
	GetUnspentOutputs(address string) ([]*Output, error)
	Disconnect()
}

type ElectrumOptions struct {
	Url string
	SSL bool
}

func (e *ElectrumOptions) String() string {
	if e.SSL {
		return fmt.Sprintf("ssl://%s", e.Url)
	}
	return fmt.Sprintf("tcp://%s", e.Url)
}

type ElectrumConfig struct {
	Btc    *ElectrumOptions
	Liquid *ElectrumOptions
}

var RegtestElectrumConfig = ElectrumConfig{
	Btc:    &ElectrumOptions{Url: "localhost:19001"},
	Liquid: &ElectrumOptions{Url: "localhost:19002"},
}

var DefaultWalletSyncIntervals = map[boltz.Currency]time.Duration{
	boltz.CurrencyBtc:    3 * time.Minute,
	boltz.CurrencyLiquid: 1 * time.Minute,
}

type Currency struct {
	Chain       ChainProvider
	blockHeight atomic.Uint32
}

type Onchain struct {
	Btc                 *Currency
	Liquid              *Currency
	Network             *boltz.Network
	Wallets             []Wallet
	OnWalletChange      *utils.ChannelForwarder[[]Wallet]
	WalletSyncIntervals map[boltz.Currency]time.Duration

	syncWait   sync.WaitGroup
	syncCtx    context.Context
	syncCancel func()
}

func (onchain *Onchain) Init() {
	onchain.OnWalletChange = utils.ForwardChannel(make(chan []Wallet), 0, false)
	onchain.syncCtx, onchain.syncCancel = context.WithCancel(context.Background())
	if onchain.WalletSyncIntervals == nil {
		onchain.WalletSyncIntervals = DefaultWalletSyncIntervals
	}
}

func (onchain *Onchain) AddWallet(wallet Wallet) {
	onchain.Wallets = append(onchain.Wallets, wallet)
	onchain.OnWalletChange.Send(onchain.Wallets)
	onchain.startSyncLoop(wallet)
}

func (onchain *Onchain) RemoveWallet(id Id) {
	onchain.Wallets = slices.DeleteFunc(onchain.Wallets, func(current Wallet) bool {
		return current.GetWalletInfo().Id == id
	})
	onchain.OnWalletChange.Send(onchain.Wallets)
}

func (onchain *Onchain) startSyncLoop(wallet Wallet) {
	onchain.syncWait.Add(1)
	go func() {
		defer onchain.syncWait.Done()
		for {
			// avoid traffic spikes if a lot of wallets are using the same backend
			currency := wallet.GetWalletInfo().Currency
			interval, ok := onchain.WalletSyncIntervals[currency]
			if !ok {
				logger.Warnf("No sync interval found for currency %s", currency)
				return
			}
			sleep := time.Duration(float64(interval) * (0.75 + rand.Float64()*0.5))
			select {
			case <-onchain.syncCtx.Done():
				if err := wallet.Disconnect(); err != nil {
					info := wallet.GetWalletInfo()
					logger.Errorf("Error shutting down wallet %s: %s", info.String(), err.Error())
				}
				return
			case <-time.After(sleep):
				if slices.Contains(onchain.Wallets, wallet) {
					if err := wallet.Sync(); err != nil {
						info := wallet.GetWalletInfo()
						logger.Errorf("Sync for wallet %d failed: %v", info.Id, err)
					}
				} else {
					return
				}
			}
		}
	}()
}

func (onchain *Onchain) GetCurrency(currency boltz.Currency) (*Currency, error) {
	if currency == boltz.CurrencyBtc && onchain.Btc != nil {
		return onchain.Btc, nil
	} else if currency == boltz.CurrencyLiquid && onchain.Liquid != nil {
		return onchain.Liquid, nil
	}
	return nil, errors.New("invalid currency")
}

func (walletChecker *WalletChecker) Allowed(wallet Wallet) bool {
	info := wallet.GetWalletInfo()
	id := walletChecker.Id == nil || info.Id == *walletChecker.Id
	currency := info.Currency == walletChecker.Currency || walletChecker.Currency == ""
	name := walletChecker.Name == nil || info.Name == *walletChecker.Name
	readonly := !info.Readonly || walletChecker.AllowReadonly
	tenantId := walletChecker.TenantId == nil || info.TenantId == *walletChecker.TenantId
	return wallet.Ready() && id && currency && name && readonly && tenantId
}

func (onchain *Onchain) GetAnyWallet(checker WalletChecker) (Wallet, error) {
	for _, wallet := range onchain.Wallets {
		if checker.Allowed(wallet) {
			return wallet, nil
		}
	}
	var msg string
	if checker.AllowReadonly {
		msg += "readonly "
	}
	msg += "wallet with"
	if checker.Id != nil {
		msg += fmt.Sprintf(" id: %d ", *checker.Id)
	}
	if checker.Name != nil {
		msg += fmt.Sprintf(" name: %s ", *checker.Name)
	}
	if checker.Currency != "" {
		msg += fmt.Sprintf(" currency: %s ", checker.Currency)
	}
	msg += "not found"
	return nil, errors.New(msg)
}

func (onchain *Onchain) GetWallets(checker WalletChecker) []Wallet {
	var wallets []Wallet
	for _, wallet := range onchain.Wallets {
		if checker.Allowed(wallet) {
			wallets = append(wallets, wallet)
		}
	}
	return wallets
}

var FeeFloor = map[boltz.Currency]float64{
	boltz.CurrencyLiquid: 0.1,
	boltz.CurrencyBtc:    2,
}

func (onchain *Onchain) EstimateFee(currency boltz.Currency) (float64, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}

	minFee := FeeFloor[currency]

	fee, err := chain.Chain.EstimateFee()
	return math.Max(minFee, fee), err
}

func (onchain *Onchain) GetTransaction(currency boltz.Currency, txId string, ourOutputBlindingKey *btcec.PrivateKey, retry bool) (boltz.Transaction, error) {
	if txId == "" {
		return nil, errors.New("empty transaction id")
	}
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil, err
	}
	retryCount := 5
	for {
		// Check if the transaction is in the mempool
		hex, err := chain.Chain.GetRawTransaction(txId)
		if err != nil {
			if retryCount == 0 || !retry {
				return nil, err
			}
			retryCount--
			retryInterval := 10 * time.Second
			logger.Debugf("Transaction %s not found yet, retrying in %s", txId, retryInterval)
			<-time.After(retryInterval)
		} else {
			return boltz.NewTxFromHex(currency, hex, ourOutputBlindingKey)
		}
	}
}

func (onchain *Onchain) GetTransactionFee(transaction boltz.Transaction) (uint64, error) {
	if btcTransaction, ok := transaction.(*boltz.BtcTransaction); ok {
		var fee uint64
		transactions := make(map[string]*boltz.BtcTransaction)
		for _, input := range btcTransaction.MsgTx().TxIn {
			prevOut := input.PreviousOutPoint
			id := prevOut.Hash.String()
			_, ok := transactions[id]
			if !ok {
				transaction, err := onchain.GetTransaction(boltz.CurrencyBtc, id, nil, false)
				if err != nil {
					return 0, errors.New("could not fetch input tx: " + err.Error())
				}
				transactions[id] = transaction.(*boltz.BtcTransaction)
			}
			fee += uint64(transactions[id].MsgTx().TxOut[prevOut.Index].Value)
		}
		for _, output := range btcTransaction.MsgTx().TxOut {
			fee -= uint64(output.Value)
		}
		return fee, nil
	} else if liquidTransaction, ok := transaction.(*boltz.LiquidTransaction); ok {
		for _, output := range liquidTransaction.Outputs {
			out, err := confidential.UnblindOutputWithKey(output, nil)
			if err == nil && len(output.Script) == 0 {
				return out.Value, nil
			}
		}
		return 0, fmt.Errorf("could not find fee output")
	}
	return 0, fmt.Errorf("unknown transaction type")
}

func (onchain *Onchain) RegisterBlockListener(ctx context.Context, currency boltz.Currency) *utils.ChannelForwarder[*BlockEpoch] {
	chain, err := onchain.GetCurrency(currency)
	if err != nil || chain.Chain == nil {
		logger.Warnf("no block listener for %s", currency)
		return nil
	}

	logger.Infof("Connecting to block %s epoch stream", currency)
	blockNotifier := utils.ForwardChannel(make(chan *BlockEpoch), 0, false)

	go func() {
		blockTime := time.Duration(boltz.GetBlockTime(currency) * float64(time.Minute))
		tickerDuration := blockTime / 10
		if onchain.Network == boltz.Regtest {
			tickerDuration = 1 * time.Second
		}
		ticker := time.NewTicker(tickerDuration)
		defer ticker.Stop()
		defer blockNotifier.Close()
		var prevHeight uint32
		for {
			select {
			case <-ctx.Done():
				logger.Debugf("Stopped block listener for %s", currency)
				return
			case <-ticker.C:
				height, err := chain.Chain.GetBlockHeight()
				if err != nil {
					logger.Errorf("Could not get block height for %s: %s", currency, err.Error())
					continue
				}
				if height != prevHeight {
					prevHeight = height
					chain.blockHeight.Store(height)
					block := &BlockEpoch{Height: height}
					blockNotifier.Send(block)
				}
			}
		}
	}()

	return blockNotifier
}

func (onchain *Onchain) GetBlockProvider(currency boltz.Currency) ChainProvider {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil
	}

	return chain.Chain
}

func (onchain *Onchain) GetBlockHeight(currency boltz.Currency) (uint32, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}
	height := chain.blockHeight.Load()
	if height == 0 {
		height, err = chain.Chain.GetBlockHeight()
		if err != nil {
			return 0, err
		}
		chain.blockHeight.Store(height)
	}
	return height, nil
}

func (onchain *Onchain) BroadcastTransaction(transaction boltz.Transaction) (string, error) {
	chain, err := onchain.GetCurrency(boltz.TransactionCurrency(transaction))
	if err != nil {
		return "", err
	}

	serialized, err := transaction.Serialize()
	if err != nil {
		return "", err
	}

	return chain.Chain.BroadcastTransaction(serialized)
}

func (onchain *Onchain) IsTransactionConfirmed(currency boltz.Currency, txId string, retry bool) (bool, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return false, err
	}

	retryCount := 5
	for {
		confirmed, err := chain.Chain.IsTransactionConfirmed(txId)
		if err != nil {
			if errors.Is(err, errors.ErrUnsupported) {
				logger.Warnf("Transaction confirmation check not supported for %s", currency)
				return false, err
			}
			if retryCount == 0 || !retry {
				return false, err
			}
			retryCount--
			retryInterval := 10 * time.Second
			logger.Debugf("Transaction %s not yet in mempool, retrying in %s", txId, retryInterval)
			<-time.After(retryInterval)
		} else {
			return confirmed, nil
		}
	}
}

func (onchain *Onchain) GetUnspentOutputs(currency boltz.Currency, address string) ([]*Output, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil, err
	}
	return chain.Chain.GetUnspentOutputs(address)
}

type OutputArgs struct {
	TransactionId  string
	Currency       boltz.Currency
	Address        string
	BlindingKey    *btcec.PrivateKey
	ExpectedAmount uint64
}

type OutputResult struct {
	Transaction boltz.Transaction
	Vout        uint32
	Value       uint64
}

var ErrNotConfirmed = errors.New("lockup transaction not confirmed")

func (onchain *Onchain) FindOutput(info OutputArgs) (*OutputResult, error) {
	lockupTransaction, err := onchain.GetTransaction(info.Currency, info.TransactionId, info.BlindingKey, true)
	if err != nil {
		return nil, fmt.Errorf("could not decode lockup transaction: %w", err)
	}

	vout, value, err := lockupTransaction.FindVout(onchain.Network, info.Address)
	if err != nil {
		return nil, err
	}

	if info.ExpectedAmount != 0 && value < info.ExpectedAmount {
		return nil, fmt.Errorf("locked up less onchain coins than expected: %d < %d", value, info.ExpectedAmount)
	}

	return &OutputResult{
		Transaction: lockupTransaction,
		Vout:        vout,
		Value:       value,
	}, nil
}

func (onchain *Onchain) Disconnect() {
	onchain.OnWalletChange.Close()
	onchain.Btc.Chain.Disconnect()
	onchain.Liquid.Chain.Disconnect()
	onchain.syncCancel()

	done := make(chan struct{})
	go func() {
		onchain.syncWait.Wait()
		close(done)
	}()
	select {
	case <-time.After(10 * time.Second):
		logger.Warnf("Wallet disconnect timed out")
	case <-done:
	}
}
