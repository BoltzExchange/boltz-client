package boltz

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func SerializeTransaction(transaction *wire.MsgTx) (string, error) {
	var transactionHex bytes.Buffer
	err := transaction.Serialize(&transactionHex)

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(transactionHex.Bytes()), nil
}

func createNestedP2shScript(redeemScript []byte) []byte {
	addressScript := []byte{
		txscript.OP_0,
		txscript.OP_DATA_32,
	}

	redeemScriptHash := sha256.Sum256(redeemScript)
	addressScript = append(addressScript, redeemScriptHash[:]...)

	return addressScript
}

func createP2wshScript(redeemScript []byte) ([]byte, error) {
	hash := sha256.Sum256(redeemScript)
	return txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(hash[:]).Script()
}
