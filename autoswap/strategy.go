package autoswap

import (
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
)

type Strategy = func(channels []*lightning.LightningChannel) []*rawRecommendation

func (cfg *Config) channelRecommendation(channel *lightning.LightningChannel) (recommendation *rawRecommendation) {
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
	if swapType != "" && (swapType == cfg.Type || cfg.Type == "") {
		target := float64(upper+lower) / 2
		recommendation = &rawRecommendation{
			Type:   swapType,
			Amount: uint64(math.Abs(float64(channel.LocalSat) - target)),
		}
		if channel.Id != 0 {
			recommendation.Channel = channel
		}
	}
	return recommendation
}

func (cfg *Config) totalBalanceStrategy(channels []*lightning.LightningChannel) []*rawRecommendation {
	var recommendations []*rawRecommendation

	var total lightning.LightningChannel

	for _, channel := range channels {
		total.LocalSat += channel.LocalSat
		total.RemoteSat += channel.RemoteSat
		total.Capacity += channel.Capacity
	}

	logger.Debugf("Total local channel balance: %d", total.LocalSat)

	recommendation := cfg.channelRecommendation(&total)
	if recommendation != nil {
		recommendations = append(recommendations, recommendation)
	}

	return recommendations
}

func (cfg *Config) perChannelStrategy(channels []*lightning.LightningChannel) []*rawRecommendation {
	var recommendations []*rawRecommendation

	for _, channel := range channels {
		recommendation := cfg.channelRecommendation(channel)
		if recommendation != nil {
			recommendations = append(recommendations, recommendation)
		}
	}

	return recommendations
}
