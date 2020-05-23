package boltz

import (
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// TODO: detect output type and act accordingly
// TODO: actually calculate a fee
func ConstructRefundTransaction(
	lockupTransaction *btcutil.Tx,
	vout uint32,
	privateKey *btcec.PrivateKey,
	redeemScript []byte,
	outputAddress btcutil.Address,
	timeoutBlockHeight uint32,
) (*wire.MsgTx, error) {
	inputValue := lockupTransaction.MsgTx().TxOut[vout].Value

	refundTransaction := wire.NewMsgTx(wire.TxVersion)
	refundTransaction.LockTime = timeoutBlockHeight

	// Add the output
	outputScript, err := txscript.PayToAddrScript(outputAddress)

	if err != nil {
		return nil, err
	}

	refundTransaction.AddTxOut(&wire.TxOut{
		PkScript: outputScript,
		Value:    inputValue - 1000,
	})

	// Add the input
	signatureScriptBuilder := txscript.NewScriptBuilder()
	signatureScriptBuilder.AddData(createNestedP2shScript(redeemScript))

	signatureScript, err := signatureScriptBuilder.Script()

	if err != nil {
		return nil, err
	}

	input := wire.NewTxIn(wire.NewOutPoint(lockupTransaction.Hash(), vout), signatureScript, nil)
	input.Sequence = 0

	refundTransaction.AddTxIn(input)

	// Sign the input
	signatureHash := txscript.NewTxSigHashes(refundTransaction)
	signature, err := txscript.RawTxInWitnessSignature(
		refundTransaction,
		signatureHash,
		0,
		inputValue,
		redeemScript,
		txscript.SigHashAll,
		privateKey,
	)

	if err != nil {
		return nil, err
	}

	// Set the witness for the input
	refundTransaction.TxIn[0].Witness = wire.TxWitness{signature, []byte{}, redeemScript}

	return refundTransaction, nil
}
