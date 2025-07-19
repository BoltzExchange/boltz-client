package onchain_test

import (
	"testing"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/assert"
)

func TestMultiTxProvider_GetRawTransaction(t *testing.T) {
	mockProvider1 := onchainmock.NewMockTxProvider(t)
	mockProvider2 := onchainmock.NewMockTxProvider(t)
	txId := "test-tx-id"
	txHex := "test-tx-hex"

	mockProvider1.EXPECT().GetRawTransaction(txId).Return("", assert.AnError)
	mockProvider2.EXPECT().GetRawTransaction(txId).Return(txHex, nil)

	provider := onchain.MultiTxProvider{
		Providers: []onchain.TxProvider{mockProvider1, mockProvider2},
	}

	hex, err := provider.GetRawTransaction(txId)
	assert.NoError(t, err)
	assert.Equal(t, txHex, hex)
}

func TestMultiTxProvider_BroadcastTransaction(t *testing.T) {
	txHex := "test-tx-hex"
	txId := "test-tx-id"

	errorProvider := func() onchain.TxProvider {
		mock := onchainmock.NewMockTxProvider(t)
		mock.EXPECT().BroadcastTransaction(txHex).Return("", assert.AnError)
		return mock
	}

	successProvider := func() onchain.TxProvider {
		mock := onchainmock.NewMockTxProvider(t)
		mock.EXPECT().BroadcastTransaction(txHex).Return(txId, nil)
		return mock
	}

	provider := onchain.MultiTxProvider{
		Providers: []onchain.TxProvider{errorProvider(), successProvider(), errorProvider()},
	}

	id, err := provider.BroadcastTransaction(txHex)
	assert.NoError(t, err)
	assert.Equal(t, txId, id)

	provider = onchain.MultiTxProvider{
		Providers: []onchain.TxProvider{errorProvider(), errorProvider(), errorProvider()},
	}
	_, err = provider.BroadcastTransaction(txHex)
	assert.Error(t, err)
}

func TestMultiTxProvider_IsTransactionConfirmed(t *testing.T) {
	mockProvider1 := onchainmock.NewMockTxProvider(t)
	mockProvider2 := onchainmock.NewMockTxProvider(t)
	txId := "test-tx-id"

	mockProvider1.EXPECT().IsTransactionConfirmed(txId).Return(false, assert.AnError)
	mockProvider2.EXPECT().IsTransactionConfirmed(txId).Return(true, nil)

	provider := onchain.MultiTxProvider{
		Providers: []onchain.TxProvider{mockProvider1, mockProvider2},
	}

	confirmed, err := provider.IsTransactionConfirmed(txId)
	assert.NoError(t, err)
	assert.True(t, confirmed)
}
