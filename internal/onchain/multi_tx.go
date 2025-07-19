package onchain

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"golang.org/x/sync/errgroup"
)

type MultiTxProvider struct {
	TxProvider
	Providers []TxProvider
}

func (m MultiTxProvider) GetRawTransaction(txId string) (hex string, err error) {
	for _, provider := range m.Providers {
		hex, err = provider.GetRawTransaction(txId)
		if err == nil {
			return hex, nil
		}
	}
	return "", err
}

func (m MultiTxProvider) BroadcastTransaction(txHex string) (txId string, err error) {
	var group errgroup.Group
	for _, provider := range m.Providers {
		provider := provider
		group.Go(func() error {
			result, err := provider.BroadcastTransaction(txHex)
			if err == nil {
				txId = result
				return nil
			}
			logger.Debugf("Error broadcasting transaction: %v", err)
			return err
		})
	}
	err = group.Wait()
	// only error if all providers failed
	if txId == "" {
		return "", err
	}
	return txId, nil
}

func (m MultiTxProvider) IsTransactionConfirmed(txId string) (confirmed bool, err error) {
	for _, provider := range m.Providers {
		confirmed, err = provider.IsTransactionConfirmed(txId)
		if err == nil {
			return confirmed, nil
		} else {
			logger.Debugf("Error checking transaction confirmation: %v", err)
		}
	}
	return false, err
}
