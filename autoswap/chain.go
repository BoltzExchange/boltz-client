package autoswap

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
)

type ChainRecommendation = checks

type ChainSwapper struct {
	common
	rpc RpcProvider

	cfg *ChainConfig
}

func (swapper *ChainSwapper) setConfig(cfg *ChainConfig) {
	logger.Debugf("Setting auto chainswap config: %+v", cfg)
	logger.Info(cfg.description)

	swapper.cfg = cfg
	if cfg.Enabled {
		swapper.Start()
	} else {
		swapper.Stop()
	}
}

func (cfg *ChainConfig) getRecommendation(balance *onchain.Balance, pairInfo *boltzrpc.PairInfo) *ChainRecommendation {
	if balance.Confirmed > cfg.FromThreshold {
		amount := balance.Confirmed - (cfg.FromThreshold / 2)
		checked := check(amount, checkParams{Pair: pairInfo, MaxFeePercent: cfg.maxFeePercent})
		return &checked
	}
	return nil
}

func (swapper *ChainSwapper) GetRecommendation() (*ChainRecommendation, error) {
	balance, err := swapper.cfg.fromWallet.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("could not get wallet balance: %w", err)
	}

	pairInfo, err := swapper.rpc.GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, utils.SerializePair(swapper.cfg.pair))
	if err != nil {
		return nil, fmt.Errorf("could not get pair info: %w", err)
	}

	return swapper.cfg.getRecommendation(balance, pairInfo), nil
}

func (swapper *ChainSwapper) GetConfig() *ChainConfig {
	return swapper.cfg
}

func (swapper *ChainSwapper) execute(recommendation *ChainRecommendation) error {
	if recommendation != nil {
		if recommendation.Dismissed() {
			logger.Infof("Skipping swap recommendation %v because of %v", recommendation, recommendation.DismissedReasons)
			return nil
		}
		logger.Infof("Executing Swap recommendation: %+v", recommendation)
		fromWalletId := swapper.cfg.fromWallet.GetWalletInfo().Id
		request := &boltzrpc.CreateChainSwapRequest{
			Amount:       recommendation.Amount,
			Pair:         utils.SerializePair(swapper.cfg.pair),
			FromWalletId: &fromWalletId,
		}
		if swapper.cfg.ToAddress != "" {
			request.ToAddress = &swapper.cfg.ToAddress
		} else {
			toWalletId := swapper.cfg.toWallet.GetWalletInfo().Id
			request.ToWalletId = &toWalletId
		}
		return swapper.rpc.CreateAutoChainSwap(swapper.cfg.entity, swapper.cfg.Request(recommendation.Amount))
	}
	return nil
}

func (swapper *ChainSwapper) Restart() {
	if swapper.cfg.Enabled {
		swapper.Stop()
		if err := swapper.cfg.Init(swapper.database, swapper.onchain); err != nil {
			swapper.err = fmt.Errorf("chain config has become invalid: %w", err)
		} else {
			swapper.Start()
		}
	}
}

func (swapper *ChainSwapper) Start() {
	swapper.Stop()

	logger.Info("Starting chain swapper")

	swapper.stop = make(chan bool)
	go func() {
		updates, stop := swapper.rpc.GetBlockUpdates(swapper.cfg.pair.From)
		defer stop()
		for {
			select {
			case <-swapper.stop:
				return
			case _, ok := <-updates:
				if ok {
					logger.Debugf("Checking for chain swap recommendation")
					recommendation, err := swapper.GetRecommendation()
					if err != nil {
						logger.Warn("Could not get swap recommendation: " + err.Error())
						continue
					}

					if err := swapper.execute(recommendation); err != nil {
						logger.Errorf("Could not act on swap recommendation: %s", err)
					}
				}
			}
		}
	}()
}
