package onchain_test

import (
	"testing"
	"time"

	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/stretchr/testify/assert"
)

const (
	txHex  = "test-tx-hex"
	txId   = "test-tx-id"
	height = uint32(100)
	fee    = float64(1.5)
)

func neverProvider(t *testing.T) onchain.ChainProvider {
	return onchainmock.NewMockChainProvider(t)
}

func TestMultiChainProvider_GetRawTransaction(t *testing.T) {
	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetRawTransaction(txId).Return(txHex, nil)
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetRawTransaction(txId).Return("", assert.AnError)
		return mockChain
	}

	tests := []struct {
		name      string
		providers func(t *testing.T) []onchain.ChainProvider
		wantHex   string
		wantErr   bool
	}{
		{
			name: "all providers success",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{successProvider(t), neverProvider(t)}
			},
			wantHex: txHex,
			wantErr: false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), successProvider(t), neverProvider(t)}
			},
			wantHex: txHex,
			wantErr: false,
		},
		{
			name: "all provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			wantHex: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
			}

			hex, err := provider.GetRawTransaction(txId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantHex, hex)
			}
		})
	}
}

func TestMultiChainProvider_BroadcastTransaction(t *testing.T) {
	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().BroadcastTransaction(txHex).RunAndReturn(func(txHex string) (string, error) {
			// without the delay, the test is flaky because it could succeed on the result of the first provider
			// before the second one is called
			time.Sleep(10 * time.Millisecond)
			return txId, nil
		})
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().BroadcastTransaction(txHex).Return("", assert.AnError)
		return mockChain
	}

	tests := []struct {
		name      string
		providers func(t *testing.T) []onchain.ChainProvider
		wantTxId  string
		wantErr   bool
	}{
		{
			name: "all providers success",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{successProvider(t), successProvider(t)}
			},
			wantTxId: txId,
			wantErr:  false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), successProvider(t), errorProvider(t)}
			},
			wantTxId: txId,
			wantErr:  false,
		},
		{
			name: "all provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			wantTxId: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
			}

			id, err := provider.BroadcastTransaction(txHex)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTxId, id)
			}
		})
	}
}

func TestMultiChainProvider_IsTransactionConfirmed(t *testing.T) {
	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().IsTransactionConfirmed(txId).Return(true, nil)
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().IsTransactionConfirmed(txId).Return(false, assert.AnError)
		return mockChain
	}

	tests := []struct {
		name          string
		providers     func(t *testing.T) []onchain.ChainProvider
		boltz         func(t *testing.T) onchain.ChainProvider
		wantConfirmed bool
		wantErr       bool
	}{
		{
			name: "all providers success",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{successProvider(t), successProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantConfirmed: true,
			wantErr:       false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), successProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantConfirmed: true,
			wantErr:       false,
		},
		{
			name: "all provider failure -> boltz fallback",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return successProvider(t)
			},
			wantConfirmed: true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
				Boltz:     tt.boltz(t),
			}

			confirmed, err := provider.IsTransactionConfirmed(txId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfirmed, confirmed)
			}
		})
	}
}

func TestMultiChainProvider_GetBlockHeight(t *testing.T) {
	heightProvider := func(t *testing.T, height uint32) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetBlockHeight().Return(height, nil).Maybe()
		mockChain.EXPECT().Disconnect().Return().Maybe()
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetBlockHeight().Return(0, assert.AnError)
		return mockChain
	}

	tests := []struct {
		name       string
		providers  func(t *testing.T) []onchain.ChainProvider
		boltz      func(t *testing.T) onchain.ChainProvider
		wantHeight uint32
		wantErr    bool
	}{
		{
			name: "all providers with same height",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{heightProvider(t, height), heightProvider(t, height)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return heightProvider(t, height+2)
			},
			wantHeight: height,
			wantErr:    false,
		},
		{
			name: "all providers with different height",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{heightProvider(t, height), errorProvider(t), heightProvider(t, height+1)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return heightProvider(t, height+2)
			},
			wantHeight: height + 1,
			wantErr:    false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), heightProvider(t, height)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return heightProvider(t, height+2)
			},
			wantHeight: height,
			wantErr:    false,
		},
		{
			name: "all provider failure -> boltz fallback",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return heightProvider(t, height+2)
			},
			wantHeight: height + 2,
			wantErr:    false,
		},
		{
			name: "all provider failure -> boltz failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return errorProvider(t)
			},
			wantErr: true,
		},
		{
			name: "all provider failure -> no boltz",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var boltzProvider onchain.ChainProvider
			if tt.boltz != nil {
				boltzProvider = tt.boltz(t)
			}

			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
				Boltz:     boltzProvider,
			}

			h, err := provider.GetBlockHeight()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantHeight, h)
			}
		})
	}
}

