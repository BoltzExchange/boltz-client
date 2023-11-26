package autoswap

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/BurntSushi/toml"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
)

type SwapExecution struct {
	Amount  uint64
	Channel *boltzrpc.LightningChannel
	Pair    boltz.Pair
}

type AutoSwapper struct {
	cfg        *Config
	onchain    *onchain.Onchain
	database   *database.Database
	stop       chan bool
	configPath string
	err        error

	ExecuteSwap        func(request *boltzrpc.CreateSwapRequest) error
	ExecuteReverseSwap func(request *boltzrpc.CreateReverseSwapRequest) error
	ListChannels       func() ([]*lightning.LightningChannel, error)
	GetServiceInfo     func(pair boltz.Pair) (*boltzrpc.Fees, *boltzrpc.Limits, error)
}

func (swapper *AutoSwapper) Init(database *database.Database, onchain *onchain.Onchain, configPath string) {
	swapper.onchain = onchain
	swapper.database = database
	swapper.configPath = configPath
}

func (swapper *AutoSwapper) saveConfig() error {
	return swapper.cfg.Write(swapper.configPath)
}

func (swapper *AutoSwapper) SetConfigValue(field string, value string) error {
	logger.Debugf("Setting auto swap config field %v to %v", field, value)
	cfg := *swapper.cfg
	if err := cfg.SetValue(field, value); err != nil {
		return err
	}
	return swapper.SetConfig(&cfg)
}

func (swapper *AutoSwapper) SetConfig(cfg *Config) error {
	logger.Debugf("Setting auto swap config: %+v", cfg)
	if err := cfg.Init(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	message := fmt.Sprintf("Using %v strategy to recommend swaps", cfg.strategyName)
	if cfg.Type != "" {
		message += " of type " + string(cfg.Type)
	}
	message += " for pair " + string(cfg.Pair)

	logger.Info(message)
	swapper.cfg = cfg
	if cfg.Enabled {
		if err := swapper.Start(); err != nil {
			logger.Error("Could not start auto swapper: " + err.Error())
		}
	} else {
		swapper.Stop()
	}
	return swapper.saveConfig()
}

func (swapper *AutoSwapper) LoadConfig() error {
	var cfg Config
	var err error

	if !utils.FileExists(swapper.configPath) {
		logger.Info("Got no autoswap configuration, loading defaults")
		cfg = DefaultConfig
	} else {
		if _, err = toml.DecodeFile(swapper.configPath, &cfg); err != nil {
			err = fmt.Errorf("Could not decode autoswap config: " + err.Error())
		}
	}

	if err == nil {
		err = swapper.SetConfig(&cfg)
	}
	// only set error if we dont have a config yet
	if err != nil && swapper.cfg == nil {
		swapper.err = err
	}
	return err
}

func (swapper *AutoSwapper) Configured() bool {
	return swapper.cfg != nil
}

func (swapper *AutoSwapper) GetConfig() Config {
	return *swapper.cfg
}

func (swapper *AutoSwapper) getDismissedChannels() (DismissedChannels, error) {
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
		reasons.addChannel(swap.ChanId, ReasonPendingSwap)
	}
	for _, swap := range reverseSwaps {
		reasons.addChannel(swap.ChanId, ReasonPendingSwap)
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
		reasons.addChannel(swap.ChanId, ReasonFailedSwap)
	}
	for _, swap := range failedReverseSwaps {
		reasons.addChannel(swap.ChanId, ReasonFailedSwap)
	}

	return reasons, nil
}

func (swapper *AutoSwapper) validateRecommendations(
	recommendations []*rawRecommendation,
	budget int64,
) ([]*SwapRecommendation, error) {
	fees, limits, err := swapper.GetServiceInfo(swapper.cfg.Pair)
	if err != nil {
		return nil, err
	}

	dismissedChannels, err := swapper.getDismissedChannels()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Dismissed channels: %v", dismissedChannels)

	// we might be able to fit more swaps in the budget if we sort by amount
	slices.SortFunc(recommendations, func(a, b *rawRecommendation) int {
		return int(a.Amount - b.Amount)
	})

	var checked []*SwapRecommendation
	for _, recommendation := range recommendations {
		recommendation := recommendation.Check(fees, limits, swapper.cfg)
		reasons, ok := dismissedChannels[recommendation.Channel.GetId()]
		if ok {
			recommendation.DismissedReasons = append(recommendation.DismissedReasons, reasons...)
		}
		if len(recommendation.DismissedReasons) == 0 {
			budget -= int64(recommendation.FeeEstimate)
		}
		if budget < 0 {
			recommendation.Dismiss(ReasonBudgetExceeded)
		}
		checked = append(checked, recommendation)
	}

	return checked, nil
}

