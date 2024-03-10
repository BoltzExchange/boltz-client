package autoswap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/BurntSushi/toml"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
)

var ErrorNotConfigured = errors.New("autoswap not configured")

type Limits struct {
	MinAmount uint64
	MaxAmount uint64
}

type PairInfo struct {
	Limits
	PercentageFee utils.Percentage
	OnchainFee    uint64
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
	GetPairInfo        func(pair *boltzrpc.Pair, swapType boltz.SwapType) (*PairInfo, error)
}

func (swapper *AutoSwapper) Init(database *database.Database, onchain *onchain.Onchain, configPath string) {
	swapper.onchain = onchain
	swapper.database = database
	swapper.configPath = configPath
}

func (swapper *AutoSwapper) SetConfigValue(name string, value any) error {
	if err := swapper.requireConfig(); err != nil {
		return err
	}
	if err := swapper.cfg.SetValue(name, value); err != nil {
		return err
	}
	return swapper.setConfig(swapper.cfg)
}

func (swapper *AutoSwapper) setConfig(cfg *Config) error {
	logger.Debugf("Setting auto swap config: %+v", cfg)
	message := fmt.Sprintf("Using %v strategy to recommend swaps", cfg.strategyName)
	if cfg.swapType != "" {
		message += " of type " + string(cfg.swapType)
	}
	message += " for currency " + string(cfg.currency)

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

func (swapper *AutoSwapper) SetConfig(values *SerializedConfig) error {
	cfg := NewConfig(values)
	if err := cfg.Init(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return swapper.setConfig(cfg)
}

func (swapper *AutoSwapper) LoadConfig() error {
	var err error

	if !utils.FileExists(swapper.configPath) {
		return nil
	}
	serialized := &SerializedConfig{}
	var cfgToml any
	if _, err = toml.DecodeFile(swapper.configPath, &cfgToml); err != nil {
		err = fmt.Errorf("Could not decode autoswap config: " + err.Error())
	}
	if err == nil {
		// cant go from toml to proto directly, so we need to marshal again
		cfgJson, _ := json.Marshal(cfgToml)
		if err = protojson.Unmarshal(cfgJson, serialized); err != nil {
			err = fmt.Errorf("Could not decode autoswap config: " + err.Error())
		}
	}

	if err == nil {
		err = swapper.SetConfig(serialized)
	}
	// only set error if we dont have a config yet
	if err != nil && swapper.cfg == nil {
		swapper.err = err
	}
	return err
}

func (swapper *AutoSwapper) saveConfig() error {
	buf := new(bytes.Buffer)
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated:   true,
		EmitDefaultValues: true,
	}
	marshalled, _ := marshaler.Marshal(swapper.cfg.SerializedConfig)
	var asJson any
	// cant go from json to toml directly, so we need to unmarshal again
	_ = json.Unmarshal(marshalled, &asJson)
	if err := toml.NewEncoder(buf).Encode(asJson); err != nil {
		return err
	}
	return os.WriteFile(swapper.configPath, buf.Bytes(), 0666)
}

func (swapper *AutoSwapper) requireConfig() error {
	if swapper.cfg == nil {
		if swapper.err != nil {
			return fmt.Errorf("%w: %w", ErrorNotConfigured, swapper.err)
		}
		return ErrorNotConfigured
	}
	return nil
}

func (swapper *AutoSwapper) GetConfig() (*Config, error) {
	if err := swapper.requireConfig(); err != nil {
		return nil, err
	}
	cfg := *swapper.cfg
	return &cfg, nil
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

func (swapper *AutoSwapper) validateRecommendations(
	recommendations []*rawRecommendation,
	budget int64,
) ([]*SwapRecommendation, error) {
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
		pairInfo, err := swapper.GetPairInfo(swapper.cfg.GetPair(recommendation.Type), recommendation.Type)
		if err != nil {
			logger.Warn("Could not get pair info: " + err.Error())
			continue
		}

		recommendation := recommendation.Check(pairInfo, swapper.cfg)
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
	if err := swapper.requireConfig(); err != nil {
		return nil, err
	}
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

func (swapper *AutoSwapper) execute(recommendation *SwapRecommendation, address string) error {
	var chanIds []string
	if chanId := recommendation.Channel.GetId(); chanId != 0 {
		chanIds = append(chanIds, chanId.ToCln())
	}
	pair := swapper.cfg.GetPair(recommendation.Type)
	var err error
	if recommendation.Type == boltz.ReverseSwap {
		err = swapper.ExecuteReverseSwap(&boltzrpc.CreateReverseSwapRequest{
			Amount:         int64(recommendation.Amount),
			Address:        address,
			AcceptZeroConf: bool(swapper.cfg.AcceptZeroConf),
			Pair:           pair,
			ChanIds:        chanIds,
			Wallet:         &swapper.cfg.Wallet,
		})
	} else if recommendation.Type == boltz.NormalSwap {
		err = swapper.ExecuteSwap(&boltzrpc.CreateSwapRequest{
			Amount:   int64(recommendation.Amount),
			Pair:     pair,
			ChanIds:  chanIds,
			AutoSend: true,
			Wallet:   &swapper.cfg.Wallet,
		})
	}
	return err
}

func (swapper *AutoSwapper) Enabled() bool {
	return swapper.cfg != nil && swapper.cfg.Enabled
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
	if err := swapper.requireConfig(); err != nil {
		return err
	}
	swapper.Stop()

	logger.Info("Starting auto swapper")

	cfg := swapper.cfg
	address, err := cfg.GetAddress(swapper.onchain.Network)
	if err != nil {
		logger.Info(err.Error())
	}
	normalSwaps := cfg.swapType == "" || cfg.swapType == boltz.NormalSwap
	wallet, err := swapper.onchain.GetWallet(cfg.Wallet, cfg.currency, !normalSwaps)
	if wallet != nil {
		if normalSwaps || address == "" {
			address, err = wallet.NewAddress()
			if err != nil {
				err = errors.New("could not get external address: " + err.Error())
			}
			logger.Debugf("Got new address %v from wallet %v", address, wallet.Name())
		}
	} else if address == "" {
		err = fmt.Errorf("neither external address or wallet is available for currency %s: %v", cfg.currency, err)
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
