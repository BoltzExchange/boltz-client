package boltz

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type BtcTransaction struct {
	btcutil.Tx
}

func NewBtcTxFromHex(hexString string) (*BtcTransaction, error) {
	transactionBytes, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}

	btcTx, err := btcutil.NewTxFromBytes(transactionBytes)
	if err != nil {
		return nil, err
	}
	return &BtcTransaction{Tx: *btcTx}, nil
}

func (transaction *BtcTransaction) Hash() string {
	return transaction.MsgTx().TxHash().String()
}

func (transaction *BtcTransaction) Serialize() (string, error) {
	var transactionHex bytes.Buffer
	err := transaction.MsgTx().Serialize(&transactionHex)

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(transactionHex.Bytes()), nil
}

func (transaction *BtcTransaction) VSize() uint64 {
	witnessSize := transaction.MsgTx().SerializeSize() - transaction.MsgTx().SerializeSizeStripped()
	return uint64(transaction.MsgTx().SerializeSizeStripped()) + uint64(math.Ceil(float64(witnessSize)/4))
}

func (transaction *BtcTransaction) FindVout(network *Network, addressToFind string) (uint32, uint64, error) {
	for vout, output := range transaction.MsgTx().TxOut {
		_, outputAddresses, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, network.Btc)

		// Just ignore outputs we can't decode
		if err != nil {
			continue
		}

		for _, outputAddress := range outputAddresses {
			if outputAddress.EncodeAddress() == addressToFind {
				return uint32(vout), uint64(output.Value), nil
			}
		}
	}
	return 0, 0, errors.New("Could not find address in transaction")
}

func constructBtcTransaction(network *Network, outputs []OutputDetails, outputAddressRaw string, fee uint64) (Transaction, error) {
	outputAddress, err := btcutil.DecodeAddress(outputAddressRaw, network.Btc)
	if err != nil {
		return nil, errors.New("Could not decode address: " + err.Error())
	}

	transaction := wire.NewMsgTx(wire.TxVersion)

	var inputSum int64

	for _, output := range outputs {
		// Set the highest timeout block height as locktime
		if output.TimeoutBlockHeight > transaction.LockTime {
			transaction.LockTime = output.TimeoutBlockHeight
		}

		lockupTx := output.LockupTransaction.(*BtcTransaction).Tx

		// Calculate the sum of all inputs
		inputSum += lockupTx.MsgTx().TxOut[output.Vout].Value

		// Add the input to the transaction
		input := wire.NewTxIn(wire.NewOutPoint(lockupTx.Hash(), output.Vout), nil, nil)
		input.Sequence = 0

		transaction.AddTxIn(input)
	}

	// Add the output
	outputScript, err := txscript.PayToAddrScript(outputAddress)

	if err != nil {
		return nil, err
	}

	transaction.AddTxOut(&wire.TxOut{
		PkScript: outputScript,
		Value:    inputSum - int64(fee),
	})

	// Construct the signature script and witnesses and sign the inputs
	for i, output := range outputs {
		lockupTx := output.LockupTransaction.(*BtcTransaction)
		txOut := lockupTx.MsgTx().TxOut[output.Vout]

		switch output.OutputType {
		case Legacy:
			// Set the signed signature script for legacy output
			signature, err := txscript.RawTxInSignature(
				transaction,
				i,
				output.RedeemScript,
				txscript.SigHashAll,
				output.PrivateKey,
			)

			if err != nil {
				return nil, err
			}

			signatureScriptBuilder := txscript.NewScriptBuilder()
			signatureScriptBuilder.AddData(signature)
			signatureScriptBuilder.AddData(output.Preimage)
			signatureScriptBuilder.AddData(output.RedeemScript)

			signatureScript, err := signatureScriptBuilder.Script()

			if err != nil {
				return nil, err
			}

			transaction.TxIn[i].SignatureScript = signatureScript

		case Compatibility:
			// Set the signature script for compatibility outputs
			signatureScriptBuilder := txscript.NewScriptBuilder()
			signatureScriptBuilder.AddData(createNestedP2shScript(output.RedeemScript))

			signatureScript, err := signatureScriptBuilder.Script()

			if err != nil {
				return nil, err
			}

			transaction.TxIn[i].SignatureScript = signatureScript
		}

		// Add the signed witness in case the output is not a legacy one
		if output.OutputType != Legacy {
			prevoutFetcher := txscript.NewCannedPrevOutputFetcher(txOut.PkScript, txOut.Value)
			signatureHash := txscript.NewTxSigHashes(transaction, prevoutFetcher)
			signature, err := txscript.RawTxInWitnessSignature(
				transaction,
				signatureHash,
				i,
				txOut.Value,
				output.RedeemScript,
				txscript.SigHashAll,
				output.PrivateKey,
			)

			if err != nil {
				return nil, err
			}

			transaction.TxIn[i].Witness = wire.TxWitness{signature, output.Preimage, output.RedeemScript}
		}
	}

	return &BtcTransaction{
		Tx: *btcutil.NewTx(transaction),
	}, nil
}
