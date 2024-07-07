package autoswap

import (
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
)

type Strategy = func(channels []*lightning.LightningChannel) []*lightningRecommendation

func (cfg *LightningConfig) channelRecommendation(channel *lightning.LightningChannel) (recommendation *lightningRecommendation) {
	upper := channel.Capacity
	lower := uint64(0)

	if !cfg.maxBalance.IsZero() {
		upper = cfg.maxBalance.Get(channel.Capacity)
	}
	if !cfg.minBalance.IsZero() {
		lower = cfg.minBalance.Get(channel.Capacity)
	}

	var swapType boltz.SwapType
	if channel.LocalSat > upper {
		swapType = boltz.ReverseSwap
	} else if channel.LocalSat < lower {
		swapType = boltz.NormalSwap
	}
	if swapType != "" && cfg.Allowed(swapType) {
		target := float64(upper+lower) / 2
		recommendation = &lightningRecommendation{
			Type:   swapType,
			Amount: uint64(math.Abs(float64(channel.LocalSat) - target)),
		}
		if channel.Id != 0 {
			recommendation.Channel = channel
		}
	}
	return recommendation
}

func (cfg *LightningConfig) totalBalanceStrategy(channels []*lightning.LightningChannel) []*lightningRecommendation {
	var recommendations []*lightningRecommendation

	var total lightning.LightningChannel

	for _, channel := range channels {
		total.LocalSat += channel.LocalSat
		total.RemoteSat += channel.RemoteSat
		total.Capacity += channel.Capacity
	}

	logger.Debugf("Total channel balances %+v", total)

	recommendation := cfg.channelRecommendation(&total)
	if recommendation != nil {
		recommendations = append(recommendations, recommendation)
	}

	return recommendations
}

func (cfg *LightningConfig) perChannelStrategy(channels []*lightning.LightningChannel) []*lightningRecommendation {
	var recommendations []*lightningRecommendation

	for _, channel := range channels {
		recommendation := cfg.channelRecommendation(channel)
		if recommendation != nil {
			recommendations = append(recommendations, recommendation)
		}
	}

	return recommendations
}
