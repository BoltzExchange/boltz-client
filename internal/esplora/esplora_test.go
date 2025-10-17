//go:build !unit

package esplora

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

const (
	blockstreamAPI = "https://blockstream.info/api"

	genesisTxId = "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

	confirmedTxId = "0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098"
)

func TestInitClient(t *testing.T) {
	client := InitClient(blockstreamAPI)
	require.NotNil(t, client)
	require.Equal(t, blockstreamAPI, client.api)
	require.NotNil(t, client.httpClient)
}

func TestGetBlockHeight(t *testing.T) {
	client := InitClient(blockstreamAPI)
	defer client.Disconnect()

	height, err := client.GetBlockHeight()
	require.NoError(t, err)
	require.GreaterOrEqual(t, height, uint32(918877))

	t.Logf("Current block height: %d", height)
}

func TestGetRawTransaction(t *testing.T) {
	client := InitClient(blockstreamAPI)
	defer client.Disconnect()

	tests := []struct {
		name string
		txId string
	}{
		{
			name: "genesis transaction",
			txId: genesisTxId,
		},
		{
			name: "confirmed transaction",
			txId: confirmedTxId,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hex, err := client.GetRawTransaction(tt.txId)
			require.NoError(t, err)

			_, err = boltz.NewTxFromHex(boltz.CurrencyBtc, hex, nil)
			require.NoError(t, err)
		})
	}

	t.Run("invalid transaction", func(t *testing.T) {
		invalidTxId := "0000000000000000000000000000000000000000000000000000000000000000"
		_, err := client.GetRawTransaction(invalidTxId)
		require.Error(t, err, "Expected error for invalid transaction ID")
	})
}

func TestIsTransactionConfirmed(t *testing.T) {
	client := InitClient(blockstreamAPI)
	defer client.Disconnect()

	tests := []struct {
		name              string
		txId              string
		shouldBeConfirmed bool
	}{
		{
			name:              "genesis transaction should be confirmed",
			txId:              genesisTxId,
			shouldBeConfirmed: true,
		},
		{
			name:              "early transaction should be confirmed",
			txId:              confirmedTxId,
			shouldBeConfirmed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confirmed, err := client.IsTransactionConfirmed(tt.txId)
			require.NoError(t, err)
			require.Equal(t, tt.shouldBeConfirmed, confirmed)

			t.Logf("Transaction %s confirmed: %v", tt.txId, confirmed)
		})
	}
}

func TestBroadcastTransactionInvalid(t *testing.T) {
	client := InitClient(blockstreamAPI)
	defer client.Disconnect()

	// Try to broadcast an invalid transaction
	invalidTxHex := "0000"
	_, err := client.BroadcastTransaction(invalidTxHex)
	require.Error(t, err, "Expected error when broadcasting invalid transaction")
	require.Contains(t, err.Error(), "could not broadcast tx", "Error should indicate broadcast failure")
}

func TestGetUnspentOutputs(t *testing.T) {
	client := InitClient(blockstreamAPI)
	defer client.Disconnect()

	t.Run("address with UTXOs", func(t *testing.T) {
		// Using a bech32 address with moderate amount of UTXOs
		address := "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq"
		utxos, err := client.GetUnspentOutputs(address)
		require.NoError(t, err)
		require.NotNil(t, utxos)

		t.Logf("Address %s has %d UTXOs", address, len(utxos))

		// Verify UTXOs structure
		for _, utxo := range utxos {
			require.NotEmpty(t, utxo.TxId)
			require.Greater(t, utxo.Value, uint64(0))
		}
	})

	t.Run("address with no UTXOs", func(t *testing.T) {
		// Using a valid bech32 address with no UTXOs
		address := "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
		utxos, err := client.GetUnspentOutputs(address)
		require.NoError(t, err)
		require.NotNil(t, utxos)
		require.Equal(t, 0, len(utxos))
		t.Logf("Empty address has %d UTXOs", len(utxos))
	})
}
