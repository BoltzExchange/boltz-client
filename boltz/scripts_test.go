package boltz

import (
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckSwapScript(t *testing.T) {
	redeemScript, _ := hex.DecodeString("a9140d90b94f98198ea9ba3a94a34d27897c27024305876321037c7980160182adad9eaea06c1b1cdf9dfdce5ef865c386a112bff4a62196caf66702f800b1752103de7f16653d93ff6ceac681050e75692d7a6fa05ea473d7df90aeac40fa11e28d68ac")
	preimageHash, _ := hex.DecodeString("26cb777d4fa07a4fe47aa25bed4db29dfe32edfaac3f708299decc6d1199109c")

	key, _ := hex.DecodeString("88c4ac1e6d099ea63eda4a0ae4863420dbca9aa1bce536aa63d46db28c7b780e")
	refundKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), key)

	timeoutBlockHeight := 248

	assert.Nil(t, CheckSwapScript(redeemScript, preimageHash, refundKey, timeoutBlockHeight))

	err := errors.New("invalid redeem script")
	newKey, _ := btcec.NewPrivateKey(btcec.S256())

	assert.Equal(t, err, CheckSwapScript(redeemScript, []byte{}, refundKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckSwapScript(redeemScript, preimageHash, newKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckSwapScript(redeemScript, preimageHash, refundKey, 0))
}

func TestFormatHeight(t *testing.T) {
	assert.Equal(t, "0400", formatHeight(4))
	assert.Equal(t, "36a7", formatHeight(632630))
}
