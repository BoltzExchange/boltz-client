package autoswap

import (
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"math"

	"github.com/BoltzExchange/boltz-client/boltz"
)

type Strategy = func(channels []*lightning.LightningChannel) []*rawRecommendation

func (cfg *Config) calculateSwapAmount(channel *lightning.LightningChannel) uint64 {
	if cfg.Type == boltz.ReverseSwap {
		reserve := cfg.LocalBalanceReserve.Calculate(float64(channel.Capacity))
		target := math.Max(float64(channel.RemoteSat), reserve)
		if channel.RemoteSat >= channel.LocalSat {
			target = reserve
		}
		return channel.LocalSat - uint64(target)
	}
	return uint64(math.Abs(float64(channel.LocalSat) - float64(channel.Capacity)/2))
}

func (cfg *Config) channelSwapType(channel *lightning.LightningChannel) *boltz.SwapType {
	threshold := cfg.ChannelImbalanceThreshold.Ratio()
	balancedness := float64(channel.LocalSat)/float64(channel.Capacity) - 0.5
	var swapType boltz.SwapType
	if balancedness < -threshold {
		swapType = boltz.NormalSwap
	} else if balancedness > threshold {
		swapType = boltz.ReverseSwap
	}
	if swapType != "" && (swapType == cfg.Type || cfg.Type == "") {
		return &swapType
	}
	return nil
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

	var recommend *boltz.SwapType
	if cfg.LocalBalanceThreshold != 0 {
		swapType := boltz.ReverseSwap
		if total.LocalSat > cfg.LocalBalanceThreshold {
			recommend = &swapType
		}
	} else {
		recommend = cfg.channelSwapType(&total)
	}
	if recommend != nil {
		recommendations = append(recommendations, &rawRecommendation{
			Type:   *recommend,
			Amount: cfg.calculateSwapAmount(&total),
		})
	}

	return recommendations
}

func (cfg *Config) perChannelStrategy(channels []*lightning.LightningChannel) []*rawRecommendation {
	var recommendations []*rawRecommendation

	for _, channel := range channels {
		swapType := cfg.channelSwapType(channel)
		if swapType != nil {
			recommendations = append(recommendations, &rawRecommendation{
				Type:    *swapType,
				Amount:  cfg.calculateSwapAmount(channel),
				Channel: channel,
			})
		}
	}

	return recommendations
}
