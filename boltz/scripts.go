package boltz

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/input"
	"strings"
)

func CheckSwapScript(redeemScript []byte, preimageHash []byte, refundKey *btcec.PrivateKey, timeoutBlockHeight int) error {
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
		return errors.New("invalid redeem script")
	}

	return nil
}

func formatHeight(height int) string {
	endian := make([]byte, 8)
	binary.LittleEndian.PutUint64(endian, uint64(height))

	hexNumber := hex.EncodeToString(endian)

	for strings.HasSuffix(hexNumber, "0") {
		hexNumber = hexNumber[:len(hexNumber)-1]
	}

	return hexNumber
}
