package boltz

import (
	"errors"
	"fmt"
	"maps"
	"math"

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
	LockupTransaction  Transaction
	Vout               uint32
	Fee                uint64
	Address            string
	PrivateKey         *btcec.PrivateKey
	Preimage           []byte // empty for refunds
	TimeoutBlockHeight uint32

	SwapTree           *SwapTree
	Cooperative        bool
	RefundSwapTree     *SwapTree    // required for cooperative chain swap claims
	FundingAddressTree *FundingTree // required for funding address refunds
	FundingAddressId   string       // when set, triggers direct funding address refund flow

	SwapId   string
	SwapType SwapType
}

// Fee represents either a fixed fee in satoshis or a fee rate per vbyte.
// Exactly one of Sats or SatsPerVbyte should be set, not both.
type Fee struct {
	// Fixed fee amount in satoshis
	Sats *uint64
	// Fee rate in satoshis per virtual byte
	SatsPerVbyte *float64
}

func (f *Fee) HasSats() bool {
	return f.Sats != nil
}

func (f *Fee) HasSatsPerVbyte() bool {
	return f.SatsPerVbyte != nil
}

func (f *Fee) IsValid() bool {
	return (f.HasSats() && !f.HasSatsPerVbyte()) || (!f.HasSats() && f.HasSatsPerVbyte())
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

func TransactionCurrency(transaction Transaction) Currency {
	switch transaction.(type) {
	case *LiquidTransaction:
		return CurrencyLiquid
	default:
		return CurrencyBtc
	}
}

type OutputResult struct {
	Err error
	Fee uint64
}

type Results map[string]OutputResult

func (results Results) SetErr(id string, err error) {
	if results[id].Err == nil {
		results[id] = OutputResult{Err: err}
	}
}

func ConstructTransaction(network *Network, currency Currency, outputs []OutputDetails, fee Fee, boltzApi *Api) (Transaction, Results, error) {
	construct := constructBtcTransaction
	if currency == CurrencyLiquid {
		construct = constructLiquidTransaction
	}
	results := make(Results, len(outputs))

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
			if err != nil {
				results.SetErr(output.SwapId, err)
				continue
			}
			if value < output.Fee {
				results.SetErr(output.SwapId, fmt.Errorf("value less than fee: %d < %d", value, output.Fee))
				continue
			}

			results[output.SwapId] = OutputResult{Fee: output.Fee}
			outValues[output.Address] += value - output.Fee
		}
		return outValues
	}

	if !fee.IsValid() {
		return nil, nil, fmt.Errorf("invalid fee: %v", fee)
	}

	var transaction Transaction
	var err error

	if fee.HasSats() {
		transaction, err = construct(network, outputs, getOutValues(*fee.Sats))
		if err != nil {
			return nil, nil, err
		}
	} else {
		noFeeTransaction, err := construct(network, outputs, getOutValues(0))
		if err != nil {
			return nil, nil, err
		}

		fee := uint64(math.Ceil(float64(noFeeTransaction.VSize()) * *fee.SatsPerVbyte))

		transaction, err = construct(network, outputs, getOutValues(fee))
		if err != nil {
			return nil, nil, err
		}
	}

	var valid []OutputDetails
	reconstruct := false
	for i, output := range outputs {
		err = func() error {
			if !output.Cooperative {
				return nil
			}

			if boltzApi == nil {
				return errors.New("boltzApi is required for cooperative transactions")
			}

			// Handle direct funding address refund (not through a swap)
			if output.FundingAddressId != "" {
				return handleFundingAddressRefund(network, currency, transaction, outputs, i, boltzApi)
			}

			serialized, err := transaction.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize transaction: %w", err)
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
			switch output.SwapType {
			case ReverseSwap:
				signature, err = boltzApi.ClaimReverseSwap(output.SwapId, claimRequest)
			case NormalSwap:
				signature, err = boltzApi.RefundSwap(output.SwapId, refundRequest)
			default:
				signature, err = func() (*PartialSignature, error) {
					if output.IsRefund() {
						return boltzApi.RefundChainSwap(output.SwapId, refundRequest)
					}
					details, err := boltzApi.GetChainSwapClaimDetails(output.SwapId)
					if err != nil {
						return nil, err
					}
					var boltzSignature *PartialSignature
					if details.FundingAddressId != "" {
						if output.FundingAddressTree == nil {
							return nil, fmt.Errorf("FundingAddressTree is required for cooperatively signing chain swap with funding address")
						}
						boltzSession, err := NewFundingSigningSession(output.FundingAddressTree)
						if err != nil {
							return nil, fmt.Errorf("could not initialize signing session: %w", err)
						}
						boltzSignature, err = boltzSession.Sign(&FundingAddressSigningDetails{
							TransactionHash: details.TransactionHash,
							PubNonce:        details.PubNonce,
						})
					} else {
						if output.RefundSwapTree == nil {
							return nil, errors.New("RefundSwapTree is required for cooperatively claiming chain swap")
						}
						boltzSession, err := NewSigningSession(output.RefundSwapTree)
						if err != nil {
							return nil, fmt.Errorf("could not initialize signing session: %w", err)
						}
						boltzSignature, err = boltzSession.Sign(details.TransactionHash, details.PubNonce)
					}
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
				reconstruct = true
			} else {
				nonCoop := outputs[i]
				nonCoop.Cooperative = false
				valid = append(valid, nonCoop)
				reconstruct = true
			}
		} else {
			valid = append(valid, output)
		}
	}

	if len(valid) == 0 {
		return nil, results, fmt.Errorf("all outputs invalid")
	}

	if reconstruct {
		transaction, newResults, err := ConstructTransaction(network, currency, valid, fee, boltzApi)
		if err != nil {
			return nil, nil, err
		}
		maps.Copy(results, newResults)
		return transaction, results, nil
	}

	return transaction, results, err
}

