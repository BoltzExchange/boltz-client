//go:build !unit

package esplora

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

const (
	blockstreamAPI = "https://blockstream.info/api"

	genesisTxId = "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

	confirmedTxId = "0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098"

	addressWithUtxos = "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq"
	emptyAddress     = "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"

	esploraAttempts = 3
	esploraRetry    = time.Second
)

func testClient(t *testing.T) *Client {
	client := InitClient(blockstreamAPI)
	t.Cleanup(client.Disconnect)
	return client
}

func requireEventuallyNoError(t *testing.T, run func() error) {
	t.Helper()

	var err error
	for attempt := 1; attempt <= esploraAttempts; attempt++ {
		err = run()
		if err == nil {
			return
		}
		if attempt < esploraAttempts {
			t.Logf("attempt %d/%d failed: %v", attempt, esploraAttempts, err)
			time.Sleep(esploraRetry)
		}
	}

	require.NoError(t, err)
}

func TestInitClient(t *testing.T) {
	client := InitClient(blockstreamAPI)
	require.NotNil(t, client)
	require.Equal(t, blockstreamAPI, client.api)
	require.NotNil(t, client.httpClient)
}

func TestGetBlockHeight(t *testing.T) {
	client := testClient(t)

	var height uint32
	requireEventuallyNoError(t, func() error {
		var err error
		height, err = client.GetBlockHeight()
		return err
	})
	require.GreaterOrEqual(t, height, uint32(918877))

	t.Logf("Current block height: %d", height)
}

func TestGetRawTransaction(t *testing.T) {
	client := testClient(t)

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
			var hex string
			requireEventuallyNoError(t, func() error {
				var err error
				hex, err = client.GetRawTransaction(tt.txId)
				return err
			})

			_, err := boltz.NewTxFromHex(boltz.CurrencyBtc, hex, nil)
			require.NoError(t, err)
		})
	}

	t.Run("invalid transaction", func(t *testing.T) {
		invalidTxId := "0000000000000000000000000000000000000000000000000000000000000000"
		requireEventuallyNoError(t, func() error {
			_, err := client.GetRawTransaction(invalidTxId)
			if err == nil {
				return errors.New("expected error for invalid transaction ID")
			}
			if strings.Contains(err.Error(), "status") {
				return nil
			}
			return err
		})
	})
}

func TestIsTransactionConfirmed(t *testing.T) {
	client := testClient(t)

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
			var confirmed bool
			requireEventuallyNoError(t, func() error {
				var err error
				confirmed, err = client.IsTransactionConfirmed(tt.txId)
				return err
			})
			require.Equal(t, tt.shouldBeConfirmed, confirmed)

			t.Logf("Transaction %s confirmed: %v", tt.txId, confirmed)
		})
	}
}

func TestBroadcastTransactionInvalid(t *testing.T) {
	client := testClient(t)

	invalidTxHex := "0000"
	requireEventuallyNoError(t, func() error {
		_, err := client.BroadcastTransaction(invalidTxHex)
		if err == nil {
			return errors.New("expected error when broadcasting invalid transaction")
		}
		if !strings.Contains(err.Error(), "could not broadcast tx") {
			return err
		}
		return nil
	})
}

func TestGetUnspentOutputs(t *testing.T) {
	client := testClient(t)

	t.Run("address with UTXOs", func(t *testing.T) {
		var utxos []*onchain.Output
		requireEventuallyNoError(t, func() error {
			var err error
			utxos, err = client.GetUnspentOutputs(addressWithUtxos)
			return err
		})
		require.NotNil(t, utxos)

		t.Logf("Address %s has %d UTXOs", addressWithUtxos, len(utxos))

		for _, utxo := range utxos {
			require.NotEmpty(t, utxo.TxId)
			require.Greater(t, utxo.Value, uint64(0))
		}
	})

	t.Run("address with no UTXOs", func(t *testing.T) {
		var utxos []*onchain.Output
		requireEventuallyNoError(t, func() error {
			var err error
			utxos, err = client.GetUnspentOutputs(emptyAddress)
			return err
		})
		require.NotNil(t, utxos)
		require.Equal(t, 0, len(utxos))
		t.Logf("Empty address has %d UTXOs", len(utxos))
	})
}
