package autoswap

import (
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
)

type Strategy = func(channels []*lightning.LightningChannel) []*lightningRecommendation

var channelBuffer uint64 = 10000

func (cfg *LightningConfig) channelRecommendation(channel *lightning.LightningChannel) *lightningRecommendation {
	outbound := cfg.outboundBalance.Get(channel.Capacity)
	inbound := cfg.inboundBalance.Get(channel.Capacity)

	if channel.Capacity < outbound+inbound {
		logger.Warnf("Capacity of channel %d is smaller than the sum of the outbound and inbound tresholds", channel.Id)
		return nil
	}

	recommendation := &lightningRecommendation{}
	if channel.Id != 0 {
		recommendation.Channel = channel
	}
	if channel.OutboundSat < outbound {
		recommendation.Type = boltz.NormalSwap
		if cfg.swapType == boltz.NormalSwap {
			recommendation.Amount = channel.InboundSat - channelBuffer
		}
	} else if channel.InboundSat < inbound {
		recommendation.Type = boltz.ReverseSwap
		if cfg.swapType == boltz.ReverseSwap {
			recommendation.Amount = channel.OutboundSat - channelBuffer
		}
	}
	if recommendation.Type != "" && cfg.Allowed(recommendation.Type) {
		if recommendation.Amount == 0 {
			target := float64(outbound+(channel.Capacity-inbound)) / 2
			recommendation.Amount = uint64(math.Abs(float64(channel.OutboundSat) - target))
		}
		return recommendation
	}
	return nil
}

func (cfg *LightningConfig) totalBalanceStrategy(channels []*lightning.LightningChannel) []*lightningRecommendation {
	var recommendations []*lightningRecommendation

	var total lightning.LightningChannel

	for _, channel := range channels {
		total.OutboundSat += channel.OutboundSat
		total.InboundSat += channel.InboundSat
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
