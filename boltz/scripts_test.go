package boltz

import (
	"encoding/hex"
	"errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckSwapScript(t *testing.T) {
	redeemScript, _ := hex.DecodeString("a9140d90b94f98198ea9ba3a94a34d27897c27024305876321037c7980160182adad9eaea06c1b1cdf9dfdce5ef865c386a112bff4a62196caf66702f800b1752103de7f16653d93ff6ceac681050e75692d7a6fa05ea473d7df90aeac40fa11e28d68ac")
	preimageHash, _ := hex.DecodeString("26cb777d4fa07a4fe47aa25bed4db29dfe32edfaac3f708299decc6d1199109c")

	key, _ := hex.DecodeString("88c4ac1e6d099ea63eda4a0ae4863420dbca9aa1bce536aa63d46db28c7b780e")
	refundKey, _ := btcec.PrivKeyFromBytes(key)

	var timeoutBlockHeight uint32 = 248

	assert.Nil(t, CheckSwapScript(redeemScript, preimageHash, refundKey, timeoutBlockHeight))

	err := errors.New("invalid redeem script")
	newKey, _ := btcec.NewPrivateKey()

	assert.Equal(t, err, CheckSwapScript(redeemScript, []byte{}, refundKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckSwapScript(redeemScript, preimageHash, newKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckSwapScript(redeemScript, preimageHash, refundKey, 0))
}

func TestCheckReverseSwapScript(t *testing.T) {
	redeemScript, _ := hex.DecodeString("8201208763a9147ba0ab22fcffda41fd324aba4b5ce192ba9ec5dd882102e82694032768e49526972307874d868b67c87c37e9256c05a2c5c0474e7395e3677502f800b175210247d7443123302272524c9754b44a6e7e6e1236719e9f468e15927aa4ea26301168ac")
	preimageHash, _ := hex.DecodeString("fa9ef1d253d34e9e44da97b00c6ec6a95058f646de35ddb7649fc3313ac6fc61")

	key, _ := hex.DecodeString("dddc90e33843662631fb8c3833c4743ffd8f00a94715735633bf178e62eb291c")
	claimKey, _ := btcec.PrivKeyFromBytes(key)

	var timeoutBlockHeight uint32 = 248

	assert.Nil(t, CheckReverseSwapScript(redeemScript, preimageHash, claimKey, timeoutBlockHeight))

	err := errors.New("invalid redeem script")
	newKey, _ := btcec.NewPrivateKey()

	assert.Equal(t, err, CheckReverseSwapScript(redeemScript, []byte{}, claimKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckReverseSwapScript(redeemScript, preimageHash, newKey, timeoutBlockHeight))
	assert.Equal(t, err, CheckReverseSwapScript(redeemScript, preimageHash, claimKey, 0))
}

func TestFormatHeight(t *testing.T) {
	assert.Equal(t, "68", formatHeight(104))
	assert.Equal(t, "36a709", formatHeight(632630))
}
