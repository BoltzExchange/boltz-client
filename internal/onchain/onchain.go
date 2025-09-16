package onchain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
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

type FeeProvider interface {
	EstimateFee() (float64, error)
}

type BlockProvider interface {
	FeeProvider
	RegisterBlockListener(ctx context.Context, channel chan<- *BlockEpoch) error
	GetBlockHeight() (uint32, error)
	Disconnect()
	GetUnspentOutputs(address string) ([]*Output, error)
}

type TxProvider interface {
	GetRawTransaction(txId string) (string, error)
	BroadcastTransaction(txHex string) (string, error)
	IsTransactionConfirmed(txId string) (bool, error)
}

type AddressProvider interface {
	IsUsed(address string) (bool, error)
}

type ElectrumOptions struct {
	Url string
	SSL bool
}

type ElectrumConfig struct {
	Btc    ElectrumOptions
	Liquid ElectrumOptions
}

var RegtestElectrumConfig = ElectrumConfig{
	Btc:    ElectrumOptions{Url: "localhost:19001"},
	Liquid: ElectrumOptions{Url: "localhost:19002"},
}

type Currency struct {
	Blocks BlockProvider
	Tx     TxProvider
	FeeFallback FeeProvider

	blockHeight uint32
}

type Onchain struct {
	Btc            *Currency
	Liquid         *Currency
	Network        *boltz.Network
	Wallets        []Wallet
	OnWalletChange *utils.ChannelForwarder[[]Wallet]
}

func (onchain *Onchain) Init() {
	onchain.OnWalletChange = utils.ForwardChannel(make(chan []Wallet), 0, false)
}

func (onchain *Onchain) AddWallet(wallet Wallet) {
	onchain.Wallets = append(onchain.Wallets, wallet)
	onchain.OnWalletChange.Send(onchain.Wallets)
}

func (onchain *Onchain) RemoveWallet(id Id) {
	onchain.Wallets = slices.DeleteFunc(onchain.Wallets, func(current Wallet) bool {
		return current.GetWalletInfo().Id == id
	})
	onchain.OnWalletChange.Send(onchain.Wallets)
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
	boltz.CurrencyBtc:   2,
}

func (onchain *Onchain) EstimateFee(currency boltz.Currency) (float64, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}

	minFee := FeeFloor[currency]

	fee, err := chain.Blocks.EstimateFee()
	if err != nil {
		logger.Warnf("Could not get fee for %s from default provider: %s", currency, err.Error())
		if chain.FeeFallback != nil {
			logger.Infof("Using fallback provider for %s", currency)
			fee, err = chain.FeeFallback.EstimateFee()
		} else {
			return 0, fmt.Errorf("could not get fee for %s: %w", currency, err)
		}
	}
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
		hex, err := chain.Tx.GetRawTransaction(txId)
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

const retryInterval = 15 * time.Second

func (onchain *Onchain) RegisterBlockListener(ctx context.Context, currency boltz.Currency) *utils.ChannelForwarder[*BlockEpoch] {
	chain, err := onchain.GetCurrency(currency)
	if err != nil || chain.Blocks == nil {
		logger.Warnf("no block listener for %s", currency)
		return nil
	}

	logger.Infof("Connecting to block %s epoch stream", currency)
	blocks := make(chan *BlockEpoch)
	blockNotifier := utils.ForwardChannel(make(chan *BlockEpoch), 0, false)

	go func() {
		defer func() {
			blockNotifier.Close()
			logger.Debugf("Closed block listener for %s", currency)
		}()
		for {
			err := chain.Blocks.RegisterBlockListener(ctx, blocks)
			if err != nil && ctx.Err() == nil {
				logger.Errorf("Lost connection to %s block epoch stream: %s", currency, err.Error())
				logger.Infof("Retrying connection in %s", retryInterval)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
			}
		}
	}()

	go func() {
		for block := range blocks {
			chain.blockHeight = block.Height
			blockNotifier.Send(block)
		}
	}()

	return blockNotifier
}

func (onchain *Onchain) GetBlockProvider(currency boltz.Currency) BlockProvider {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil
	}

	return chain.Blocks
}

func (onchain *Onchain) GetBlockHeight(currency boltz.Currency) (uint32, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}
	if chain.blockHeight == 0 {
		chain.blockHeight, err = chain.Blocks.GetBlockHeight()
	}
	return chain.blockHeight, err
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

	return chain.Tx.BroadcastTransaction(serialized)
}

func (onchain *Onchain) IsTransactionConfirmed(currency boltz.Currency, txId string, retry bool) (bool, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return false, err
	}

	retryCount := 5
	for {
		confirmed, err := chain.Tx.IsTransactionConfirmed(txId)
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
	return chain.Blocks.GetUnspentOutputs(address)
}

type OutputArgs struct {
	TransactionId    string
	Currency         boltz.Currency
	Address          string
	BlindingKey      *btcec.PrivateKey
	ExpectedAmount   uint64
	RequireConfirmed bool
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
	if info.RequireConfirmed {
		confirmed, err := onchain.IsTransactionConfirmed(info.Currency, info.TransactionId, false)
		if !errors.Is(err, errors.ErrUnsupported) {
			if err != nil {
				return nil, errors.New("Could not check if lockup transaction is confirmed: " + err.Error())
			}
			if !confirmed {
				return nil, ErrNotConfirmed
			}
		}
	}

	return &OutputResult{
		Transaction: lockupTransaction,
		Vout:        vout,
		Value:       value,
	}, nil
}

func (onchain *Onchain) Disconnect() {
	onchain.OnWalletChange.Close()
	onchain.Btc.Blocks.Disconnect()
	onchain.Liquid.Blocks.Disconnect()
	var wg sync.WaitGroup
	wg.Add(len(onchain.Wallets))

	for _, wallet := range onchain.Wallets {
		wallet := wallet
		go func() {
			if err := wallet.Disconnect(); err != nil {
				logger.Errorf("Error shutting down wallet: %s", err.Error())
			}
			wg.Done()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-time.After(10 * time.Second):
		logger.Warnf("Wallet disconnect timed out")
	case <-done:
	}
}
