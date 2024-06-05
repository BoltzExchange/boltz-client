package autoswap

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
)

type ChainRecommendation = checks

type ChainSwapper struct {
	common
	rpc RpcProvider

	cfg *ChainConfig
}

func (swapper *ChainSwapper) setConfig(cfg *ChainConfig) {
	logger.Debugf("Setting chain autoswap config: %+v", cfg)
	logger.Infof("Chain autoswap (%s): %s", cfg.entity.Name, cfg.description)

	swapper.cfg = cfg
	if cfg.Enabled {
		swapper.Start()
	} else {
		swapper.Stop()
	}
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

	budget, err := swapper.GetCurrentBudget(true)
	if err != nil {
		return nil, fmt.Errorf("could not get current budget: %w", err)
	}

	cfg := swapper.cfg
	if balance.Confirmed > cfg.FromThreshold {
		amount := balance.Confirmed - (cfg.FromThreshold / 2)

		checked := check(amount, checkParams{Pair: pairInfo, MaxFeePercent: cfg.maxFeePercent, Budget: &budget.Amount})

		state := boltzrpc.SwapState_PENDING
		pendingSwaps, err := swapper.database.QueryChainSwaps(database.SwapQuery{
			State:    &state,
			EntityId: &swapper.cfg.entity.Id,
		})
		if err != nil {
			return nil, fmt.Errorf("could not query pending swaps: %w", err)
		}
		if len(pendingSwaps) > 0 {
			checked.Dismiss(ReasonPendingSwap)
		}
		return &checked, nil
	}
	return nil, nil
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
		if err := swapper.cfg.Init(); err != nil {
			swapper.err = fmt.Errorf("chain config has become invalid: %w", err)
		} else {
			swapper.Start()
		}
	}
}

func (swapper *ChainSwapper) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	return swapper.common.GetCurrentBudget(
		createIfMissing,
		swapper.cfg,
		swapper.cfg.entity.Id,
	)
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
