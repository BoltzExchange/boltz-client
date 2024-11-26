package autoswap

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/serializers"
)

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
