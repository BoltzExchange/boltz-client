package boltz

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSerializeTransaction(t *testing.T) {
	rawTx := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0804ffff001d02fd04ffffffff0100f2052a01000000434104f5eeb2b10c944c6b9fbcfff94c35bdeecd93df977882babc7f3a2cf7f5c81d3b09a68db7f0e04f21de5d4230e75e6dbe7ad16eefe0d4325a62067dc6f369446aac00000000"
	bytesTx, _ := hex.DecodeString(rawTx)
	tx, _ := btcutil.NewTxFromBytes(bytesTx)

	serializedTransaction, err := SerializeTransaction(tx.MsgTx())

	assert.Nil(t, err)
	assert.Equal(t, serializedTransaction, rawTx)
}

func TestCreateNestedP2shScript(t *testing.T) {
	script := createNestedP2shScript(redeemScript)
	expectedScript, _ := hex.DecodeString("0020f47e2b7c85fe6af0189ba3af47102f6f59d9ba3c2edfc51ddd4f495e93c28d71")

	assert.Equal(t, expectedScript, script)
}
