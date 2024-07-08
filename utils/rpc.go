package utils

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
)

func ParseCurrency(grpcCurrency *boltzrpc.Currency) boltz.Currency {
	if grpcCurrency == nil {
		return ""
	} else if *grpcCurrency == boltzrpc.Currency_BTC {
		return boltz.CurrencyBtc
	} else {
		return boltz.CurrencyLiquid
	}
}

func ParsePair(grpcPair *boltzrpc.Pair) (pair boltz.Pair) {
	if grpcPair == nil {
		return boltz.PairBtc
	}
	return boltz.Pair{
		From: ParseCurrency(&grpcPair.From),
		To:   ParseCurrency(&grpcPair.To),
	}
}

func SerializeCurrency(currency boltz.Currency) boltzrpc.Currency {
	if currency == boltz.CurrencyBtc {
		return boltzrpc.Currency_BTC
	} else {
		return boltzrpc.Currency_LBTC
	}
}

func SerializePair(pair boltz.Pair) *boltzrpc.Pair {
	return &boltzrpc.Pair{
		From: SerializeCurrency(pair.From),
		To:   SerializeCurrency(pair.To),
	}
}
