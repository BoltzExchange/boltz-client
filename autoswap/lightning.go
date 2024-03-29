package autoswap

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/database"
	"slices"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

type LightningSwapper struct {
	common
	rpc RpcProvider

	cfg *LightningConfig
}

func (swapper *LightningSwapper) setConfig(cfg *LightningConfig) {
	logger.Debugf("Setting auto swap config: %+v", cfg)
	message := fmt.Sprintf("Using %v strategy to recommend swaps", cfg.description)
	if cfg.swapType != "" {
		message += " of type " + string(cfg.swapType)
	}
	message += " for currency " + string(cfg.currency)

	logger.Info(message)
	swapper.cfg = cfg
	if cfg.Enabled {
		swapper.start()
	} else {
		swapper.Stop()
	}
}

func (swapper *LightningSwapper) getDismissedChannels() (DismissedChannels, error) {
	reasons := make(DismissedChannels)

	swaps, err := swapper.database.QueryPendingSwaps()
	if err != nil {
		return nil, errors.New("Could not query pending swaps: " + err.Error())
	}

	reverseSwaps, err := swapper.database.QueryPendingReverseSwaps()
	if err != nil {
		return nil, errors.New("Could not query pending reverse swaps: " + err.Error())
	}

	for _, swap := range swaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}
	for _, swap := range reverseSwaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}

	since := time.Now().Add(time.Duration(-swapper.cfg.FailureBackoff) * time.Second)
	failedSwaps, err := swapper.database.QueryFailedSwaps(since)
	if err != nil {
		return nil, errors.New("Could not query failed swaps: " + err.Error())
	}

	failedReverseSwaps, err := swapper.database.QueryFailedReverseSwaps(since)
	if err != nil {
		return nil, errors.New("Could not query failed reverse swaps: " + err.Error())
	}
	for _, swap := range failedSwaps {
		reasons.addChannels(swap.ChanIds, ReasonFailedSwap)
	}
	for _, swap := range failedReverseSwaps {
		reasons.addChannels(swap.ChanIds, ReasonFailedSwap)
	}

	return reasons, nil
}

type lightningRecommendation struct {
	Amount  uint64
	Type    boltz.SwapType
	Channel *lightning.LightningChannel
}

type LightningRecommendation struct {
	checks
	Type    boltz.SwapType
	Channel *lightning.LightningChannel
}

func (swapper *LightningSwapper) validateRecommendations(
	recommendations []*lightningRecommendation,
	budget int64,
) ([]*LightningRecommendation, error) {
	dismissedChannels, err := swapper.getDismissedChannels()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Dismissed channels: %v", dismissedChannels)

	// we might be able to fit more swaps in the budget if we sort by amount
	slices.SortFunc(recommendations, func(a, b *lightningRecommendation) int {
		return int(a.Amount - b.Amount)
	})

	var checked []*LightningRecommendation
	for _, recommendation := range recommendations {
		swapType := boltzrpc.SwapType_SUBMARINE
		if recommendation.Type == boltz.ReverseSwap {
			swapType = boltzrpc.SwapType_REVERSE
		}
		pairInfo, err := swapper.rpc.GetAutoSwapPairInfo(swapType, swapper.cfg.GetPair(recommendation.Type))
		if err != nil {
			logger.Warn("Could not get pair info: " + err.Error())
			continue
		}

		params := checkParams{
			MaxFeePercent:    swapper.cfg.maxFeePercent,
			Budget:           &budget,
			Pair:             pairInfo,
			DismissedReasons: dismissedChannels[recommendation.Channel.GetId()],
		}

		checked = append(checked, &LightningRecommendation{
			checks:  check(recommendation.Amount, params),
			Type:    recommendation.Type,
			Channel: recommendation.Channel,
		})
	}

	return checked, nil
}

func (swapper *LightningSwapper) GetSwapRecommendations() ([]*LightningRecommendation, error) {
	channels, err := swapper.rpc.GetLightningChannels()
	if err != nil {
		return nil, err
	}

	recommendations := swapper.cfg.strategy(channels)

	budget, err := swapper.GetCurrentBudget(true)
	if err != nil {
		return nil, errors.New("Could not get budget: " + err.Error())
	}

	logger.Debugf("Current autoswap budget: %+v", *budget)

	return swapper.validateRecommendations(recommendations, budget.Amount)
}

func (swapper *LightningSwapper) execute(recommendation *LightningRecommendation) error {
	var chanIds []string
	if chanId := recommendation.Channel.GetId(); chanId != 0 {
		chanIds = append(chanIds, chanId.ToCln())
	}
	pair := swapper.cfg.GetPair(recommendation.Type)
	var err error
	if recommendation.Type == boltz.ReverseSwap {
		err = swapper.rpc.CreateAutoReverseSwap(&database.DefaultEntity, &boltzrpc.CreateReverseSwapRequest{
			Amount:         recommendation.Amount,
			Address:        swapper.cfg.StaticAddress,
			AcceptZeroConf: swapper.cfg.AcceptZeroConf,
			Pair:           pair,
			ChanIds:        chanIds,
			WalletId:       swapper.cfg.walletId,
		})
	} else if recommendation.Type == boltz.NormalSwap {
		err = swapper.rpc.CreateAutoSwap(&database.DefaultEntity, &boltzrpc.CreateSwapRequest{
			Amount: recommendation.Amount,
			Pair:   pair,
			//ChanIds:          chanIds,
			SendFromInternal: true,
			WalletId:         swapper.cfg.walletId,
		})
	}
	return err
}

func (swapper *LightningSwapper) Restart() {
	swapper.Stop()
	if swapper.cfg.Enabled {
		if err := swapper.cfg.InitWallet(swapper.onchain); err != nil {
			logger.Errorf("Autoswap wallet configuration has become invalid: %s", err)
			return
		}
		swapper.start()
	}
}

func (swapper *LightningSwapper) start() {
	swapper.Stop()

	logger.Info("Starting auto swapper")

	swapper.stop = make(chan bool)
	go func() {
		ticker := time.NewTicker(time.Duration(swapper.cfg.ChannelPollInterval) * time.Second)

		for {
			recommendations, err := swapper.GetSwapRecommendations()
			if err != nil {
				logger.Warn("Could not fetch swap recommendations: " + err.Error())
			}
			if len(recommendations) > 0 {
				logger.Infof("Got %v swap recommendations", len(recommendations))
				for _, recommendation := range recommendations {
					if recommendation.Dismissed() {
						logger.Infof("Skipping swap recommendation %v because of %v", recommendation, recommendation.DismissedReasons)
						continue
					}

					logger.Infof("Executing Swap recommendation: %+v", recommendation)

					err := swapper.execute(recommendation)
					if err != nil {
						logger.Error("Could not act on swap recommendation : " + err.Error())
					}
				}
			}
			// wait for ticker after executing so that it runs immediately upon startup
			select {
			case <-ticker.C:
				continue
			case <-swapper.stop:
				return
			}
		}
	}()
}

func (swapper *LightningSwapper) GetConfig() *LightningConfig {
	return swapper.cfg
}
