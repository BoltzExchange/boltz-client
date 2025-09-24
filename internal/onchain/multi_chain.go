package onchain

import (
	"errors"
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
	for _, provider := range m.Providers {
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
	providers := m.Providers
	group.Add(len(providers))
	// using boltz all the time here is fine since it would be a anyways and it doesn't change the trust profile
	for _, provider := range providers {
		provider := provider
		go func() {
			defer group.Done()
			result, err := provider.BroadcastTransaction(txHex)
			if err == nil {
				txId = result
			} else {
				logger.Debugf("Error broadcasting transaction: %v", err)
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
	// its important to not include boltz by default here, and first try all providers
	// on their own since it could always return true otherwise, making the different providers useless.
	for _, provider := range m.Providers {
		confirmed, err := provider.IsTransactionConfirmed(txId)
		if err == nil && confirmed {
			result = confirmed
		} else {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if len(merr.Errors) == len(m.Providers) {
		logger.Warnf("All providers failed, trying fallback: %v", &merr)
		return m.Boltz.IsTransactionConfirmed(txId)
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
		logger.Warnf("All providers failed, trying fallback: %v", &merr)
		if m.Boltz != nil {
			return m.Boltz.GetBlockHeight()
		}
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

func (m MultiChainProvider) Disconnect() {
	for _, provider := range m.allProviders() {
		provider.Disconnect()
	}
}
