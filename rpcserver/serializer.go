package rpcserver

import (
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
)

func serializeSwap(swap *database.Swap) *boltzrpc.SwapInfo {
	serializedSwap := swap.Serialize()

	return &boltzrpc.SwapInfo{
		Id:                  serializedSwap.Id,
		Status:              serializedSwap.Status,
		PrivateKey:          serializedSwap.PrivateKey,
		Preimage:            serializedSwap.Preimage,
		RedeemScript:        serializedSwap.RedeemScript,
		Invoice:             serializedSwap.Invoice,
		LockupAddress:       serializedSwap.Address,
		ExpectedAmount:      int64(serializedSwap.ExpectedAmount),
		TimeoutBlockHeight:  uint32(serializedSwap.TimeoutBlockHeight),
		LockupTransactionId: serializedSwap.LockupTransactionId,
		RefundTransactionId: serializedSwap.RefundTransactionId,
	}
}

func serializeChannelCreation(channelCreation *database.ChannelCreation) *boltzrpc.ChannelCreationInfo {
	serializedChannelCreation := channelCreation.Serialize()

	return &boltzrpc.ChannelCreationInfo{
		SwapId:                 serializedChannelCreation.SwapId,
		Status:                 serializedChannelCreation.Status,
		InboundLiquidity:       uint32(serializedChannelCreation.InboundLiquidity),
		Private:                serializedChannelCreation.Private,
		FundingTransactionId:   serializedChannelCreation.FundingTransactionId,
		FundingTransactionVout: uint32(serializedChannelCreation.FundingTransactionVout),
	}
}

func serializeReverseSwap(reverseSwap *database.ReverseSwap) *boltzrpc.ReverseSwapInfo {
	serializedReverseSwap := reverseSwap.Serialize()

	return &boltzrpc.ReverseSwapInfo{
		Id:                 serializedReverseSwap.Id,
		Status:             serializedReverseSwap.Status,
		PrivateKey:         serializedReverseSwap.PrivateKey,
		Preimage:           serializedReverseSwap.Preimage,
		RedeemScript:       serializedReverseSwap.RedeemScript,
		Invoice:            serializedReverseSwap.Invoice,
		ClaimAddress:       serializedReverseSwap.ClaimAddress,
		OnchainAmount:      int64(serializedReverseSwap.OnchainAmount),
		TimeoutBlockHeight: uint32(serializedReverseSwap.TimeoutBlockHeight),
	}
}
