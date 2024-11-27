package serializers

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"
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
	if currency == boltz.NormalSwap {
		return boltzrpc.SwapType_SUBMARINE
	} else if currency == boltz.ReverseSwap {
		return boltzrpc.SwapType_REVERSE
	} else {
		return boltzrpc.SwapType_CHAIN
	}
}

func SerializePair(pair boltz.Pair) *boltzrpc.Pair {
	return &boltzrpc.Pair{
		From: SerializeCurrency(pair.From),
		To:   SerializeCurrency(pair.To),
	}
}

func SerializeChanId(chanId lightning.ChanId) *boltzrpc.ChannelId {
	if chanId != 0 {
		return &boltzrpc.ChannelId{
			Cln: chanId.ToCln(),
			Lnd: chanId.ToLnd(),
		}
	}
	return nil
}

func SerializeChanIds(chanIds []lightning.ChanId) (result []*boltzrpc.ChannelId) {
	for _, chanId := range chanIds {
		result = append(result, SerializeChanId(chanId))
	}
	return result
}

func SerializeLightningChannel(channel *lightning.LightningChannel) *boltzrpc.LightningChannel {
	if channel == nil {
		return nil
	}
	return &boltzrpc.LightningChannel{
		Id:          SerializeChanId(channel.Id),
		Capacity:    channel.Capacity,
		OutboundSat: channel.OutboundSat,
		InboundSat:  channel.InboundSat,
		PeerId:      channel.PeerId,
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
