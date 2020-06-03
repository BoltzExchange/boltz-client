package boltz

import (
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/input"
	"strings"
)

var invalidRedeemScript = errors.New("invalid redeem script")

func CheckSwapScript(redeemScript, preimageHash []byte, refundKey *btcec.PrivateKey, timeoutBlockHeight int) error {
	disassembledScript, err := txscript.DisasmString(redeemScript)

	if err != nil {
		return err
	}

	expectedScript := []string{
		"OP_HASH160",
		hex.EncodeToString(input.Ripemd160H(preimageHash)),
		"OP_EQUAL",
		"OP_IF",
		strings.Split(disassembledScript, " ")[4],
		"OP_ELSE",
		formatHeight(timeoutBlockHeight),
		"OP_CHECKLOCKTIMEVERIFY",
		"OP_DROP",
		hex.EncodeToString(refundKey.PubKey().SerializeCompressed()),
		"OP_ENDIF",
		"OP_CHECKSIG",
	}

	if disassembledScript != strings.Join(expectedScript, " ") {
		return invalidRedeemScript
	}

	return nil
}

func CheckReverseSwapScript(redeemScript, preimageHash []byte, claimKey *btcec.PrivateKey, timeoutBlockHeight int) error {
	disassembledScript, err := txscript.DisasmString(redeemScript)

	if err != nil {
		return err
	}

	expectedScript := []string{
		"OP_SIZE",
		"20",
		"OP_EQUAL",
		"OP_IF",
		"OP_HASH160",
		hex.EncodeToString(input.Ripemd160H(preimageHash)),
		"OP_EQUALVERIFY",
		hex.EncodeToString(claimKey.PubKey().SerializeCompressed()),
		"OP_ELSE",
		"OP_DROP",
		formatHeight(timeoutBlockHeight),
		"OP_CHECKLOCKTIMEVERIFY",
		"OP_DROP",
		strings.Split(disassembledScript, " ")[13],
		"OP_ENDIF",
		"OP_CHECKSIG",
	}

	if disassembledScript != strings.Join(expectedScript, " ") {
		return invalidRedeemScript
	}

	return nil
}

func formatHeight(height int) string {
	test, _ := txscript.NewScriptBuilder().AddInt64(int64(height)).Script()
	return hex.EncodeToString(test[1:])
}
