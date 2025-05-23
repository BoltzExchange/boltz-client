package autoswap

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"
)

func serializeLightningChannel(channel *lightning.LightningChannel) *boltzrpc.LightningChannel {
	if channel == nil {
		return nil
	}
	return &boltzrpc.LightningChannel{
		Id:          lightning.SerializeChanId(channel.Id),
		Capacity:    channel.Capacity,
		OutboundSat: channel.OutboundSat,
		InboundSat:  channel.InboundSat,
		PeerId:      channel.PeerId,
	}
}

func serializeLightningSwap(swap *LightningSwap) *autoswaprpc.LightningSwap {
	if swap == nil {
		return nil
	}
	return &autoswaprpc.LightningSwap{
		Amount:           swap.Amount,
		Type:             serializers.SerializeSwapType(swap.Type),
		FeeEstimate:      swap.FeeEstimate,
		DismissedReasons: swap.DismissedReasons,
	}
}

func serializeAutoChainSwap(swap *ChainSwap) *autoswaprpc.ChainSwap {
	if swap == nil {
		return nil
	}
	return &autoswaprpc.ChainSwap{
		Amount:           swap.Amount,
		FeeEstimate:      swap.FeeEstimate,
		DismissedReasons: swap.DismissedReasons,
	}
}
