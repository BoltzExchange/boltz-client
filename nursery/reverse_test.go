package nursery

import (
	"encoding/hex"
	"encoding/json"
	testpkg "github.com/BoltzExchange/boltz-client/test"
	"os"
	"testing"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	LockupTx      string  `json:"lockupTx"`
	LockupAddress string  `json:"lockupAddress"`
	Fee           float64 `json:"fee"`
	Swap          *database.ReverseSwap
}

func getTestData(t *testing.T) *testCase {
	testpkg.InitLogger()
	bytes, err := os.ReadFile("data/test.json")
	require.NoError(t, err)

	var test testCase
	err = json.Unmarshal(bytes, &test)
	require.NoError(t, err)

	test.Swap = &database.ReverseSwap{
		Id:                  "NphZ9v",
		PairId:              "L-BTC/BTC",
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		AcceptZeroConf:      true,
		Invoice:             "lnbcrt1231230n1pjs4fuupp5s2ymkcnw8gjys9ydqnm4lqwfmexrvgap3rm20sylkpz520kz2fjsdpz2djkuepqw3hjqnpdgf2yxgrpv3j8yetnwvcqz95xqrrsssp55vdvd337frex235ar45hg94xgvqga4pp5cmhr7njvctsgl4y8jfs9qyyssqq43qjk77adynjq8qxpcsdma77aelwq5ygrsctvng077krd35utg8qczgkcefw2hkcjw4pxmslmvnuy67452ppsxncuvgvjxa9wpl80cqj6n9u9",
		OnchainAmount:       122231,
		TimeoutBlockHeight:  1596,
		LockupTransactionId: "",
		ClaimTransactionId:  "",
	}
	test.Swap.Preimage, _ = hex.DecodeString("2a56524c04cdac083e6da9902332550b1988a5cd0df86b5893500f00df25763e")
	test.Swap.PrivateKey, _ = database.ParsePrivateKey("755001b726c53ee4a553b259706a9aa6c31bf742ff5f4b4b6365579c97ab0390")
	test.Swap.RedeemScript, _ = hex.DecodeString("8201208763a914ce2065ef5a1758dbd042b58883101e5fc4a54c01882103bb5584e503873ed74e797fc6d6c1f0f478347260a0467c40df4902eb82aaca0a6775023c06b1752102b7f308c1192866e17498f6f9d57b353dcf62f0485d7989a459b04445b271c0a668ac")
	test.Swap.BlindingKey, _ = database.ParsePrivateKey("3d1d4f2aa498ca42e658747dcbd93d7ab50ba34ac855f6015e7a5b29c03bffe2")

	return &test
}

func TestLiquidClaimTx(t *testing.T) {
	testpkg.InitLogger()
	test := getTestData(t)

	test.Swap.ClaimAddress = "el1qqfhkj4kr0ysyrp57rr7fjf324pvpeepyplmyzc8x46ae3smmqfechzk7t7yt28pk8sn3nnktxgngxddy0g5c2awc38fxzvz24"
	_, _, err := createClaimTransaction(boltz.Regtest, test.Swap, test.LockupTx, test.Fee)
	require.NoError(t, err)

	test.Swap.ClaimAddress = "XS59g2deRD2TtUp9LHmncjb1MNBQaGvSPK"
	_, _, err = createClaimTransaction(boltz.Regtest, test.Swap, test.LockupTx, test.Fee)
	require.NoError(t, err)
}

func TestLockupAddr(t *testing.T) {
	testpkg.InitLogger()
	test := getTestData(t)

	lockupAddress, err := boltz.WitnessScriptHashAddress(boltz.PairLiquid, boltz.Regtest, test.Swap.RedeemScript, test.Swap.BlindingKey.PubKey())

	require.NoError(t, err)
	require.Equal(t, test.LockupAddress, lockupAddress)
}

func TestFindVout(t *testing.T) {
	testpkg.InitLogger()
	test := getTestData(t)

	tx, err := boltz.NewTxFromHex(test.LockupTx, test.Swap.BlindingKey)
	require.NoError(t, err)

	vout, _, err := tx.FindVout(boltz.Regtest, test.LockupAddress)

	require.NoError(t, err)
	require.Equal(t, uint32(0), vout)
}
