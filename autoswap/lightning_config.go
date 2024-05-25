package autoswap

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
)

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
		MaxBalancePercent: 75,
		MinBalancePercent: 25,
		Currency:          boltzrpc.Currency_LBTC,
	})
}

type SerializedLnConfig = autoswaprpc.LightningConfig

type LightningConfig struct {
	*SerializedLnConfig

	walletId *database.Id

	maxFeePercent  utils.Percentage
	currency       boltz.Currency
	swapType       boltz.SwapType
	maxBalance     Balance
	minBalance     Balance
	strategy       Strategy
	description    string
}

func NewConfig(serialized *SerializedLnConfig) *LightningConfig {
	return &LightningConfig{SerializedLnConfig: withLightningBase(serialized)}
}

func (cfg *LightningConfig) Init(chain *onchain.Onchain) error {
	var err error
	cfg.swapType, err = boltz.ParseSwapType(cfg.SwapType)
	if err != nil {
		return fmt.Errorf("invalid swap type: %w", err)
	}

	cfg.currency = utils.ParseCurrency(&cfg.Currency)
	cfg.maxFeePercent = utils.Percentage(cfg.MaxFeePercent)
	cfg.maxBalance = Balance{Absolute: cfg.MaxBalance}
	cfg.minBalance = Balance{Absolute: cfg.MinBalance}

	// Only consider relative values if absolute values are not set
	if cfg.MaxBalance == 0 && cfg.MinBalance == 0 {
		cfg.maxBalance.Relative = utils.Percentage(cfg.MaxBalancePercent)
		cfg.minBalance.Relative = utils.Percentage(cfg.MinBalancePercent)
	}

	if cfg.minBalance.IsZero() && cfg.maxBalance.IsZero() {
		return errors.New("no balance threshold set")
	}

	if !cfg.maxBalance.IsZero() && !cfg.minBalance.IsZero() {
		if cfg.minBalance.Get(100) > cfg.maxBalance.Get(100) {
			return errors.New("min balance must be smaller than max balance")
		}
	}

	if cfg.PerChannel {
		if cfg.minBalance.IsAbsolute() {
			return errors.New("absolute balance threshold not supported for per channel rebalancing")
		}
		if cfg.AllowNormalSwaps() {
			return errors.New("per channel rebalancing only supported for reverse swaps")
		}
		cfg.strategy = cfg.perChannelStrategy
		cfg.description = "per channel"
	} else {
		cfg.strategy = cfg.totalBalanceStrategy
		cfg.description = "total balance"
	}

	if cfg.minBalance.IsZero() {
		if cfg.AllowNormalSwaps() {
			return errors.New("min balance must be set for normal swaps")
		}
		cfg.description += fmt.Sprintf(" (max %s)", cfg.maxBalance)
	} else if cfg.maxBalance.IsZero() {
		if cfg.AllowReverseSwaps() {
			return errors.New("max balance must be set for reverse swaps")
		}
		cfg.description += fmt.Sprintf(" (min %s)", cfg.minBalance)
	} else {
		cfg.description += fmt.Sprintf(" (min %s, max %s)", cfg.minBalance, cfg.maxBalance)
	}

	if cfg.Enabled {
		if chain == nil {
			return errors.New("can not initialize wallet without onchain")
		}
		return cfg.InitWallet(chain)
	}

	return nil
}

func (cfg *LightningConfig) InitWallet(chain *onchain.Onchain) (err error) {
	var wallet onchain.Wallet
	if cfg.Wallet != "" {
		wallet, err = chain.GetAnyWallet(onchain.WalletChecker{
			Name:          &cfg.Wallet,
			Currency:      cfg.currency,
			AllowReadonly: !cfg.AllowNormalSwaps(),
		})
		if err != nil {
			err = fmt.Errorf("could not find from wallet: %s", err)
		} else {
			id := wallet.GetWalletInfo().Id
			cfg.walletId = &id
		}
	} else if cfg.AllowNormalSwaps() {
		err = errors.New("wallet name must be set for normal swaps")
	} else if cfg.StaticAddress != "" {
		if err = boltz.ValidateAddress(chain.Network, cfg.StaticAddress, cfg.currency); err != nil {
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
