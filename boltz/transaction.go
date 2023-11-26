package boltz

import (
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
)

type OutputType int

const (
	SegWit OutputType = iota
	Compatibility
	Legacy
)

type Transaction interface {
	Hash() string
	Serialize() (string, error)
	VSize() uint64
	FindVout(network *Network, address string) (uint32, uint64, error)
}

type OutputDetails struct {
	LockupTransaction Transaction
	Vout              uint32
	OutputType        OutputType

	RedeemScript []byte
	PrivateKey   *btcec.PrivateKey

	// Should be set to an empty array in case of a refund
	Preimage []byte

	// Can be zero in case of a claim transaction
	TimeoutBlockHeight uint32
}

func NewTxFromHex(hexString string, ourOutputBlindingKey *btcec.PrivateKey) (Transaction, error) {
	if ourOutputBlindingKey != nil {
		liquidTx, err := NewLiquidTxFromHex(hexString, ourOutputBlindingKey)
		if err == nil {
			return liquidTx, nil
		}
	}

	return NewBtcTxFromHex(hexString)
}

func ConstructTransaction(pair Pair, network *Network, outputs []OutputDetails, outputAddress string, satPerVbyte float64) (Transaction, uint64, error) {
	var construct func(*Network, []OutputDetails, string, uint64) (Transaction, error)
	if pair == PairLiquid {
		construct = constructLiquidTransaction
	} else if pair == PairBtc {
		construct = constructBtcTransaction
	} else {
		return nil, 0, fmt.Errorf("invalid pair: %v", pair)
	}

	noFeeTransaction, err := construct(network, outputs, outputAddress, 0)

	if err != nil {
		return nil, 0, err
	}

	fee := uint64(float64(noFeeTransaction.VSize()) * satPerVbyte)
	transaction, err := construct(network, outputs, outputAddress, fee)
	return transaction, fee, err
}
