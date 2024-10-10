package onchain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/vulpemventures/go-elements/confidential"
)

type Id = uint64

type BlockEpoch struct {
	Height uint32
}

type Balance struct {
	Total       uint64
	Confirmed   uint64
	Unconfirmed uint64
}

type TransactionOutput struct {
	Address      string
	Amount       uint64
	IsOurAddress bool
}

type WalletTransaction struct {
	Id              string
	Timestamp       time.Time
	Outputs         []TransactionOutput
	BlockHeight     uint32
	BalanceChange   int64
	IsConsolidation bool
}

type WalletInfo struct {
	Id       Id
	Name     string
	Currency boltz.Currency
	Readonly bool
	TenantId Id
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

type BlockProvider interface {
	RegisterBlockListener(ctx context.Context, channel chan<- *BlockEpoch) error
	GetBlockHeight() (uint32, error)
	EstimateFee() (float64, error)
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

type Wallet interface {
	NewAddress() (string, error)
	SendToAddress(address string, amount uint64, satPerVbyte float64, sendAll bool) (string, error)
	Ready() bool
	GetBalance() (*Balance, error)
	GetWalletInfo() WalletInfo
	Disconnect() error
	GetTransactions(limit, offset uint64) ([]*WalletTransaction, error)
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

func (onchain *Onchain) EstimateFee(currency boltz.Currency, allowLowball bool) (float64, error) {
	if currency == boltz.CurrencyLiquid && onchain.Network == boltz.MainNet && allowLowball {
		if boltzProvider, ok := onchain.Liquid.Tx.(*BoltzTxProvider); ok {
			return boltzProvider.GetFeeEstimation(boltz.CurrencyLiquid)
		}
	}
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}

	var minFee float64
	if chain == onchain.Liquid {
		minFee = 0.1
	} else if chain == onchain.Btc {
		minFee = 1
	}

	fee, err := chain.Blocks.EstimateFee()
	if err != nil && currency == boltz.CurrencyLiquid {
		logger.Warnf("Could not get fee for liquid, falling back to hardcoded min fee: %s", err.Error())
		return minFee, nil
	}
	return math.Max(minFee, fee), err
}

func (onchain *Onchain) GetTransaction(currency boltz.Currency, txId string, ourOutputBlindingKey *btcec.PrivateKey) (boltz.Transaction, error) {
	if txId == "" {
		return nil, errors.New("empty transaction id")
	}
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil, err
	}
	retry := 5
	for {
		// Check if the transaction is in the mempool
		hex, err := chain.Tx.GetRawTransaction(txId)
		if err != nil {
			if retry == 0 {
				return nil, err
			}
			retry--
			retryInterval := 10 * time.Second
			logger.Debugf("Transaction %s not found yet, retrying in %s", txId, retryInterval)
			<-time.After(retryInterval)
		} else {
			return boltz.NewTxFromHex(currency, hex, ourOutputBlindingKey)
		}
	}
}

func (onchain *Onchain) GetTransactionFee(currency boltz.Currency, txId string) (uint64, error) {
	transaction, err := onchain.GetTransaction(currency, txId, nil)
	if err != nil {
		return 0, err
	}
	if btcTransaction, ok := transaction.(*boltz.BtcTransaction); ok {
		var fee uint64
		transactions := make(map[string]*boltz.BtcTransaction)
		for _, input := range btcTransaction.MsgTx().TxIn {
			prevOut := input.PreviousOutPoint
			id := prevOut.Hash.String()
			_, ok := transactions[id]
			if !ok {
				transaction, err := onchain.GetTransaction(currency, id, nil)
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

func (onchain *Onchain) IsTransactionConfirmed(currency boltz.Currency, txId string) (bool, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return false, err
	}

	retry := 5
	if onchain.Network == boltz.Regtest {
		retry = 0
	}
	for {
		confirmed, err := chain.Tx.IsTransactionConfirmed(txId)
		if err != nil || !confirmed {
			if retry == 0 {
				return false, err
			}
			retry--
			retryInterval := 10 * time.Second
			logger.Debugf("Transaction %s not confirmed yet, retrying in %s", txId, retryInterval)
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
