package boltz

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
)

const sigHashType = txscript.SigHashDefault

var dummySignature = make([]byte, 64)

type Transaction interface {
	Hash() string
	Serialize() (string, error)
	VSize() uint64
	FindVout(network *Network, address string) (uint32, uint64, error)
}

type OutputDetails struct {
	LockupTransaction Transaction
	Vout              uint32

	// which address to use as the destination for the output
	Address string

	PrivateKey *btcec.PrivateKey

	// Should be set to an empty array in case of a refund
	Preimage []byte

	// Can be zero in case of a claim transaction
	TimeoutBlockHeight uint32

	// taproot only
	SwapTree    *SwapTree
	Cooperative bool

	SwapId   string
	SwapType SwapType
}

func (output *OutputDetails) IsRefund() bool {
	return len(output.Preimage) == 0
}

func NewTxFromHex(currency Currency, hexString string, ourOutputBlindingKey *btcec.PrivateKey) (Transaction, error) {
	if currency == CurrencyLiquid {
		liquidTx, err := NewLiquidTxFromHex(hexString, ourOutputBlindingKey)
		if err == nil {
			return liquidTx, nil
		}
	}

	return NewBtcTxFromHex(hexString)
}

func ConstructTransaction(network *Network, currency Currency, outputs []OutputDetails, satPerVbyte float64, boltzApi *Boltz) (Transaction, uint64, error) {
	var construct func(*Network, []OutputDetails, uint64) (Transaction, error)
	if currency == CurrencyLiquid {
		construct = constructLiquidTransaction
	} else if currency == CurrencyBtc {
		construct = constructBtcTransaction
	} else {
		return nil, 0, fmt.Errorf("invalid pair: %v", currency)
	}

	noFeeTransaction, err := construct(network, outputs, 0)

	if err != nil {
		return nil, 0, err
	}

	fee := uint64(float64(noFeeTransaction.VSize()) * satPerVbyte)
	transaction, err := construct(network, outputs, fee)
	if err != nil {
		return nil, 0, err
	}

	for i, output := range outputs {
		if output.Cooperative {
			if boltzApi == nil {
				return nil, 0, errors.New("boltzApi is required for cooperative transactions")
			}
			session, err := NewSigningSession(outputs[i].SwapTree)
			if err != nil {
				return nil, 0, fmt.Errorf("could not initialize signing session: %w", err)
			}

			serialized, err := transaction.Serialize()
			if err != nil {
				return nil, 0, fmt.Errorf("could not serialize transaction: %w", err)
			}

			pubNonce := session.PublicNonce()
			var signature *PartialSignature
			if output.SwapType == ReverseSwap {
				signature, err = boltzApi.ClaimReverseSwap(ClaimReverseSwapRequest{
					Id:          output.SwapId,
					Transaction: serialized,
					PubNonce:    pubNonce[:],
					Index:       i,
					Preimage:    output.Preimage,
				})
			} else {
				signature, err = boltzApi.RefundSwap(RefundSwapRequest{
					Id:          output.SwapId,
					Transaction: serialized,
					PubNonce:    pubNonce[:],
					Index:       i,
				})
			}
			if err != nil {
				return nil, 0, fmt.Errorf("could not get partial signature from boltz: %w", err)
			}

			if err := session.Finalize(transaction, outputs, network, signature); err != nil {
				return nil, 0, fmt.Errorf("could not finalize signing session: %w", err)
			}
		}
	}

	return transaction, fee, err
}
