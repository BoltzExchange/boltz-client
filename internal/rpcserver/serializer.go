package rpcserver

import (
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
)

func serializeOptionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func serializeCurrency(currency boltz.Currency) boltzrpc.Currency {
	if currency == boltz.CurrencyBtc {
		return boltzrpc.Currency_BTC
	} else {
		return boltzrpc.Currency_LBTC
	}
}

func serializePair(pair boltz.Pair) *boltzrpc.Pair {
	return &boltzrpc.Pair{
		From: serializeCurrency(pair.From),
		To:   serializeCurrency(pair.To),
	}
}

func serializeSwap(swap *database.Swap) *boltzrpc.SwapInfo {
	if swap == nil {
		return nil
	}
	serializedSwap := swap.Serialize()

	serialized := &boltzrpc.SwapInfo{
		Id:                  serializedSwap.Id,
		Pair:                serializePair(swap.Pair),
		ChanIds:             lightning.SerializeChanIds(swap.ChanIds),
		State:               swap.State,
		Error:               serializedSwap.Error,
		Status:              serializedSwap.Status,
		PrivateKey:          serializedSwap.PrivateKey,
		Preimage:            serializedSwap.Preimage,
		RedeemScript:        serializedSwap.RedeemScript,
		Invoice:             serializedSwap.Invoice,
		LockupAddress:       serializedSwap.Address,
		ExpectedAmount:      serializedSwap.ExpectedAmount,
		TimeoutBlockHeight:  serializedSwap.TimeoutBlockHeight,
		LockupTransactionId: serializedSwap.LockupTransactionId,
		RefundTransactionId: serializedSwap.RefundTransactionId,
		RefundAddress:       serializeOptionalString(serializedSwap.RefundAddress),
		BlindingKey:         serializeOptionalString(serializedSwap.BlindingKey),
		CreatedAt:           serializeTime(swap.CreatedAt),
		ServiceFee:          serializedSwap.ServiceFee,
		OnchainFee:          serializedSwap.OnchainFee,
		WalletId:            serializedSwap.WalletId,
		TenantId:            serializedSwap.TenantId,
		IsAuto:              serializedSwap.IsAuto,
	}

	return serialized
}

func serializeAnySwap(swap *database.AnySwap) *boltzrpc.AnySwapInfo {
	toAmount := swap.Amount
	if swap.OnchainFee != nil {
		toAmount -= *swap.OnchainFee
	}
	if swap.ServiceFee != nil {
		toAmount = uint64(int64(toAmount) - *swap.ServiceFee)
	}
	return &boltzrpc.AnySwapInfo{
		Id:         swap.Id,
		Type:       serializers.SerializeSwapType(swap.Type),
		Pair:       serializePair(swap.Pair),
		State:      swap.State,
		Error:      serializeOptionalString(swap.Error),
		Status:     swap.Status.String(),
		FromAmount: swap.Amount,
		ToAmount:   toAmount,
		CreatedAt:  serializeTime(swap.CreatedAt),
		ServiceFee: swap.ServiceFee,
		OnchainFee: swap.OnchainFee,
		IsAuto:     swap.IsAuto,
		TenantId:   swap.TenantId,
	}
}

func serializeReverseSwap(reverseSwap *database.ReverseSwap) *boltzrpc.ReverseSwapInfo {
	if reverseSwap == nil {
		return nil
	}
	serializedReverseSwap := reverseSwap.Serialize()

	return &boltzrpc.ReverseSwapInfo{
		Id:                  serializedReverseSwap.Id,
		Pair:                serializePair(reverseSwap.Pair),
		ChanIds:             lightning.SerializeChanIds(reverseSwap.ChanIds),
		State:               reverseSwap.State,
		Error:               serializedReverseSwap.Error,
		Status:              serializedReverseSwap.Status,
		PrivateKey:          serializedReverseSwap.PrivateKey,
		Preimage:            serializedReverseSwap.Preimage,
		RedeemScript:        serializedReverseSwap.RedeemScript,
		Invoice:             serializedReverseSwap.Invoice,
		ClaimAddress:        serializedReverseSwap.ClaimAddress,
		OnchainAmount:       serializedReverseSwap.OnchainAmount,
		InvoiceAmount:       serializedReverseSwap.InvoiceAmount,
		TimeoutBlockHeight:  serializedReverseSwap.TimeoutBlockHeight,
		LockupTransactionId: serializedReverseSwap.LockupTransactionId,
		ClaimTransactionId:  serializedReverseSwap.ClaimTransactionId,
		BlindingKey:         serializeOptionalString(serializedReverseSwap.BlindingKey),
		CreatedAt:           serializeTime(reverseSwap.CreatedAt),
		PaidAt:              serializeOptionalTime(reverseSwap.PaidAt),
		ServiceFee:          serializedReverseSwap.ServiceFee,
		OnchainFee:          serializedReverseSwap.OnchainFee,
		RoutingFeeMsat:      serializedReverseSwap.RoutingFeeMsat,
		ExternalPay:         serializedReverseSwap.ExternalPay,
		TenantId:            serializedReverseSwap.TenantId,
		IsAuto:              serializedReverseSwap.IsAuto,
	}
}

