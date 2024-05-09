package onchain

import (
	"github.com/BoltzExchange/boltz-client/boltz"
)

type boltzTxProvider struct {
	*boltz.Api
	currency boltz.Currency
}

func NewBoltzTxProvider(boltz *boltz.Api, currency boltz.Currency) TxProvider {
	return &boltzTxProvider{boltz, currency}
}

func (txProvider boltzTxProvider) GetRawTransaction(txId string) (string, error) {
	return txProvider.GetTransaction(txId, txProvider.currency)
}

func (txProvider boltzTxProvider) BroadcastTransaction(txHex string) (string, error) {
	return txProvider.Api.BroadcastTransaction(txProvider.currency, txHex)
}

func (txProvider boltzTxProvider) IsTransactionConfirmed(txId string) (bool, error) {
	return true, nil
}
