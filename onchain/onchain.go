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

type BlockListener interface {
	RegisterBlockListener(channel chan *BlockEpoch) error
	GetBlockHeight() (uint32, error)
}

type FeeProvider interface {
	EstimateFee(confTarget int32) (float64, error)
}

type TxProvider interface {
	GetTxHex(txId string) (string, error)
}

type Wallet interface {
	FeeProvider
	BlockListener
	NewAddress() (string, error)
	SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error)
	Ready() bool
}

type Currency struct {
	Listener BlockListener
	Fees     FeeProvider
	Wallet   Wallet
	Tx       TxProvider
}

type Onchain struct {
	Btc     *Currency
	Liquid  *Currency
	Network *boltz.Network
}

func (onchain *Onchain) GetCurrency(pair boltz.Pair) (*Currency, error) {
	if pair == boltz.PairBtc && onchain.Btc != nil {
		return onchain.Btc, nil
	} else if pair == boltz.PairLiquid && onchain.Liquid != nil {
		return onchain.Liquid, nil
	}
	return nil, errors.New("no currency for pair")
}

func (onchain *Onchain) GetWallet(pair boltz.Pair) (wallet Wallet, err error) {
	currency, err := onchain.GetCurrency(pair)
	if err != nil {
		return nil, err
	}
	if currency.Wallet == nil {
		return nil, fmt.Errorf("no wallet for pair %v", pair)
	}
	if !currency.Wallet.Ready() {
		return nil, fmt.Errorf("wallet for pair not ready %v", pair)
	}
	return currency.Wallet, nil
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
			return math.Max(minFee, fee), err
		}
		logger.Warn("Fee provider failed. Falling back to wallet fee estimation: " + err.Error())
	}
	if currency.Wallet != nil && currency.Wallet.Ready() {
		fee, err := currency.Wallet.EstimateFee(confTarget)
		return math.Max(minFee, fee), err
	}
	return 0, errors.New("no fee provider for currency")
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
	wallet, err := onchain.GetWallet(pair)
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
