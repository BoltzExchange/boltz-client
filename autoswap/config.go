package autoswap

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/BoltzExchange/boltz-client/boltz"
)

func withBase(config *SerializedConfig) *SerializedConfig {
	base := &SerializedConfig{
		FailureBackoff:      24 * 60 * 60,
		MaxFeePercent:       1,
		ChannelPollInterval: 30,
		Budget:              100000,
		BudgetInterval:      7 * 60 * 60 * 24,
	}
	proto.Merge(base, config)
	return base
}

func DefaultConfig() *SerializedConfig {
	// we cant include values like currency in the base config
	// since we couldnt know wether the user didnt set the currency at all or set it to BTC
	return withBase(&SerializedConfig{
		MaxBalancePercent: 75,
		MinBalancePercent: 25,
		Currency:          boltzrpc.Currency_LBTC,
	})
}

type Balance struct {
	Absolute uint64
	Relative utils.Percentage
}

func (b Balance) IsZero() bool {
	return b.Absolute == 0 && b.Relative == 0
}

func (b Balance) IsAbsolute() bool {
	return b.Absolute != 0
}

func (b Balance) Get(capacity uint64) uint64 {
	if b.IsAbsolute() {
		return b.Absolute
	}
	return b.Relative.Calculate(capacity)
}

func (b Balance) String() string {
	if b.IsAbsolute() {
		return utils.Satoshis(b.Absolute)
	}
	return b.Relative.String()
}

type SerializedConfig = autoswaprpc.Config

type Config struct {
	*SerializedConfig

	maxFeePercent utils.Percentage
	currency      boltz.Currency
	swapType      boltz.SwapType
	maxBalance    Balance
	minBalance    Balance
	strategy      Strategy
	strategyName  string
}

func NewConfig(serialized *SerializedConfig) *Config {
	return &Config{SerializedConfig: withBase(serialized)}
}

type DismissedChannels map[lightning.ChanId][]string
type ChannelLimits map[lightning.ChanId]uint64

func (dismissed DismissedChannels) addChannels(chanIds []lightning.ChanId, reason string) {
	if chanIds == nil {
		chanIds = []lightning.ChanId{0}
	}
	for _, chanId := range chanIds {
		if !slices.Contains(dismissed[chanId], reason) {
			dismissed[chanId] = append(dismissed[chanId], reason)
		}
	}
}

func (cfg *Config) Init() error {
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
		if cfg.swapType == "" || cfg.swapType == boltz.NormalSwap {
			return errors.New("per channel rebalancing only supported for reverse swaps")
		}
		cfg.strategy = cfg.perChannelStrategy
		cfg.strategyName = "per channel"
	} else {
		cfg.strategy = cfg.totalBalanceStrategy
		cfg.strategyName = "total balance"
	}

	if cfg.minBalance.IsZero() {
		if cfg.swapType != boltz.ReverseSwap {
			return errors.New("min balance must be set for normal swaps")
		}
		cfg.strategyName += fmt.Sprintf(" (max %s)", cfg.maxBalance)
	} else if cfg.maxBalance.IsZero() {
		if cfg.swapType != boltz.NormalSwap {
			return errors.New("max balance must be set for reverse swaps")
		}
		cfg.strategyName += fmt.Sprintf(" (min %s)", cfg.minBalance)
	} else {
		cfg.strategyName += fmt.Sprintf(" (min %s, max %s)", cfg.minBalance, cfg.maxBalance)
	}

	return nil
}

func (cfg *Config) GetAddress(network *boltz.Network) (string, error) {
	if cfg.StaticAddress == "" {
		return "", errors.New("No address for Currency " + string(cfg.currency))
	}
	err := boltz.ValidateAddress(network, cfg.StaticAddress, cfg.currency)
	if err != nil {
		return "", errors.New("Invalid address for Currency " + string(cfg.currency) + " :" + err.Error())
	}
	return cfg.StaticAddress, nil
}

func (cfg *Config) getField(name string) (protoreflect.FieldDescriptor, error) {
	reflect := cfg.SerializedConfig.ProtoReflect()
	fields := reflect.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if strings.EqualFold(name, string(field.JSONName())) {
			return field, nil
		}
	}
	return nil, errors.New("Unknown field")
}

func (cfg *Config) GetValue(name string) (any, error) {
	field, err := cfg.getField(name)
	if err != nil {
		return "", err
	}

	return cfg.SerializedConfig.ProtoReflect().Get(field).Interface(), nil
}

func (cfg *Config) SetValue(name string, value any) error {
	field, err := cfg.getField(name)
	if err != nil {
		return err
	}

	cloned := proto.Clone(cfg.SerializedConfig).(*SerializedConfig)
	stringValue := fmt.Sprint(value)
	var setValue any

	switch field.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		value, err := strconv.ParseInt(stringValue, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer: %w", err)
		}
		setValue = value
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		value, err := strconv.ParseUint(stringValue, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer: %w", err)
		}
		setValue = value
	case protoreflect.FloatKind:
		value, err := strconv.ParseFloat(stringValue, 64)
		if err != nil {
			return fmt.Errorf("invalid float: %w", err)
		}
		setValue = value
	case protoreflect.BoolKind:
		value, err := strconv.ParseBool(stringValue)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %w", err)
		}
		setValue = value
	case protoreflect.StringKind:
		setValue = value
	case protoreflect.EnumKind:
		values := field.Enum().Values()
		for i := 0; i < values.Len(); i++ {
			if strings.EqualFold(stringValue, string(values.Get(i).Name())) {
				setValue = values.Get(i).Number()
			}
		}
		if setValue == nil {
			return fmt.Errorf("invalid enum value: %s", stringValue)
		}

	default:
		return errors.New("Unknown field type")
	}
	cloned.ProtoReflect().Set(field, protoreflect.ValueOf(setValue))
	updated := NewConfig(cloned)
	// make sure the new values are still valid
	if err := updated.Init(); err != nil {
		return err
	}
	*cfg = *updated
	return nil
}

func (cfg *Config) StrategyName() string {
	return cfg.strategyName
}

func (cfg *Config) GetPair(swapType boltz.SwapType) *boltzrpc.Pair {
	currency := cfg.SerializedConfig.Currency
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
