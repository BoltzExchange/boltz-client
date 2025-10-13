package onchain

import (
	"errors"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type BoltzProvider struct {
	*boltz.Api
	currency boltz.Currency
}

var _ ChainProvider = &BoltzProvider{}

func NewBoltzChainProvider(boltz *boltz.Api, currency boltz.Currency) *BoltzProvider {
	return &BoltzProvider{boltz, currency}
}

func (txProvider BoltzProvider) GetRawTransaction(txId string) (string, error) {
	return txProvider.GetTransaction(txId, txProvider.currency)
}

func (txProvider BoltzProvider) BroadcastTransaction(txHex string) (string, error) {
	return txProvider.Api.BroadcastTransaction(txProvider.currency, txHex)
}

func (txProvider BoltzProvider) IsTransactionConfirmed(txId string) (bool, error) {
	transaction, err := txProvider.GetTransactionDetails(txId, txProvider.currency)
	if err != nil {
		return false, err
	}
	return transaction.Confirmations > 0, nil
}

func (txProvider BoltzProvider) EstimateFee() (float64, error) {
	return txProvider.Api.EstimateFee(txProvider.currency)
}

func (txProvider BoltzProvider) GetBlockHeight() (uint32, error) {
	return txProvider.Api.GetBlockHeight(txProvider.currency)
}

func (txProvider BoltzProvider) GetUnspentOutputs(address string) ([]*Output, error) {
	return nil, errors.ErrUnsupported
}

func (txProvider BoltzProvider) Disconnect() {}
