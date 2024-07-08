package autoswap

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
	"slices"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

type LightningSwapper = swapper[*LightningConfig]

type SerializedLnConfig = autoswaprpc.LightningConfig

type LightningConfig struct {
	*SerializedLnConfig
	shared

	maxFeePercent   utils.Percentage
	currency        boltz.Currency
	swapType        boltz.SwapType
	outboundBalance Balance
	inboundBalance  Balance
	strategy        Strategy
	description     string
	walletId        *database.Id
}

func NewLightningConfig(serialized *SerializedLnConfig, shared shared) *LightningConfig {
	return &LightningConfig{SerializedLnConfig: withLightningBase(serialized), shared: shared}
}

func withLightningBase(config *SerializedLnConfig) *SerializedLnConfig {
	return merge(&SerializedLnConfig{
		FailureBackoff:      24 * 60 * 60,
		MaxFeePercent:       1,
		ChannelPollInterval: 30,
		Budget:              100000,
		BudgetInterval:      7 * 60 * 60 * 24,
	}, config)
}

func DefaultLightningConfig() *SerializedLnConfig {
	// we can't include values like currency in the base config
	// since we couldn't know wether the user didn't set the currency at all or set it to BTC
	return withLightningBase(&SerializedLnConfig{
		OutboundBalancePercent: 25,
		InboundBalancePercent:  25,
		Currency:               boltzrpc.Currency_LBTC,
	})
}

func (cfg *LightningConfig) Init() error {
	var err error
	cfg.swapType, err = boltz.ParseSwapType(cfg.SwapType)
	if err != nil {
		return fmt.Errorf("invalid swap type: %w", err)
	}

	cfg.currency = utils.ParseCurrency(&cfg.Currency)
	cfg.maxFeePercent = utils.Percentage(cfg.MaxFeePercent)
	cfg.outboundBalance = Balance{Absolute: cfg.OutboundBalance}
	cfg.inboundBalance = Balance{Absolute: cfg.InboundBalance}

	// Only consider relative values if absolute values are not set
	if cfg.InboundBalance == 0 && cfg.OutboundBalance == 0 {
		cfg.outboundBalance.Relative = utils.Percentage(cfg.OutboundBalancePercent)
		cfg.inboundBalance.Relative = utils.Percentage(cfg.InboundBalancePercent)
		if cfg.OutboundBalancePercent+cfg.InboundBalancePercent >= 100 {
			return errors.New("sum of balance percentages must be smaller than 100")
		}
	}

	if cfg.inboundBalance.IsZero() && cfg.outboundBalance.IsZero() {
		return errors.New("no balance threshold set")
	}

	if cfg.PerChannel {
		if cfg.inboundBalance.IsAbsolute() {
			return errors.New("absolute balance threshold not supported for per channel rebalancing")
		}
		if cfg.AllowNormalSwaps() {
			return errors.New("per channel rebalancing only supported for reverse swaps")
		}
		cfg.strategy = cfg.perChannelStrategy
		cfg.description = "Per channel"
	} else {
		cfg.strategy = cfg.totalBalanceStrategy
		cfg.description = "Total balance"
	}

	if cfg.outboundBalance.IsZero() {
		if cfg.AllowNormalSwaps() {
			return errors.New("outbound balance must be set for normal swaps")
		}
		cfg.description += fmt.Sprintf(" (inbound %s)", cfg.inboundBalance)
	} else if cfg.inboundBalance.IsZero() {
		if cfg.AllowReverseSwaps() {
			return errors.New("inbound balance must be set for reverse swaps")
		}
		cfg.description += fmt.Sprintf(" (outbound %s)", cfg.outboundBalance)
	} else {
		cfg.description += fmt.Sprintf(" (outbound %s, inbound %s)", cfg.outboundBalance, cfg.inboundBalance)
	}

	if cfg.Wallet != "" {
		cfg.description += fmt.Sprintf(" using wallet %s (%s)", cfg.Wallet, cfg.currency)
	}
	if cfg.StaticAddress != "" {
		cfg.description += fmt.Sprintf(" with static address %s (%s)", cfg.StaticAddress, cfg.currency)
	}

	if cfg.Enabled {
		return cfg.InitWallet()
	}

	return nil
}

func (cfg *LightningConfig) InitWallet() (err error) {
	if cfg.onchain == nil {
		return errors.New("can not initialize wallet without onchain")
	}
	var wallet onchain.Wallet
	if cfg.Wallet != "" {
		wallet, err = cfg.onchain.GetAnyWallet(onchain.WalletChecker{
			Name:          &cfg.Wallet,
			Currency:      cfg.currency,
			AllowReadonly: !cfg.AllowNormalSwaps(),
		})
		if err != nil {
			err = fmt.Errorf("could not find wallet: %s", err)
		} else {
			id := wallet.GetWalletInfo().Id
			cfg.walletId = &id
		}
	} else if cfg.AllowNormalSwaps() {
		err = errors.New("wallet name must be set for normal swaps")
	} else if cfg.StaticAddress != "" {
		if err = boltz.ValidateAddress(cfg.onchain.Network, cfg.StaticAddress, cfg.currency); err != nil {
			err = fmt.Errorf("invalid static address %s: %w", cfg.StaticAddress, err)
		}
	} else {
		err = errors.New("static address or wallet must be set")
	}
	return err
}