func TestMultiChainProvider_EstimateFee(t *testing.T) {
	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().EstimateFee().Return(fee, nil)
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().EstimateFee().Return(0, assert.AnError)
		return mockChain
	}

	tests := []struct {
		name      string
		providers func(t *testing.T) []onchain.ChainProvider
		boltz     func(t *testing.T) onchain.ChainProvider
		wantFee   float64
		wantErr   bool
	}{
		{
			name: "all providers success, only first",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{successProvider(t), neverProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantFee: fee,
			wantErr: false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), successProvider(t), neverProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantFee: fee,
			wantErr: false,
		},
		{
			name: "all provider failure -> boltz fallback",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return successProvider(t)
			},
			wantFee: fee,
			wantErr: false,
		},
		{
			name: "all provider failure -> boltz failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return errorProvider(t)
			},
			wantErr: true,
		},
		{
			name: "all provider failure -> no boltz",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var boltzProvider onchain.ChainProvider
			if tt.boltz != nil {
				boltzProvider = tt.boltz(t)
			}

			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
				Boltz:     boltzProvider,
			}

			f, err := provider.EstimateFee()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no fee found")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantFee, f)
			}
		})
	}
}

func TestMultiChainProvider_GetUnspentOutputs(t *testing.T) {
	address := "test-address"
	outputs := []*onchain.Output{
		{TxId: txId, Value: 1000},
	}

	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetUnspentOutputs(address).Return(outputs, nil)
		return mockChain
	}

	emptyProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetUnspentOutputs(address).Return([]*onchain.Output{}, nil)
		return mockChain
	}

	errorProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().GetUnspentOutputs(address).Return(nil, assert.AnError)
		return mockChain
	}

	tests := []struct {
		name        string
		providers   func(t *testing.T) []onchain.ChainProvider
		boltz       func(t *testing.T) onchain.ChainProvider
		wantOutputs []*onchain.Output
		wantErr     bool
	}{
		{
			name: "all providers success",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{successProvider(t), neverProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantOutputs: outputs,
			wantErr:     false,
		},
		{
			name: "single provider failure",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), successProvider(t), neverProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantOutputs: outputs,
			wantErr:     false,
		},
		{
			name: "empty outputs from first provider",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{emptyProvider(t), successProvider(t), neverProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return neverProvider(t)
			},
			wantOutputs: outputs,
			wantErr:     false,
		},
		{
			name: "all providers return empty",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{emptyProvider(t), emptyProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return emptyProvider(t)
			},
			wantOutputs: []*onchain.Output{},
			wantErr:     false,
		},
		{
			name: "all provider failure -> boltz fallback",
			providers: func(t *testing.T) []onchain.ChainProvider {
				return []onchain.ChainProvider{errorProvider(t), errorProvider(t)}
			},
			boltz: func(t *testing.T) onchain.ChainProvider {
				return successProvider(t)
			},
			wantOutputs: outputs,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var boltzProvider onchain.ChainProvider
			if tt.boltz != nil {
				boltzProvider = tt.boltz(t)
			}

			provider := onchain.MultiChainProvider{
				Providers: tt.providers(t),
				Boltz:     boltzProvider,
			}

			result, err := provider.GetUnspentOutputs(address)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "all providers failed")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOutputs, result)
			}
		})
	}
}

func TestMultiChainProvider_Disconnect(t *testing.T) {
	successProvider := func(t *testing.T) onchain.ChainProvider {
		mockChain := onchainmock.NewMockChainProvider(t)
		mockChain.EXPECT().Disconnect().Return()
		return mockChain
	}

	provider := onchain.MultiChainProvider{
		Providers: []onchain.ChainProvider{successProvider(t), successProvider(t)},
		Boltz:     successProvider(t),
	}

	assert.NotPanics(t, func() {
		provider.Disconnect()
	})
}
