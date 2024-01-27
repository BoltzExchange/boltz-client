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

func leaf(t *testing.T, version txscript.TapscriptLeafVersion, script string) txscript.TapLeaf {
	decoded, err := hex.DecodeString(script)
	require.NoError(t, err)
	return txscript.TapLeaf{
		LeafVersion: version,
		Script:      decoded,
	}
}

func key(t *testing.T, raw string) *btcec.PublicKey {
	key, err := btcec.ParsePubKey(decode(t, raw))
	require.NoError(t, err)
	return key
}

type testCase struct {
	isLiquid           bool
	isReverse          bool
	claimPubKey        string
	refundPubKey       string
	preimageHash       string
	timeoutBlockHeight uint32

	claimLeaf  string
	refundLeaf string
	address    string
}

func setup(t *testing.T, test *testCase) *SwapTree {
	tree, err := NewSwapTree(
		test.isLiquid,
		test.isReverse,
		key(t, test.claimPubKey),
		key(t, test.refundPubKey),
		decode(t, test.preimageHash),
		test.timeoutBlockHeight,
	)

	require.NoError(t, err)
	return tree
}

func TestSwapTree(t *testing.T) {

	tests := []*testCase{
		{
			isLiquid:           false,
			isReverse:          true,
			claimPubKey:        "0217ccb3202dd3a3ad29f4bc046f2b51904ece962a6e5b05da73f5eb5eeb99b1b3",
			refundPubKey:       "0328baf0584489b39d218d0a59bbee01e93be6fba696b348a4033045f3cdc7dc37",
			preimageHash:       "a1164fdb247b47931ed41fa1bd53391205406aa723adf4fda10b9ed013001016",
			timeoutBlockHeight: 827793,

			claimLeaf:  "82012088a914fedcea7dea7e4c7923984fab9c0b409a4ea7f38a882017ccb3202dd3a3ad29f4bc046f2b51904ece962a6e5b05da73f5eb5eeb99b1b3ac",
			refundLeaf: "2028baf0584489b39d218d0a59bbee01e93be6fba696b348a4033045f3cdc7dc37ad0391a10cb1",
			address:    "bc1prmxmvl5z79ddhesfzu3ya0f8ck9k3tfvdcrxfzc8t9s7stm4nfrsyc9hzw",
		},
		{
			isLiquid:           false,
			isReverse:          false,
			claimPubKey:        "020e9e82ede019c483ef12f0a05a2f602be53a10f72cce88d6975a24592cf9ce07",
			refundPubKey:       "0278711c01248c5db8436e07c43d651355a1415f3f43d4edf2d2147bfa66c20605",
			preimageHash:       "8cc63191120acf891b9eff49136a92c421e833171ace1ee94a0838052f0c0f86",
			timeoutBlockHeight: 828670,

			claimLeaf:  "a9140aa6567ec32c5f62f0413be0fddc819f1d6fd0dd88200e9e82ede019c483ef12f0a05a2f602be53a10f72cce88d6975a24592cf9ce07ac",
			refundLeaf: "2078711c01248c5db8436e07c43d651355a1415f3f43d4edf2d2147bfa66c20605ad03fea40cb1",
			address:    "bc1px6up6cxg2vhf049x9g0v8wcztc0vvvd25zjqt6qf3streqvuzm7qm0dpkp",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tree := setup(t, test)

			address, err := tree.Address(MainNet)
			require.NoError(t, err)
			require.Equal(t, test.address, address)

			require.Equal(t, tree.ClaimLeaf, leaf(t, 192, test.claimLeaf))
			require.Equal(t, tree.RefundLeaf, leaf(t, 192, test.refundLeaf))

			_, err = tree.GetControlBlock(tree.ClaimLeaf)
			require.NoError(t, err)

			_, err = tree.GetControlBlock(tree.RefundLeaf)
			require.NoError(t, err)
		})
	}

}
