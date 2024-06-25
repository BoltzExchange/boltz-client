package onchain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
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

type WalletInfo struct {
	Id       Id
	Name     string
	Currency boltz.Currency
	Readonly bool
	EntityId Id
}

type WalletChecker struct {
	Id            *Id
	Name          *string
	Currency      boltz.Currency
	AllowReadonly bool
	EntityId      *Id
}

type BlockProvider interface {
	RegisterBlockListener(ctx context.Context, channel chan<- *BlockEpoch) error
	GetBlockHeight() (uint32, error)
	EstimateFee(confTarget int32) (float64, error)
	Shutdown()
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
	SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error)
	Ready() bool
	GetBalance() (*Balance, error)
	GetWalletInfo() WalletInfo
}

type Currency struct {
	Blocks BlockProvider
	Tx     TxProvider
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
	entityId := walletChecker.EntityId == nil || info.EntityId == *walletChecker.EntityId
	return wallet.Ready() && id && currency && name && readonly && entityId
}

func (onchain *Onchain) GetAnyWallet(checker WalletChecker) (Wallet, error) {
	for _, wallet := range onchain.Wallets {
		if checker.Allowed(wallet) {
			return wallet, nil
		}
	}
	return nil, fmt.Errorf("no wallet found for checker: %+v", checker)
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

func (onchain *Onchain) GetWalletById(id Id) (wallet Wallet, err error) {
	return onchain.GetAnyWallet(WalletChecker{Id: &id})
}

func (onchain *Onchain) EstimateFee(currency boltz.Currency, confTarget int32) (float64, error) {
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

	fee, err := chain.Blocks.EstimateFee(confTarget)
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

func (onchain *Onchain) GetBlockProvider(currency boltz.Currency) BlockProvider {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil
	}

	return chain.Blocks
}

func (onchain *Onchain) GetBlockHeight(currency boltz.Currency) (uint32, error) {
	listener := onchain.GetBlockProvider(currency)
	if listener != nil {
		return listener.GetBlockHeight()
	}
	return 0, fmt.Errorf("no block listener for currency %s", currency)
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

func (onchain *Onchain) Shutdown() {
	onchain.OnWalletChange.Close()
	onchain.Btc.Blocks.Shutdown()
	onchain.Liquid.Blocks.Shutdown()
}
