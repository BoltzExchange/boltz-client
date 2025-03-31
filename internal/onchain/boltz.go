package onchain

import (
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type BoltzTxProvider struct {
	*boltz.Api
	currency boltz.Currency
}

func NewBoltzTxProvider(boltz *boltz.Api, currency boltz.Currency) TxProvider {
	return &BoltzTxProvider{boltz, currency}
}

func (txProvider BoltzTxProvider) GetRawTransaction(txId string) (string, error) {
	return txProvider.GetTransaction(txId, txProvider.currency)
}

func (txProvider BoltzTxProvider) BroadcastTransaction(txHex string) (string, error) {
	return txProvider.Api.BroadcastTransaction(txProvider.currency, txHex)
}

func (txProvider BoltzTxProvider) IsTransactionConfirmed(txId string) (bool, error) {
	transaction, err := txProvider.Api.GetTransactionDetails(txId, txProvider.currency)
	if err != nil {
		return false, err
	}
	return transaction.Confirmations > 0, nil
}