func (swapper *AutoSwapper) GetSwapRecommendations() ([]*SwapRecommendation, error) {
	channels, err := swapper.ListChannels()
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

func (swapper *AutoSwapper) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	budgetPeriod, err := swapper.database.QueryCurrentBudgetPeriod()
	if err != nil {
		return nil, errors.New("Could not get budget period: " + err.Error())
	}

	now := time.Now()
	if budgetPeriod == nil || now.After(budgetPeriod.EndDate) {
		if createIfMissing {
			budgetDuration := time.Duration(swapper.cfg.AutoBudgetPeriod) * time.Second
			if budgetPeriod == nil {
				budgetPeriod = &database.BudgetPeriod{
					StartDate: now,
					EndDate:   now.Add(budgetDuration),
				}
			}
			for now.After(budgetPeriod.EndDate) {
				budgetPeriod.StartDate = budgetPeriod.EndDate
				budgetPeriod.EndDate = budgetPeriod.EndDate.Add(budgetDuration)
			}
			if err := swapper.database.CreateBudget(*budgetPeriod); err != nil {
				return nil, errors.New("Could not create budget period: " + err.Error())
			}
		} else {
			return nil, nil
		}
	}

	stats, err := swapper.database.QueryStats(budgetPeriod.StartDate, true)
	if err != nil {
		return nil, errors.New("Could not get past fees: " + err.Error())
	}

	budget := int64(swapper.cfg.AutoBudget) - int64(stats.TotalFees)

	return &Budget{
		BudgetPeriod: *budgetPeriod,
		Amount:       budget,
		Total:        swapper.cfg.AutoBudget,
	}, nil
}

func (swapper *AutoSwapper) execute(recommendation *SwapRecommendation, address string) error {
	pair := string(swapper.cfg.Pair)
	chanId := recommendation.Channel.GetId()
	rawChanid := chanId.ToCln()
	var err error
	if recommendation.Type == boltz.ReverseSwap {
		err = swapper.ExecuteReverseSwap(&boltzrpc.CreateReverseSwapRequest{
			Amount:         int64(recommendation.Amount),
			Address:        address,
			AcceptZeroConf: swapper.cfg.AcceptZeroConf,
			PairId:         pair,
			ChanId:         &rawChanid,
		})
	} else if recommendation.Type == boltz.NormalSwap {
		err = swapper.ExecuteSwap(&boltzrpc.CreateSwapRequest{
			Amount:   int64(recommendation.Amount),
			PairId:   pair,
			ChanId:   &rawChanid,
			AutoSend: true,
		})
	}
	return err
}

func (swapper *AutoSwapper) Running() bool {
	return swapper.stop != nil
}

func (swapper *AutoSwapper) Error() string {
	if swapper.err != nil {
		return swapper.err.Error()
	}
	return ""
}

func (swapper *AutoSwapper) Start() error {
	swapper.Stop()

	logger.Info("Starting auto swapper")

	cfg := swapper.cfg
	address, err := cfg.GetAddress(swapper.onchain.Network)
	if err != nil {
		logger.Info(err.Error())
	}
	wallet, err := swapper.onchain.GetWallet(cfg.Pair)
	normalSwaps := cfg.Type == "" || cfg.Type == boltz.NormalSwap
	if wallet != nil {
		if normalSwaps || address == "" {
			address, err = wallet.NewAddress()
			if err != nil {
				err = errors.New("could not get external address: " + err.Error())
			}
		}
	} else if address == "" {
		err = fmt.Errorf("neither external address or wallet is available for pair %s: %v", cfg.Pair, err)
	} else if normalSwaps {
		err = fmt.Errorf("normal swaps require a wallet: %v", err)
	}

	swapper.err = err
	if err != nil {
		return err
	}

	swapper.stop = make(chan bool)
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.ChannelPollInterval) * time.Second)

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

					logger.Infof("Executing Swap recommendation: %v", recommendation)

					err := swapper.execute(recommendation, address)
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
	return nil
}

func (swapper *AutoSwapper) Stop() {
	if swapper.stop != nil {
		logger.Info("Stopping auto swapper")
		swapper.stop <- true
		swapper.stop = nil
		swapper.err = nil
	}
}
