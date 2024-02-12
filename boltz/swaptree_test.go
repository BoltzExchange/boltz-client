package boltz

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"
)

func decode(t *testing.T, raw string) []byte {
	decoded, err := hex.DecodeString(raw)
	require.NoError(t, err)
	return decoded
}

func leaf(t *testing.T, isLiquid bool, script string) txscript.TapLeaf {
	decoded, err := hex.DecodeString(script)
	require.NoError(t, err)
	return txscript.TapLeaf{
		LeafVersion: leafVersion(isLiquid),
		Script:      decoded,
	}
}

func publicKey(t *testing.T, raw string) *btcec.PublicKey {
	key, err := btcec.ParsePubKey(decode(t, raw))
	require.NoError(t, err)
	return key
}

func privateKey(t *testing.T, raw string) *btcec.PrivateKey {
	key, _ := btcec.PrivKeyFromBytes(decode(t, raw))
	return key
}

type testCase struct {
	isLiquid           bool
	isReverse          bool
	ourKey             string
	boltzKey           string
	preimageHash       string
	timeoutBlockHeight uint32
	blindingKey        string

	claimLeaf  string
	refundLeaf string
	address    string
}

func setup(t *testing.T, test *testCase) *SwapTree {
	tree := &SwapTree{
		ClaimLeaf:  leaf(t, test.isLiquid, test.claimLeaf),
		RefundLeaf: leaf(t, test.isLiquid, test.refundLeaf),
	}
	err := tree.Init(
		test.isLiquid,
		privateKey(t, test.ourKey),
		publicKey(t, test.boltzKey),
	)
	require.NoError(t, err)
	return tree
}

func TestSwapTree(t *testing.T) {

	tests := []*testCase{
		{
			isLiquid:           false,
			isReverse:          true,
			ourKey:             "7886fd6464350f85c941bd80c824b1ad4f776b0aa1b4783a300b987d69966086",
			boltzKey:           "0328baf0584489b39d218d0a59bbee01e93be6fba696b348a4033045f3cdc7dc37",
			preimageHash:       "a1164fdb247b47931ed41fa1bd53391205406aa723adf4fda10b9ed013001016",
			timeoutBlockHeight: 827793,

			claimLeaf:  "82012088a914fedcea7dea7e4c7923984fab9c0b409a4ea7f38a882017ccb3202dd3a3ad29f4bc046f2b51904ece962a6e5b05da73f5eb5eeb99b1b3ac",
			refundLeaf: "2028baf0584489b39d218d0a59bbee01e93be6fba696b348a4033045f3cdc7dc37ad0391a10cb1",
			address:    "bc1prmxmvl5z79ddhesfzu3ya0f8ck9k3tfvdcrxfzc8t9s7stm4nfrsyc9hzw",
		},
		{
			isLiquid:           false,
			isReverse:          false,
			ourKey:             "265238cbdc33eafd2ab2c9bfc3b38fcd9b4d610c62973a85ea74662147eeed99",
			boltzKey:           "020e9e82ede019c483ef12f0a05a2f602be53a10f72cce88d6975a24592cf9ce07",
			preimageHash:       "8cc63191120acf891b9eff49136a92c421e833171ace1ee94a0838052f0c0f86",
			timeoutBlockHeight: 828670,

			claimLeaf:  "a9140aa6567ec32c5f62f0413be0fddc819f1d6fd0dd88200e9e82ede019c483ef12f0a05a2f602be53a10f72cce88d6975a24592cf9ce07ac",
			refundLeaf: "2078711c01248c5db8436e07c43d651355a1415f3f43d4edf2d2147bfa66c20605ad03fea40cb1",
			address:    "bc1px6up6cxg2vhf049x9g0v8wcztc0vvvd25zjqt6qf3streqvuzm7qm0dpkp",
		},
		{
			isLiquid:           true,
			isReverse:          true,
			ourKey:             "fda57d14c8f0dcaad235b50f095b55bd0f70dd17af36ec62953b7c1a99fe4860",
			boltzKey:           "020707580d72eeedc94b7429e783c227adef5b3e71a53f052e8054ae369f4b0aca",
			preimageHash:       "2febe2cec440e9d6eab320e1a92801249ee8327733ee79aa23578157d4e49514",
			timeoutBlockHeight: 2704011,
			blindingKey:        "bf3850de14cdc341e41e6be9cb3da3417c0b7d6d190258989175153a42275837",

			claimLeaf:  "82012088a9146b4608af740ded88736629d24675cef471af30288820ff807827df4764d5c0f529b07ae439c8231ceee4d0cd0b34d509007571981d37ac",
			refundLeaf: "200707580d72eeedc94b7429e783c227adef5b3e71a53f052e8054ae369f4b0acaad038b4229b1",
			address:    "lq1pqt6d2u6cvf3c2urrwwhufpwcez0ddv3ah40uvxlkeahvvap3drnyd5gpffwtvm04x48ewdr6ganltfredkhhe86y7yqst3ef4s3hxzs46uxwxxfevm60",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tree := setup(t, test)

			require.NoError(t, tree.Check(test.isReverse, test.timeoutBlockHeight, decode(t, test.preimageHash)))

			blindingKey := privateKey(t, test.blindingKey).PubKey()
			require.NoError(t, tree.CheckAddress(test.address, MainNet, blindingKey))

			_, err := tree.GetControlBlock(true)
			require.NoError(t, err)

			_, err = tree.GetControlBlock(false)
			require.NoError(t, err)
		})
	}

}
