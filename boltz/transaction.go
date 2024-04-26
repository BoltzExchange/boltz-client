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
	VoutValue(vout uint32) (uint64, error)
}

type OutputDetails struct {
	LockupTransaction Transaction
	Vout              uint32
	// the absolute fee to pay for this output
	Fee uint64

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
	// swap tree of server lockup transaction, required when cooperatively claiming a chain swap
	RefundSwapTree *SwapTree

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

type OutputResult struct {
	Err error
	Fee uint64
}

func ConstructTransaction(network *Network, currency Currency, outputs []OutputDetails, satPerVbyte float64, boltzApi *Boltz) (Transaction, map[string]OutputResult, error) {
	construct := constructBtcTransaction
	if currency == CurrencyLiquid {
		construct = constructLiquidTransaction
	}
	results := make(map[string]OutputResult, len(outputs))

	getOutValues := func(fee uint64) map[string]uint64 {
		outValues := make(map[string]uint64)

		outLen := uint64(len(outputs))
		feePerOutput := fee / outLen
		feeRemainder := fee % outLen

		for i := range outputs {
			output := &outputs[i]
			output.Fee = feePerOutput + feeRemainder
			feeRemainder = 0

			value, err := output.LockupTransaction.VoutValue(output.Vout)
			if err == nil {
				if value < output.Fee {
					err = fmt.Errorf("value less than fee: %d < %d", value, output.Fee)
				}
			}

			if err != nil {
				results[output.SwapId] = OutputResult{Err: err}
				continue
			}

			results[output.SwapId] = OutputResult{Fee: output.Fee}
			outValues[output.Address] += value - output.Fee
		}
		return outValues
	}

	noFeeTransaction, err := construct(network, outputs, getOutValues(0))
	if err != nil {
		return nil, nil, err
	}

	fee := uint64(float64(noFeeTransaction.VSize()) * satPerVbyte)

	transaction, err := construct(network, outputs, getOutValues(fee))
	if err != nil {
		return nil, nil, err
	}

	var valid []OutputDetails

	for i, output := range outputs {
		err = func() error {
			if output.Cooperative {
				return nil
			}
			serialized, err := transaction.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize transaction: %w", err)
			}

			if boltzApi == nil {
				return errors.New("boltzApi is required for cooperative transactions")
			}

			session, err := NewSigningSession(outputs[i].SwapTree)
			if err != nil {
				return fmt.Errorf("could not initialize signing session: %w", err)
			}

			pubNonce := session.PublicNonce()
			refundRequest := &RefundRequest{
				Transaction: serialized,
				PubNonce:    pubNonce[:],
				Index:       i,
			}
			claimRequest := &ClaimRequest{
				Transaction: serialized,
				PubNonce:    pubNonce[:],
				Index:       i,
				Preimage:    output.Preimage,
			}
			var signature *PartialSignature
			if output.SwapType == ReverseSwap {
				signature, err = boltzApi.ClaimReverseSwap(output.SwapId, claimRequest)
			} else if output.SwapType == NormalSwap {
				signature, err = boltzApi.RefundSwap(output.SwapId, refundRequest)
			} else {
				signature, err = func() (*PartialSignature, error) {
					if output.IsRefund() {
						return boltzApi.RefundChainSwap(output.SwapId, refundRequest)
					}
					if output.RefundSwapTree == nil {
						return nil, errors.New("RefundSwapTree is required for cooperatively claiming chain swap")
					}
					boltzSession, err := NewSigningSession(output.RefundSwapTree)
					if err != nil {
						return nil, fmt.Errorf("could not initialize signing session: %w", err)
					}
					details, err := boltzApi.GetChainSwapClaimDetails(output.SwapId)
					if err != nil {
						return nil, err
					}
					boltzSignature, err := boltzSession.Sign(details.TransactionHash, details.PubNonce)
					if err != nil {
						return nil, fmt.Errorf("could not sign transaction: %w", err)
					}
					return boltzApi.ExchangeChainSwapClaimSignature(output.SwapId, &ChainSwapSigningRequest{
						Preimage:  output.Preimage,
						Signature: boltzSignature,
						ToSign:    claimRequest,
					})
				}()
			}
			if err != nil {
				return fmt.Errorf("could not get partial signature from boltz: %w", err)
			}

			if err := session.Finalize(transaction, outputs, network, signature); err != nil {
				return fmt.Errorf("could not finalize signing session: %w", err)
			}

			return nil
		}()
		if err != nil {
			if output.IsRefund() {
				results[output.SwapId] = OutputResult{Err: err}
			} else {
				nonCoop := outputs[i]
				nonCoop.Cooperative = false
				valid = append(valid, nonCoop)
			}
		} else {
			valid = append(valid, output)
		}
	}

	if len(valid) == 0 {
		return nil, results, fmt.Errorf("all outputs invalid")
	}

	if len(valid) < len(outputs) {
		transaction, newResults, err := ConstructTransaction(network, currency, valid, satPerVbyte, boltzApi)
		if err != nil {
			return nil, nil, err
		}
		for id, result := range newResults {
			results[id] = result
		}
		return transaction, results, nil
	}

	return transaction, results, err
}
