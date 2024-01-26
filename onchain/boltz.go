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

func (boltz boltzTxProvider) GetTxHex(txId string) (string, error) {
	return boltz.GetTransaction(txId, boltz.currency)
}
