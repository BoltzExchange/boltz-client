package onchain

import (
	"github.com/BoltzExchange/boltz-client/boltz"
)

type boltzTxProvider struct {
	*boltz.Boltz
	currency boltz.Currency
}

func NewBoltzTxProvider(boltz *boltz.Boltz, currency boltz.Currency) TxProvider {
	return &boltzTxProvider{boltz, currency}
}

func (txProvider boltzTxProvider) GetRawTransaction(txId string) (string, error) {
	return txProvider.GetTransaction(txId, txProvider.currency)
}

func (txProvider boltzTxProvider) BroadcastTransaction(txHex string) (string, error) {
	return txProvider.Boltz.BroadcastTransaction(txProvider.currency, txHex)
}
