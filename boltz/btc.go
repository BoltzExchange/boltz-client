package boltz

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
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
	toFind, err := btcutil.DecodeAddress(addressToFind, network.Btc)
	if err != nil {
		return 0, 0, err
	}
	scriptAddress := toFind.ScriptAddress()
	for vout, output := range transaction.MsgTx().TxOut {
		if len(output.PkScript) > 2 {
			// first 2 bytes are witness type and length
			if bytes.Equal(output.PkScript[2:], scriptAddress) {
				return uint32(vout), uint64(output.Value), nil
			}
		}
	}
	return 0, 0, errors.New("Could not find address in transaction")
}

func getPrevoutFetcher(tx *wire.MsgTx, outputs []OutputDetails) txscript.PrevOutputFetcher {
	previous := make(map[wire.OutPoint]*wire.TxOut)
	for i, input := range tx.TxIn {
		prevOut := input.PreviousOutPoint
		lockupTx := outputs[i].LockupTransaction.(*BtcTransaction).MsgTx()
		previous[prevOut] = lockupTx.TxOut[prevOut.Index]
	}

	return txscript.NewMultiPrevOutFetcher(previous)
}

func btcTaprootHash(transaction Transaction, outputs []OutputDetails, index int) ([32]byte, error) {
	tx := transaction.(*BtcTransaction).MsgTx()

	previous := make(map[wire.OutPoint]*wire.TxOut)
	for i, input := range tx.TxIn {
		prevOut := input.PreviousOutPoint
		lockupTx := outputs[i].LockupTransaction.(*BtcTransaction)
		previous[prevOut] = lockupTx.MsgTx().TxOut[prevOut.Index]
	}

	prevoutFetcher := getPrevoutFetcher(tx, outputs)
	sigHashes := txscript.NewTxSigHashes(tx, prevoutFetcher)

	hash, err := txscript.CalcTaprootSignatureHash(
		sigHashes,
		sigHashType,
		tx,
		index,
		prevoutFetcher,
	)
	return [32]byte(hash), err
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
		if !output.Cooperative {
			if output.TimeoutBlockHeight > transaction.LockTime {
				transaction.LockTime = output.TimeoutBlockHeight
			}
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

	prevoutFetcher := getPrevoutFetcher(transaction, outputs)
	sigHashes := txscript.NewTxSigHashes(transaction, prevoutFetcher)

	// Construct the signature script and witnesses and sign the inputs
	for i, output := range outputs {
		if output.Cooperative {
			// dummy signature for accurate fee estimation - actual signature is added later
			transaction.TxIn[i].Witness = wire.TxWitness{dummySignature}
		} else {
			lockupTx := output.LockupTransaction.(*BtcTransaction)
			txOut := lockupTx.MsgTx().TxOut[output.Vout]

			isRefund := output.IsRefund()
			leaf := output.SwapTree.GetLeaf(isRefund)

			signature, err := txscript.RawTxInTapscriptSignature(
				transaction,
				sigHashes,
				i,
				txOut.Value,
				txOut.PkScript,
				leaf,
				sigHashType,
				output.PrivateKey,
			)
			if err != nil {
				return nil, fmt.Errorf("could not sign Taproot input: %w", err)
			}

			witness := wire.TxWitness{signature}
			if !isRefund {
				witness = append(witness, output.Preimage)
			}

			controlBlockBytes, err := output.SwapTree.GetControlBlock(isRefund)
			if err != nil {
				return nil, fmt.Errorf("could not create control block: %w", err)
			}

			transaction.TxIn[i].Witness = append(witness, leaf.Script, controlBlockBytes)
		}
	}

	return &BtcTransaction{
		Tx: *btcutil.NewTx(transaction),
	}, nil
}
