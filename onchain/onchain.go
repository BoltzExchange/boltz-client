package onchain

import (
	"errors"
	"fmt"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
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
	Readonly() bool
	Name() string
	Currency() boltz.Currency
	GetBalance() (*Balance, error)
}

type Currency struct {
	Listener BlockListener
	Fees     FeeProvider
	Tx       TxProvider
}

type Onchain struct {
	Btc     *Currency
	Liquid  *Currency
	Network *boltz.Network
	Wallets []Wallet
}

func (onchain *Onchain) GetCurrency(currency boltz.Currency) (*Currency, error) {
	if currency == boltz.CurrencyBtc && onchain.Btc != nil {
		return onchain.Btc, nil
	} else if currency == boltz.CurrencyLiquid && onchain.Liquid != nil {
		return onchain.Liquid, nil
	}
	return nil, errors.New("invalid currency")
}

func (onchain *Onchain) getWallet(name string, currency boltz.Currency, readonly bool, allowMultiple bool) (Wallet, error) {
	if onchain.Wallets == nil {
		return nil, fmt.Errorf("no wallets")
	}
	var found []Wallet
	for _, wallet := range onchain.Wallets {
		if (wallet.Currency() == currency || currency == "") && (!wallet.Readonly() || readonly) && (wallet.Name() == name || name == "") {
			found = append(found, wallet)
		}
	}

	errMessage := "wallet"
	if name != "" {
		errMessage += " " + name
	}
	if currency != "" {
		errMessage += " for " + string(currency)
	}

	if len(found) == 0 {
		// check if the specific wallet we are looking for is readonly, so we can display a better error in that case
		if !readonly && name != "" {
			other, _ := onchain.getWallet(name, currency, true, allowMultiple)
			if other != nil {
				return nil, fmt.Errorf("%v is read only", errMessage)
			}
		}
		return nil, fmt.Errorf("no %v", errMessage)
	} else if len(found) > 1 && !allowMultiple {
		return nil, fmt.Errorf("multiple wallets for currency %s, specify a specific one", currency)
	}
	result := found[0]
	if !result.Ready() {
		return nil, fmt.Errorf("%v not ready", errMessage)
	}
	return result, nil
}

func (onchain *Onchain) GetWallet(name string, currency boltz.Currency, readonly bool) (wallet Wallet, err error) {
	return onchain.getWallet(name, currency, readonly, false)
}

func (onchain *Onchain) GetAnyWallet(currency boltz.Currency, readonly bool) (wallet Wallet, err error) {
	return onchain.getWallet("", currency, readonly, true)
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
	wallet, err := onchain.GetAnyWallet(currency, true)
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
	wallet, err := onchain.GetAnyWallet(currency, true)
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
