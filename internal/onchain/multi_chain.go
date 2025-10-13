package onchain

import (
	"errors"
	"fmt"
	"sync"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/hashicorp/go-multierror"
)

// MultiChainProvider is a chain provider that can combine multiple third party providers into a single one,
// potentially using a boltz chain provider as a fallback.
// Its a trust-minimized implementation where boltz API is only ever trusted if every single provider
// is not reachable.
type MultiChainProvider struct {
	ChainProvider
	Providers []ChainProvider
	Boltz     ChainProvider
}

var _ ChainProvider = &MultiChainProvider{}

func (m MultiChainProvider) allProviders() []ChainProvider {
	if m.Boltz == nil {
		return m.Providers
	}
	return append(m.Providers, m.Boltz)
}

func (m MultiChainProvider) GetRawTransaction(txId string) (hex string, err error) {
	for _, provider := range m.allProviders() {
		hex, err = provider.GetRawTransaction(txId)
		if err == nil {
			return hex, nil
		}
	}
	return "", err
}

func (m MultiChainProvider) BroadcastTransaction(txHex string) (txId string, err error) {
	var group sync.WaitGroup
	var merr multierror.Error
	var mutex sync.Mutex
	providers := m.allProviders()
	group.Add(len(providers))
	// Broadcasting transactions via all known (including boltz) providers is fine
	for _, provider := range providers {
		provider := provider
		go func() {
			defer group.Done()
			result, err := provider.BroadcastTransaction(txHex)
			mutex.Lock()
			defer mutex.Unlock()

			if err == nil {
				txId = result
			} else {
				logger.Debugf("Error broadcasting transaction via %s: %v", provider, err)
				merr.Errors = append(merr.Errors, err)
			}
		}()
	}
	group.Wait()
	if len(providers) == len(merr.Errors) {
		return "", &merr
	}
	return txId, nil
}

func (m MultiChainProvider) IsTransactionConfirmed(txId string) (confirmed bool, err error) {
	var result bool
	var merr multierror.Error
	// It's important to not include boltz by default here, and first try all providers
	// on their own since it could always return true otherwise, making the different providers useless.
	for _, provider := range m.Providers {
		confirmed, err := provider.IsTransactionConfirmed(txId)
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		} else if confirmed {
			result = confirmed
		}
	}
	if len(merr.Errors) == len(m.Providers) {
		if m.Boltz != nil {
			logger.Warnf("All providers failed to check if transaction (%s) is confirmed, falling back to boltz: %v", txId, &merr)
			return m.Boltz.IsTransactionConfirmed(txId)
		}
		return result, fmt.Errorf("all providers failed: %v", &merr)
	}
	return result, nil
}

func (m MultiChainProvider) GetBlockHeight() (uint32, error) {
	var result uint32
	var merr multierror.Error
	// while the block height itself isn't crucial in itself, we don't include
	// boltz by default since it could cause issues with the IsTransactionConfirmed call
	// where the other providers aren't yet synced up to the height boltz returned.
	for _, provider := range m.Providers {
		height, err := provider.GetBlockHeight()
		if err == nil {
			result = max(result, height)
		} else {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if len(merr.Errors) == len(m.Providers) {
		if m.Boltz != nil {
			logger.Warnf("All providers failed to get block height, falling back to boltz: %v", &merr)
			return m.Boltz.GetBlockHeight()
		}
		return result, fmt.Errorf("all providers failed: %v", &merr)
	}
	return result, nil
}

func (m MultiChainProvider) EstimateFee() (float64, error) {
	for _, provider := range m.allProviders() {
		fee, err := provider.EstimateFee()
		if err == nil {
			return fee, nil
		}
	}
	return 0, errors.New("no fee found")
}

func (m MultiChainProvider) GetUnspentOutputs(address string) ([]*Output, error) {
	var merr multierror.Error
	providers := m.allProviders()
	for _, provider := range providers {
		outputs, err := provider.GetUnspentOutputs(address)
		if err == nil {
			if len(outputs) > 0 {
				return outputs, nil
			}
		} else {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if len(merr.Errors) == len(providers) {
		return nil, fmt.Errorf("all providers failed: %v", &merr)
	}
	return []*Output{}, nil
}

func (m MultiChainProvider) Disconnect() {
	for _, provider := range m.allProviders() {
		provider.Disconnect()
	}
}