func (cfg *LightningConfig) Description() string {
	return cfg.description
}

func (cfg *LightningConfig) GetPair(swapType boltz.SwapType) *boltzrpc.Pair {
	currency := cfg.SerializedLnConfig.Currency
	result := &boltzrpc.Pair{}
	switch swapType {
	case boltz.NormalSwap:
		result.From = currency
		result.To = boltzrpc.Currency_BTC
	case boltz.ReverseSwap:
		result.From = boltzrpc.Currency_BTC
		result.To = currency
	}
	return result
}

func (cfg *LightningConfig) Allowed(swapType boltz.SwapType) bool {
	return cfg.swapType == swapType || cfg.swapType == ""
}

func (cfg *LightningConfig) AllowNormalSwaps() bool {
	return cfg.Allowed(boltz.NormalSwap)
}

func (cfg *LightningConfig) AllowReverseSwaps() bool {
	return cfg.Allowed(boltz.ReverseSwap)
}

func (cfg *LightningConfig) getDismissedChannels() (DismissedChannels, error) {
	reasons := make(DismissedChannels)

	swaps, err := cfg.database.QueryPendingSwaps()
	if err != nil {
		return nil, errors.New("Could not query pending swaps: " + err.Error())
	}

	reverseSwaps, err := cfg.database.QueryPendingReverseSwaps()
	if err != nil {
		return nil, errors.New("Could not query pending reverse swaps: " + err.Error())
	}

	for _, swap := range swaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}
	for _, swap := range reverseSwaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}

	since := time.Now().Add(time.Duration(-cfg.FailureBackoff) * time.Second)
	failedSwaps, err := cfg.database.QueryFailedSwaps(since)
	if err != nil {
		return nil, errors.New("Could not query failed swaps: " + err.Error())
	}

	failedReverseSwaps, err := cfg.database.QueryFailedReverseSwaps(since)
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

func (cfg *LightningConfig) validateRecommendations(
	recommendations []*lightningRecommendation,
	budget int64,
) ([]*LightningRecommendation, error) {
	dismissedChannels, err := cfg.getDismissedChannels()
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
		pairInfo, err := cfg.rpc.GetAutoSwapPairInfo(swapType, cfg.GetPair(recommendation.Type))
		if err != nil {
			logger.Warn("Could not get pair info: " + err.Error())
			continue
		}

		params := checkParams{
			MaxFeePercent:    cfg.maxFeePercent,
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

func (cfg *LightningConfig) GetSwapRecommendations() ([]*LightningRecommendation, error) {
	channels, err := cfg.rpc.GetLightningChannels()
	if err != nil {
		return nil, err
	}

	recommendations := cfg.strategy(channels)

	budget, err := cfg.GetCurrentBudget(true)
	if err != nil {
		return nil, errors.New("Could not get budget: " + err.Error())
	}

	logger.Debugf("Current autoswap budget: %+v", *budget)

	return cfg.validateRecommendations(recommendations, budget.Amount)
}

func (cfg *LightningConfig) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	return cfg.shared.GetCurrentBudget(
		createIfMissing,
		Lightning,
		cfg,
		database.DefaultEntityId,
	)
}

func (cfg *LightningConfig) execute(recommendation *LightningRecommendation) error {
	var chanIds []string
	if chanId := recommendation.Channel.GetId(); chanId != 0 {
		chanIds = append(chanIds, chanId.ToCln())
	}
	pair := cfg.GetPair(recommendation.Type)
	var err error
	if recommendation.Type == boltz.ReverseSwap {
		err = cfg.rpc.CreateAutoReverseSwap(&database.DefaultEntity, &boltzrpc.CreateReverseSwapRequest{
			Amount:         recommendation.Amount,
			Address:        cfg.StaticAddress,
			AcceptZeroConf: cfg.AcceptZeroConf,
			Pair:           pair,
			ChanIds:        chanIds,
			WalletId:       cfg.walletId,
		})
	} else if recommendation.Type == boltz.NormalSwap {
		err = cfg.rpc.CreateAutoSwap(&database.DefaultEntity, &boltzrpc.CreateSwapRequest{
			Amount: recommendation.Amount,
			Pair:   pair,
			//ChanIds:          chanIds,
			SendFromInternal: true,
			WalletId:         cfg.walletId,
		})
	}
	return err
}

func (cfg *LightningConfig) run(stop <-chan bool) {
	ticker := time.NewTicker(time.Duration(cfg.ChannelPollInterval) * time.Second)

	for {
		recommendations, err := cfg.GetSwapRecommendations()
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

				err := cfg.execute(recommendation)
				if err != nil {
					logger.Error("Could not act on swap recommendation : " + err.Error())
				}
			}
		}
		// wait for ticker after executing so that it runs immediately upon startup
		select {
		case <-ticker.C:
			continue
		case <-stop:
			return
		}
	}
}
