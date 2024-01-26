package onchain

import (
	"errors"
	"fmt"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
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

func (onchain *Onchain) GetCurrency(pair boltz.Pair) (*Currency, error) {
	if pair == boltz.PairBtc && onchain.Btc != nil {
		return onchain.Btc, nil
	} else if pair == boltz.PairLiquid && onchain.Liquid != nil {
		return onchain.Liquid, nil
	}
	return nil, errors.New("no currency for pair")
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
		return nil, fmt.Errorf("multiple wallets for currency %s", currency)
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

func (onchain *Onchain) EstimateFee(pair boltz.Pair, confTarget int32) (float64, error) {
	currency, err := onchain.GetCurrency(pair)
	if err != nil {
		return 0, err
	}

	var minFee float64
	if currency == onchain.Liquid {
		minFee = 0.11
	} else if currency == onchain.Btc {
		minFee = 1.1
	}

	if currency.Fees != nil {
		fee, err := currency.Fees.EstimateFee(confTarget)
		if err == nil {
			return math.Max(minFee, fee), nil
		}
		logger.Warn("Fee provider failed. Falling back to wallet fee estimation: " + err.Error())
	}
	wallet, err := onchain.GetAnyWallet(boltz.CurrencyForPair(pair), true)
	if err == nil {
		var fee float64
		fee, err = wallet.EstimateFee(confTarget)
		if err == nil {
			return math.Max(minFee, fee), err
		}
	} else {
		err = fmt.Errorf("no fee provider for %s", pair)
	}
	if err != nil && pair == boltz.PairLiquid {
		logger.Warnf("Could not get fee for liquid, falling back to hardcoded min fee: %s", err.Error())
		return minFee, nil
	}
	return 0, err
}

func (onchain *Onchain) GetTransactionFee(pair boltz.Pair, txId string) (uint64, error) {
	currency, err := onchain.GetCurrency(pair)
	if err != nil {
		return 0, err
	}
	hex, err := currency.Tx.GetTxHex(txId)
	if err != nil {
		return 0, err
	}
	if currency == onchain.Btc {
		transaction, err := boltz.NewBtcTxFromHex(hex)
		if err != nil {
			return 0, err
		}
		var fee uint64
		transactions := make(map[string]*boltz.BtcTransaction)
		for _, input := range transaction.MsgTx().TxIn {
			prevOut := input.PreviousOutPoint
			id := prevOut.Hash.String()
			inputTx, ok := transactions[id]
			if !ok {
				inputTxHex, err := currency.Tx.GetTxHex(id)
				if err != nil {
					return 0, err
				}
				inputTx, err = boltz.NewBtcTxFromHex(inputTxHex)
				if err != nil {
					return 0, errors.New("could not decode input tx: " + err.Error())
				}
				transactions[id] = inputTx
			}
			fee += uint64(inputTx.MsgTx().TxOut[prevOut.Index].Value)
		}
		for _, output := range transaction.MsgTx().TxOut {
			fee -= uint64(output.Value)
		}
		return fee, nil
	} else if currency == onchain.Liquid {
		liquidTx, err := boltz.NewLiquidTxFromHex(hex, nil)
		if err != nil {
			return 0, err
		}
		for _, output := range liquidTx.Outputs {
			out, err := confidential.UnblindOutputWithKey(output, nil)
			if err == nil && len(output.Script) == 0 {
				return out.Value, nil
			}
		}
		return 0, fmt.Errorf("could not find fee output")
	}
	return 0, fmt.Errorf("unknown transaction type")
}

func (onchain *Onchain) GetBlockListener(pair boltz.Pair) BlockListener {
	wallet, err := onchain.GetAnyWallet(boltz.CurrencyForPair(pair), true)
	if err == nil {
		return wallet
	}

	currency, err := onchain.GetCurrency(pair)
	if err != nil {
		return nil
	}

	return currency.Listener
}

func (onchain *Onchain) GetBlockHeight(pair boltz.Pair) (uint32, error) {
	listener := onchain.GetBlockListener(pair)
	if listener != nil {
		return listener.GetBlockHeight()
	}
	return 0, fmt.Errorf("no block listener for pair %s", pair)
}
