package autoswap

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/lightning"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/utils"
)

type rawRecommendation struct {
	Type    boltz.SwapType
	Amount  uint64
	Channel *lightning.LightningChannel
}

type SwapRecommendation struct {
	rawRecommendation
	FeeEstimate      uint64
	DismissedReasons []string
}

const (
	ReasonMaxFeePercent  = "fee exceeds maximum percentage"
	ReasonAmountBelowMin = "amount below minimal"
	ReasonBudgetExceeded = "budget exceeded"
	ReasonPendingSwap    = "pending swap"
	ReasonFailedSwap     = "failed swap"
)

func (recommendation *SwapRecommendation) Dismiss(reason string) {
	recommendation.DismissedReasons = append(recommendation.DismissedReasons, reason)
}

func (recommendation *SwapRecommendation) Dismissed() bool {
	return len(recommendation.DismissedReasons) > 0
}

func (recommendation rawRecommendation) estimateFee(fees *boltzrpc.Fees) uint64 {
	serviceFee := utils.Percentage(fees.Percentage).Calculate(float64(recommendation.Amount))

	var onchainFee uint32
	if recommendation.Type == boltz.NormalSwap {
		onchainFee = fees.Miner.Normal
	} else if recommendation.Type == boltz.ReverseSwap {
		onchainFee = fees.Miner.Reverse
	}

	return uint64(serviceFee) + uint64(onchainFee)
}

func (recommendation rawRecommendation) Check(fees *boltzrpc.Fees, limits *boltzrpc.Limits, cfg *Config) *SwapRecommendation {
	var dismissedReasons []string

	if recommendation.Amount < uint64(limits.Minimal) {
		dismissedReasons = append(dismissedReasons, ReasonAmountBelowMin)
	}
	recommendation.Amount = uint64(math.Min(float64(recommendation.Amount), float64(limits.Maximal)))

	maxFee := cfg.MaxFeePercent.Calculate(float64(recommendation.Amount))
	fee := recommendation.estimateFee(fees)
	if float64(fee) > maxFee {
		dismissedReasons = append(dismissedReasons, ReasonMaxFeePercent)
	}

	return &SwapRecommendation{
		rawRecommendation: recommendation,
		FeeEstimate:       fee,
		DismissedReasons:  dismissedReasons,
	}
}

func (recommendation *SwapRecommendation) String() string {
	return fmt.Sprintf("Type:%v Amount:%v ChanId:%v FeeEstimate:%v DismissedReasons:%v", recommendation.Type, recommendation.Amount, recommendation.Channel.GetId(), recommendation.FeeEstimate, recommendation.DismissedReasons)
}
