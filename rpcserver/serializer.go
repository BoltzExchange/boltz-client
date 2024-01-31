package rpcserver

import (
	"time"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
)

func serializeOptionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func serializeChanId(chanId lightning.ChanId) *boltzrpc.ChannelId {
	if chanId != 0 {
		return &boltzrpc.ChannelId{
			Cln: chanId.ToCln(),
			Lnd: chanId.ToLnd(),
		}
	}
	return nil
}

func serializeChanIds(chanIds []lightning.ChanId) (result []*boltzrpc.ChannelId) {
	for _, chanId := range chanIds {
		result = append(result, serializeChanId(chanId))
	}
	return result
}

func serializeSwap(swap *database.Swap) *boltzrpc.SwapInfo {
	if swap == nil {
		return nil
	}
	serializedSwap := swap.Serialize()

	return &boltzrpc.SwapInfo{
		Id:                  serializedSwap.Id,
		PairId:              serializedSwap.PairId,
		ChanIds:             serializeChanIds(swap.ChanIds),
		State:               swap.State,
		Error:               serializedSwap.Error,
		Status:              serializedSwap.Status,
		PrivateKey:          serializedSwap.PrivateKey,
		Preimage:            serializedSwap.Preimage,
		RedeemScript:        serializedSwap.RedeemScript,
		Invoice:             serializedSwap.Invoice,
		LockupAddress:       serializedSwap.Address,
		ExpectedAmount:      int64(serializedSwap.ExpectedAmount),
		TimeoutBlockHeight:  serializedSwap.TimeoutBlockHeight,
		LockupTransactionId: serializedSwap.LockupTransactionId,
		RefundTransactionId: serializedSwap.RefundTransactionId,
		RefundAddress:       serializeOptionalString(serializedSwap.RefundAddress),
		BlindingKey:         serializeOptionalString(serializedSwap.BlindingKey),
		CreatedAt:           serializeTime(swap.CreatedAt),
		ServiceFee:          serializedSwap.ServiceFee,
		OnchainFee:          serializedSwap.OnchainFee,
		AutoSend:            serializedSwap.AutoSend,
	}
}

func serializeReverseSwap(reverseSwap *database.ReverseSwap) *boltzrpc.ReverseSwapInfo {
	if reverseSwap == nil {
		return nil
	}
	serializedReverseSwap := reverseSwap.Serialize()

	return &boltzrpc.ReverseSwapInfo{
		Id:                  serializedReverseSwap.Id,
		PairId:              serializedReverseSwap.PairId,
		ChanIds:             serializeChanIds(reverseSwap.ChanIds),
		State:               reverseSwap.State,
		Error:               serializedReverseSwap.Error,
		Status:              serializedReverseSwap.Status,
		PrivateKey:          serializedReverseSwap.PrivateKey,
		Preimage:            serializedReverseSwap.Preimage,
		RedeemScript:        serializedReverseSwap.RedeemScript,
		Invoice:             serializedReverseSwap.Invoice,
		ClaimAddress:        serializedReverseSwap.ClaimAddress,
		OnchainAmount:       int64(serializedReverseSwap.OnchainAmount),
		TimeoutBlockHeight:  serializedReverseSwap.TimeoutBlockHeight,
		LockupTransactionId: serializedReverseSwap.LockupTransactionId,
		ClaimTransactionId:  serializedReverseSwap.ClaimTransactionId,
		BlindingKey:         serializeOptionalString(serializedReverseSwap.BlindingKey),
		CreatedAt:           serializeTime(reverseSwap.CreatedAt),
		ServiceFee:          serializedReverseSwap.ServiceFee,
		OnchainFee:          serializedReverseSwap.OnchainFee,
		RoutingFeeMsat:      serializedReverseSwap.RoutingFeeMsat,
	}
}

func serializeWalletBalance(balance *onchain.Balance) *boltzrpc.Balance {
	return &boltzrpc.Balance{
		Confirmed:   balance.Confirmed,
		Total:       balance.Total,
		Unconfirmed: balance.Unconfirmed,
	}
}

func serializewalletSubaccount(subaccount wallet.Subaccount, balance *onchain.Balance) *boltzrpc.Subaccount {
	return &boltzrpc.Subaccount{
		Balance: serializeWalletBalance(balance),
		Pointer: subaccount.Pointer,
		Type:    subaccount.Type,
	}
}

func serializeWalletCredentials(credentials *wallet.Credentials) *boltzrpc.WalletCredentials {
	return &boltzrpc.WalletCredentials{
		Mnemonic:       serializeOptionalString(credentials.Mnemonic),
		Xpub:           serializeOptionalString(credentials.Xpub),
		CoreDescriptor: serializeOptionalString(credentials.CoreDescriptor),
		Subaccount:     credentials.Subaccount,
	}
}

func serializeTime(t time.Time) int64 {
	return t.UTC().Unix()
}

func serializeLightningChannel(channel *lightning.LightningChannel) *boltzrpc.LightningChannel {
	if channel == nil {
		return nil
	}
	return &boltzrpc.LightningChannel{
		Id:        serializeChanId(channel.Id),
		Capacity:  channel.Capacity,
		LocalSat:  channel.LocalSat,
		RemoteSat: channel.RemoteSat,
		PeerId:    channel.PeerId,
	}
}
