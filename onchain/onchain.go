package onchain

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/vulpemventures/go-elements/confidential"
)

type BlockEpoch struct {
	Height uint32
}

type Balance struct {
	Total       uint64
	Confirmed   uint64
	Unconfirmed uint64
}

type WalletInfo struct {
	Id       int64
	Name     string
	Currency boltz.Currency
	Readonly bool
	EntityId *int64
}

func (info *WalletChecker) String() string {
	return fmt.Sprintf("%s (id: %d)", info.Name, info.Id)
}

type BlockListener interface {
	RegisterBlockListener(channel chan<- *BlockEpoch, stop <-chan bool) error
	GetBlockHeight() (uint32, error)
}

type FeeProvider interface {
	EstimateFee(confTarget int32) (float64, error)
}

type TxProvider interface {
	GetTxHex(txId string) (string, error)
}

type AddressProvider interface {
	IsUsed(address string) (bool, error)
}

type Wallet interface {
	FeeProvider
	BlockListener
	NewAddress() (string, error)
	SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error)
	Ready() bool
	GetBalance() (*Balance, error)
	GetWalletInfo() WalletInfo
}

type Currency struct {
	Listener BlockListener
	Fees     FeeProvider
	Tx       TxProvider
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

func (onchain *Onchain) RemoveWallet(id int64) {
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

type WalletChecker struct {
	Id       *int64
	Currency boltz.Currency
	Name     string
	Readonly bool
	EntityId *int64
}

func (checker *WalletChecker) Allowed(wallet Wallet) bool {
	info := wallet.GetWalletInfo()
	return wallet.Ready() &&
		(checker.Id == nil || info.Id == *checker.Id) &&
		(info.Currency == checker.Currency || checker.Currency == "") &&
		(info.Name == checker.Name || checker.Name == "") &&
		(!info.Readonly || checker.Readonly) &&
		(info.EntityId == checker.EntityId || (info.EntityId != nil && checker.EntityId != nil && *info.EntityId == *checker.EntityId))
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

func (onchain *Onchain) GetWalletById(id int64) (wallet Wallet, err error) {
	return onchain.GetAnyWallet(WalletChecker{Id: &id})
}

func (onchain *Onchain) EstimateFee(currency boltz.Currency, confTarget int32) (float64, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return 0, err
	}

	var minFee float64
	if chain == onchain.Liquid {
		minFee = 0.11
	} else if chain == onchain.Btc {
		minFee = 1.1
	}

	if chain.Fees != nil {
		fee, err := chain.Fees.EstimateFee(confTarget)
		if err == nil {
			return math.Max(minFee, fee), nil
		}
		logger.Warn("Fee provider failed. Falling back to wallet fee estimation: " + err.Error())
	}
	wallet, err := onchain.GetAnyWallet(WalletChecker{Currency: currency})
	if err == nil {
		var fee float64
		fee, err = wallet.EstimateFee(confTarget)
		if err == nil {
			return math.Max(minFee, fee), err
		}
	} else {
		err = fmt.Errorf("no fee provider for %s", currency)
	}
	if err != nil && currency == boltz.CurrencyLiquid {
		logger.Warnf("Could not get fee for liquid, falling back to hardcoded min fee: %s", err.Error())
		return minFee, nil
	}
	return 0, err
}

func (onchain *Onchain) GetTransaction(currency boltz.Currency, txId string, ourOutputBlindingKey *btcec.PrivateKey) (boltz.Transaction, error) {
	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil, err
	}
	hex, err := chain.Tx.GetTxHex(txId)
	if err != nil {
		return nil, err
	}

	return boltz.NewTxFromHex(currency, hex, ourOutputBlindingKey)
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

func (onchain *Onchain) GetBlockListener(currency boltz.Currency) BlockListener {
	wallet, err := onchain.GetAnyWallet(WalletChecker{Currency: currency, Readonly: true})
	if err == nil {
		return wallet
	}

	chain, err := onchain.GetCurrency(currency)
	if err != nil {
		return nil
	}

	return chain.Listener
}

func (onchain *Onchain) GetBlockHeight(currency boltz.Currency) (uint32, error) {
	listener := onchain.GetBlockListener(currency)
	if listener != nil {
		return listener.GetBlockHeight()
	}
	return 0, fmt.Errorf("no block listener for currency %s", currency)
}