func serializeChainSwap(chainSwap *database.ChainSwap) *boltzrpc.ChainSwapInfo {
	if chainSwap == nil {
		return nil
	}
	serializedChainSwap := chainSwap.Serialize()

	return &boltzrpc.ChainSwapInfo{
		Id:         serializedChainSwap.Id,
		Pair:       serializePair(chainSwap.Pair),
		State:      chainSwap.State,
		Error:      serializedChainSwap.Error,
		Status:     serializedChainSwap.Status,
		Preimage:   serializedChainSwap.Preimage,
		CreatedAt:  serializeTime(chainSwap.CreatedAt),
		ServiceFee: serializedChainSwap.ServiceFee,
		OnchainFee: serializedChainSwap.OnchainFee,
		FromData:   serializeChainSwapData(chainSwap.FromData),
		ToData:     serializeChainSwapData(chainSwap.ToData),
		TenantId:   chainSwap.TenantId,
		IsAuto:     serializedChainSwap.IsAuto,
	}
}

func serializeChainSwapData(chainSwap *database.ChainSwapData) *boltzrpc.ChainSwapData {
	if chainSwap == nil {
		return nil
	}
	serializedChainSwap := chainSwap.Serialize()

	return &boltzrpc.ChainSwapData{
		Id:                  serializedChainSwap.Id,
		Currency:            serializeCurrency(chainSwap.Currency),
		PrivateKey:          serializedChainSwap.PrivateKey,
		TimeoutBlockHeight:  serializedChainSwap.TimeoutBlockHeight,
		Address:             serializeOptionalString(serializedChainSwap.Address),
		Amount:              serializedChainSwap.Amount,
		LockupTransactionId: serializeOptionalString(serializedChainSwap.LockupTransactionId),
		TransactionId:       serializeOptionalString(serializedChainSwap.TransactionId),
		BlindingKey:         serializeOptionalString(serializedChainSwap.BlindingKey),
		LockupAddress:       serializedChainSwap.LockupAddress,
		WalletId:            serializedChainSwap.WalletId,
	}
}

func serializeSubmarinePair(pair boltz.Pair, submarinePair *boltz.SubmarinePair) *boltzrpc.PairInfo {
	return &boltzrpc.PairInfo{
		Pair: serializePair(pair),
		Hash: submarinePair.Hash,
		Fees: &boltzrpc.SwapFees{
			Percentage: submarinePair.Fees.Percentage,
			MinerFees:  submarinePair.Fees.MinerFees,
		},
		Limits: &boltzrpc.Limits{
			Minimal:               submarinePair.Limits.Minimal,
			Maximal:               submarinePair.Limits.Maximal,
			MaximalZeroConfAmount: submarinePair.Limits.MaximalZeroConfAmount,
		},
	}
}

func serializeReversePair(pair boltz.Pair, reversePair *boltz.ReversePair) *boltzrpc.PairInfo {
	miner := reversePair.Fees.MinerFees
	return &boltzrpc.PairInfo{
		Pair: serializePair(pair),
		Hash: reversePair.Hash,
		Fees: &boltzrpc.SwapFees{
			Percentage: reversePair.Fees.Percentage,
			MinerFees:  miner.Claim + miner.Lockup,
		},
		Limits: &boltzrpc.Limits{
			Minimal: reversePair.Limits.Minimal,
			Maximal: reversePair.Limits.Maximal,
		},
	}
}

func serializeChainPair(pair boltz.Pair, chainPair *boltz.ChainPair) *boltzrpc.PairInfo {
	miner := chainPair.Fees.MinerFees
	return &boltzrpc.PairInfo{
		Pair: serializePair(pair),
		Hash: chainPair.Hash,
		Fees: &boltzrpc.SwapFees{
			Percentage: chainPair.Fees.Percentage,
			MinerFees:  miner.Server + miner.User.Claim + miner.User.Lockup,
		},
		Limits: &boltzrpc.Limits{
			Minimal: chainPair.Limits.Minimal,
			Maximal: chainPair.Limits.Maximal,
		},
	}
}

func serializeWalletSubaccount(subaccount wallet.Subaccount, balance *onchain.Balance) *boltzrpc.Subaccount {
	return &boltzrpc.Subaccount{
		Balance:     serializers.SerializeWalletBalance(balance),
		Pointer:     subaccount.Pointer,
		Type:        subaccount.Type,
		Descriptors: subaccount.CoreDescriptors,
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

func serializeOptionalTime(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	unix := t.UTC().Unix()
	return &unix
}

func serializeTenant(tenant *database.Tenant) *boltzrpc.Tenant {
	if tenant == nil {
		return nil
	}
	return &boltzrpc.Tenant{
		Id:   tenant.Id,
		Name: tenant.Name,
	}
}
