package serializers

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
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

func SerializeSwapType(currency boltz.SwapType) boltzrpc.SwapType {
	switch currency {
	case boltz.NormalSwap:
		return boltzrpc.SwapType_SUBMARINE
	case boltz.ReverseSwap:
		return boltzrpc.SwapType_REVERSE
	default:
		return boltzrpc.SwapType_CHAIN
	}
}

func ParseSwapType(grpcType boltzrpc.SwapType) boltz.SwapType {
	switch grpcType {
	case boltzrpc.SwapType_SUBMARINE:
		return boltz.NormalSwap
	case boltzrpc.SwapType_REVERSE:
		return boltz.ReverseSwap
	default:
		return boltz.ChainSwap
	}
}

func SerializePair(pair boltz.Pair) *boltzrpc.Pair {
	return &boltzrpc.Pair{
		From: SerializeCurrency(pair.From),
		To:   SerializeCurrency(pair.To),
	}
}

func SerializeWalletBalance(balance *onchain.Balance) *boltzrpc.Balance {
	if balance == nil {
		return nil
	}
	return &boltzrpc.Balance{
		Confirmed:   balance.Confirmed,
		Total:       balance.Total,
		Unconfirmed: balance.Unconfirmed,
	}
}
