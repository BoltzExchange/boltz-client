package boltz

import (
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"math"
)

type OutputType int

const (
	SegWit OutputType = iota
	Compatibility
	Legacy
)

type OutputDetails struct {
	LockupTransaction *btcutil.Tx
	Vout              uint32
	OutputType        OutputType

	RedeemScript []byte
	PrivateKey   *btcec.PrivateKey

	// Should be set to an empty array in case of a refund
	Preimage []byte

	// Can be zero in case of a claim transaction
	TimeoutBlockHeight uint32
}

func ConstructTransaction(outputs []OutputDetails, outputAddress btcutil.Address, satPerVbyte int64) (*wire.MsgTx, error) {
	noFeeTransaction, err := constructTransaction(outputs, outputAddress, 0)

	if err != nil {
		return nil, err
	}

	witnessSize := noFeeTransaction.SerializeSize() - noFeeTransaction.SerializeSizeStripped()
	vByte := int64(noFeeTransaction.SerializeSizeStripped()) + int64(math.Ceil(float64(witnessSize)/4))

	return constructTransaction(outputs, outputAddress, vByte*satPerVbyte)
}

func constructTransaction(outputs []OutputDetails, outputAddress btcutil.Address, fee int64) (*wire.MsgTx, error) {
	transaction := wire.NewMsgTx(wire.TxVersion)

	var inputSum int64

	for _, output := range outputs {
		// Set the highest timeout block height as locktime
		if output.TimeoutBlockHeight > transaction.LockTime {
			transaction.LockTime = output.TimeoutBlockHeight
		}

		// Calculate the sum of all inputs
		inputSum += output.LockupTransaction.MsgTx().TxOut[output.Vout].Value

		// Add the input to the transaction
		input := wire.NewTxIn(wire.NewOutPoint(output.LockupTransaction.Hash(), output.Vout), nil, nil)
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
		Value:    inputSum - fee,
	})

	// Construct the signature script and witnesses and sign the inputs
	for i, output := range outputs {
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
			signatureHash := txscript.NewTxSigHashes(transaction)
			signature, err := txscript.RawTxInWitnessSignature(
				transaction,
				signatureHash,
				i,
				output.LockupTransaction.MsgTx().TxOut[output.Vout].Value,
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

	return transaction, nil
}
