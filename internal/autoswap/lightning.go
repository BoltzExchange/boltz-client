package autoswap

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"
	"google.golang.org/protobuf/proto"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
)

type LightningSwapper = swapper[*LightningConfig]

type SerializedLnConfig = autoswaprpc.LightningConfig

type LightningConfig struct {
	*SerializedLnConfig
	shared

	reserve         boltz.Percentage
	maxFeePercent   boltz.Percentage
	currency        boltz.Currency
	swapType        boltz.SwapType
	outboundBalance Balance
	inboundBalance  Balance
	strategy        Strategy
	description     string
	wallet          onchain.Wallet

	executeLock sync.Mutex
}

const DefaultReserve = boltz.Percentage(10)

func NewLightningConfig(serialized *SerializedLnConfig, shared shared) *LightningConfig {
	return &LightningConfig{SerializedLnConfig: withLightningBase(serialized), shared: shared, reserve: DefaultReserve}
}

func withLightningBase(config *SerializedLnConfig) *SerializedLnConfig {
	return merge(&SerializedLnConfig{
		FailureBackoff:      60 * 60,
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

	cfg.currency = serializers.ParseCurrency(&cfg.Currency)
	cfg.maxFeePercent = boltz.Percentage(cfg.MaxFeePercent)
	cfg.outboundBalance = Balance{Absolute: cfg.OutboundBalance}
	cfg.inboundBalance = Balance{Absolute: cfg.InboundBalance}

	// Only consider relative values if absolute values are not set
	if cfg.InboundBalance == 0 && cfg.OutboundBalance == 0 {
		cfg.outboundBalance.Relative = boltz.Percentage(cfg.OutboundBalancePercent)
		cfg.inboundBalance.Relative = boltz.Percentage(cfg.InboundBalancePercent)
		maxPercent := boltz.Percentage(100) - 2*cfg.reserve
		if cfg.outboundBalance.Relative+cfg.inboundBalance.Relative > maxPercent {
			if cfg.outboundBalance.IsZero() {
				return fmt.Errorf("inbound threshold muss be less than %s", maxPercent)
			} else if cfg.inboundBalance.IsZero() {
				return fmt.Errorf("outbound threshold muss be less than %s", maxPercent)
			} else {
				return fmt.Errorf("sum of thresholds must be less than %s", maxPercent)
			}
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

	// don't require a wallet if the config is disabled to allow for easier testing
	if err := cfg.InitWallet(); err != nil && cfg.Enabled {
		return err
	}

	return nil
}

func (cfg *LightningConfig) InitWallet() (err error) {
	if cfg.onchain == nil {
		return errors.New("can not initialize wallet without onchain")
	}
	if cfg.Wallet != "" {
		cfg.wallet, err = cfg.onchain.GetAnyWallet(onchain.WalletChecker{
			Name:          &cfg.Wallet,
			Currency:      cfg.currency,
			AllowReadonly: !cfg.AllowNormalSwaps(),
		})
		if err != nil {
			err = fmt.Errorf("could not find wallet: %s", err)
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

func (cfg *LightningConfig) GetPair(swapType boltzrpc.SwapType) *boltzrpc.Pair {
	currency := cfg.Currency
	result := &boltzrpc.Pair{}
	switch swapType {
	case boltzrpc.SwapType_SUBMARINE:
		result.From = currency
		result.To = boltzrpc.Currency_BTC
	case boltzrpc.SwapType_REVERSE:
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

	query := database.PendingSwapQuery
	query.Include = boltzrpc.IncludeSwaps_AUTO
	swaps, err := cfg.database.QuerySwaps(query)
	if err != nil {
		return nil, errors.New("Could not query pending swaps: " + err.Error())
	}

	reverseSwaps, err := cfg.database.QueryReverseSwaps(query)
	if err != nil {
		return nil, errors.New("Could not query pending reverse swaps: " + err.Error())
	}

	for _, swap := range swaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}
	for _, swap := range reverseSwaps {
		reasons.addChannels(swap.ChanIds, ReasonPendingSwap)
	}

	query = database.FailedSwapQuery
	query.Since = time.Now().Add(time.Duration(-cfg.FailureBackoff) * time.Second)
	query.Include = boltzrpc.IncludeSwaps_AUTO
	failedSwaps, err := cfg.database.QuerySwaps(query)
	if err != nil {
		return nil, errors.New("Could not query failed swaps: " + err.Error())
	}

	failedReverseSwaps, err := cfg.database.QueryReverseSwaps(query)
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

type LightningSwap struct {
	checks
	Type boltz.SwapType
}

func (lightningSwap *LightningSwap) GetAmount() uint64 {
	if lightningSwap == nil {
		return 0
	}
	return lightningSwap.Amount
}

type LightningRecommendation struct {
	Swap       *LightningSwap
	Channel    *lightning.LightningChannel
	Thresholds *autoswaprpc.LightningThresholds
}

func (cfg *LightningConfig) validateRecommendations(
	recommendations []*LightningRecommendation,
	budget uint64,
	includeAll bool,
) ([]*LightningRecommendation, error) {
	dismissedChannels, err := cfg.getDismissedChannels()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Dismissed channels: %v", dismissedChannels)

	var balance *onchain.Balance
	if cfg.wallet != nil {
		balance, err = cfg.wallet.GetBalance()
		if err != nil {
			return nil, errors.New("Could not get wallet balance: " + err.Error())
		}
		logger.Debugf("Wallet balance: %+v", balance)
	}

	// we might be able to fit more swaps in the budget if we sort by amount
	slices.SortFunc(recommendations, func(a, b *LightningRecommendation) int {
		return int(a.Swap.GetAmount() - b.Swap.GetAmount())
	})

	var checked []*LightningRecommendation
	for _, recommendation := range recommendations {
		swap := recommendation.Swap
		if swap != nil {
			swapType := serializers.SerializeSwapType(swap.Type)
			pairInfo, err := cfg.rpc.GetAutoSwapPairInfo(swapType, cfg.GetPair(swapType))
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
			swap.checks = check(swap.GetAmount(), params)
			if balance != nil && swap.Type == boltz.NormalSwap && swap.Amount > balance.Confirmed {
				swap.Dismiss(ReasonInsufficientFunds)
			}
		}

		if includeAll || swap != nil {
			checked = append(checked, recommendation)
		}
	}

	return checked, nil
}

func (cfg *LightningConfig) GetSwapRecommendations(includeAll bool) ([]*autoswaprpc.LightningRecommendation, error) {
	cfg.executeLock.Lock()
	defer cfg.executeLock.Unlock()
	return cfg.getSwapRecommendations(includeAll)
}

func (cfg *LightningConfig) getSwapRecommendations(includeAll bool) ([]*autoswaprpc.LightningRecommendation, error) {
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

	validated, err := cfg.validateRecommendations(recommendations, budget.Amount, includeAll)
	if err != nil {
		return nil, fmt.Errorf("could not validate recommendations: %w", err)
	}

	var result []*autoswaprpc.LightningRecommendation
	for _, recommendation := range validated {
		result = append(result, &autoswaprpc.LightningRecommendation{
			Swap:       serializeLightningSwap(recommendation.Swap),
			Channel:    serializeLightningChannel(recommendation.Channel),
			Thresholds: recommendation.Thresholds,
		})
	}
	return result, nil
}

func (cfg *LightningConfig) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	return cfg.shared.GetCurrentBudget(
		createIfMissing,
		Lightning,
		cfg,
		database.DefaultTenantId,
	)
}

func (cfg *LightningConfig) walletId() *database.Id {
	if cfg.wallet != nil {
		id := cfg.wallet.GetWalletInfo().Id
		return &id
	}
	return nil
}

func (cfg *LightningConfig) CheckAndExecute(accepted []*autoswaprpc.LightningRecommendation, force bool) error {
	cfg.executeLock.Lock()
	defer cfg.executeLock.Unlock()
	logger.Debugf("Checking for lightning swap recommendation")
	recommendations, err := cfg.getSwapRecommendations(false)
	if err != nil {
		return fmt.Errorf("could not fetch swap recommendations: %w", err)
	}
	for _, recommendation := range recommendations {
		if err := checkAccepted(recommendation, accepted); err != nil {
			if errors.Is(err, errNotInAccepted) {
				continue
			}
			return err
		}
		if err := cfg.execute(recommendation, force); err != nil {
			return fmt.Errorf("could not execute recommendation: %w", err)
		}
	}
	return nil
}

func (cfg *LightningConfig) execute(recommendation *autoswaprpc.LightningRecommendation, force bool) error {
	if !force && len(recommendation.Swap.DismissedReasons) > 0 {
		logger.Infof("Skipping swap recommendation %+v", recommendation.Swap)
		return nil
	}

	logger.Infof("Executing Swap recommendation: %+v", recommendation.Swap)
	var chanIds []string
	if recommendation.Channel.Id != nil {
		chanIds = append(chanIds, recommendation.Channel.Id.Cln)
	}
	swap := recommendation.Swap
	pair := cfg.GetPair(swap.Type)
	var err error
	switch swap.Type {
	case boltzrpc.SwapType_REVERSE:
		err = cfg.rpc.CreateAutoReverseSwap(&database.DefaultTenant, &boltzrpc.CreateReverseSwapRequest{
			Amount:         swap.Amount,
			Address:        cfg.StaticAddress,
			AcceptZeroConf: cfg.AcceptZeroConf,
			Pair:           pair,
			ChanIds:        chanIds,
			WalletId:       cfg.walletId(),
		})
	case boltzrpc.SwapType_SUBMARINE:
		err = cfg.rpc.CreateAutoSwap(&database.DefaultTenant, &boltzrpc.CreateSwapRequest{
			Amount: swap.Amount,
			Pair:   pair,
			//ChanIds:          chanIds,
			SendFromInternal: true,
			WalletId:         cfg.walletId(),
		})
	}
	return err
}

func (cfg *LightningConfig) run(stop <-chan bool) {
	ticker := time.NewTicker(time.Duration(cfg.ChannelPollInterval) * time.Second)

	for {
		if err := cfg.CheckAndExecute(nil, false); err != nil {
			logger.Errorf("Lightning autoswap: %s", err)
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

var errNotInAccepted = errors.New("not in accepted")

func checkAccepted(recommendation *autoswaprpc.LightningRecommendation, accepted []*autoswaprpc.LightningRecommendation) error {
	if len(accepted) > 0 {
		for _, check := range accepted {
			if check.GetSwap() != nil && proto.Equal(recommendation.Channel.GetId(), check.Channel.GetId()) {
				return checkAcceptedReasons(check.Swap.DismissedReasons, recommendation.Swap.DismissedReasons)
			}
		}
		return errNotInAccepted
	}
	return nil
}