// handleFundingAddressRefund handles cooperative signing for direct funding address refunds.
func handleFundingAddressRefund(
	network *Network,
	currency Currency,
	transaction Transaction,
	outputs []OutputDetails,
	index int,
	boltzApi *Api,
) error {
	output := outputs[index]

	if output.FundingAddressTree == nil {
		return errors.New("FundingAddressTree is required for refunding funding address")
	}

	var txHash []byte
	var err error
	if currency == CurrencyLiquid {
		txHash = liquidTaprootHash(&transaction.(*LiquidTransaction).Transaction, network, outputs, index, true)
	} else {
		txHash, err = btcTaprootHash(transaction, outputs, index)
	}
	if err != nil {
		return fmt.Errorf("could not compute transaction hash: %w", err)
	}

	session, err := NewFundingSigningSession(output.FundingAddressTree)
	if err != nil {
		return fmt.Errorf("could not create signing session: %w", err)
	}

	ourNonce := session.PublicNonce()
	refundRequest := &FundingAddressRefundRequest{
		PubNonce:        ourNonce[:],
		TransactionHash: txHash,
	}

	boltzSignature, err := boltzApi.RefundFundingAddress(output.FundingAddressId, refundRequest)
	if err != nil {
		return fmt.Errorf("could not get partial signature from Boltz: %w", err)
	}

	haveAllNonces, err := session.RegisterPubNonce([66]byte(boltzSignature.PubNonce))
	if err != nil {
		return fmt.Errorf("could not register Boltz nonce: %w", err)
	}
	if !haveAllNonces {
		return errors.New("could not combine all nonces")
	}

	_, err = session.Session.Sign([32]byte(txHash))
	if err != nil {
		return fmt.Errorf("could not create partial signature: %w", err)
	}

	boltzPartial, err := decodePartialSignature(boltzSignature.PartialSignature)
	if err != nil {
		return fmt.Errorf("could not decode Boltz partial signature: %w", err)
	}

	finalSig, err := session.CombineSig(boltzPartial)
	if err != nil {
		return fmt.Errorf("could not combine Boltz signature: %w", err)
	}
	if !finalSig {
		return errors.New("could not finalize signature after combining both partials")
	}

	finalSchnorrSig := session.FinalSig()
	if finalSchnorrSig == nil {
		return errors.New("final signature is nil")
	}

	signatureBytes := finalSchnorrSig.Serialize()
	if currency == CurrencyLiquid {
		tx := &transaction.(*LiquidTransaction).Transaction
		tx.Inputs[index].Witness = [][]byte{signatureBytes}
	} else {
		tx := transaction.(*BtcTransaction).MsgTx()
		tx.TxIn[index].Witness = [][]byte{signatureBytes}
	}
	return nil
}
