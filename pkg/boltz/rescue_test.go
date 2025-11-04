package boltz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveKey(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	lastKeyIndex := uint32(0)
	firstKey, err := DeriveKey(mnemonic, lastKeyIndex)
	require.NoError(t, err)
	require.NotNil(t, firstKey)

	lastKeyIndex = 1
	secondKey, err := DeriveKey(mnemonic, lastKeyIndex)
	require.NoError(t, err)
	require.NotNil(t, secondKey)
	require.NotEqual(t, firstKey.PubKey().SerializeCompressed(), secondKey.PubKey().SerializeCompressed())
}
