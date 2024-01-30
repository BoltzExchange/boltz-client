package autoswap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/BurntSushi/toml"

	"github.com/BoltzExchange/boltz-client/boltz"
)

var DefaultConfig Config = Config{
	MaxBalancePercent:   75,
	MinBalancePercent:   25,
	FailureBackoff:      24 * 60 * 60,
	Enabled:             false,
	MaxFeePercent:       1,
	ChannelPollInterval: 30,
	Type:                "",
	Currency:            boltz.CurrencyLiquid,
	Budget:              100000,
	BudgetInterval:      7 * 60 * 60 * 24,
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
	return uint64(b.Relative.Calculate(float64(capacity)))
}

func (b Balance) String() string {
	if b.IsAbsolute() {
		return utils.Satoshis(b.Absolute)
	}
	return b.Relative.String()
}

type Config struct {
	Enabled             bool
	ChannelPollInterval uint64
	LiquidAddress       string
	BitcoinAddress      string
	MaxBalance          uint64
	MinBalance          uint64
	MaxBalancePercent   utils.Percentage
	MinBalancePercent   utils.Percentage
	MaxFeePercent       utils.Percentage
	AcceptZeroConf      bool
	FailureBackoff      uint64
	Budget              uint64
	BudgetInterval      uint64
	Currency            boltz.Currency
	Type                boltz.SwapType
	PerChannel          bool
	Wallet              string
	MaxSwapAmount       uint64

	maxBalance   Balance
	minBalance   Balance
	strategy     Strategy
	strategyName string
}

type DismissedChannels map[lightning.ChanId][]string
type ChannelLimits map[lightning.ChanId]uint64

func (dismissed DismissedChannels) addChannels(chanIds []lightning.ChanId, reason string) {
	for _, chanId := range chanIds {
		if !slices.Contains(dismissed[chanId], reason) {
			dismissed[chanId] = append(dismissed[chanId], reason)
		}
	}
}

func (cfg *Config) Init() error {
	cfg.maxBalance = Balance{Absolute: cfg.MaxBalance}
	cfg.minBalance = Balance{Absolute: cfg.MinBalance}

	// Only consider relative values if absolute values are not set
	if cfg.MaxBalance == 0 && cfg.MinBalance == 0 {
		cfg.maxBalance.Relative = cfg.MaxBalancePercent
		cfg.minBalance.Relative = cfg.MinBalancePercent
	}

	if cfg.minBalance.IsZero() && cfg.maxBalance.IsZero() {
		return errors.New("no balance threshold set")
	}

	if !cfg.maxBalance.IsZero() && !cfg.minBalance.IsZero() {
		if cfg.minBalance.Get(1) > cfg.maxBalance.Get(1) {
			return errors.New("min balance must be smaller than max balance")
		}
	}

	if cfg.Budget == 0 {
		cfg.Budget = DefaultConfig.Budget
	}

	if cfg.BudgetInterval == 0 {
		cfg.BudgetInterval = DefaultConfig.BudgetInterval
	}

	if cfg.PerChannel {
		if cfg.minBalance.IsAbsolute() {
			return errors.New("absolute balance threshold not supported for per channel rebalancing")
		}
		cfg.strategy = cfg.perChannelStrategy
		cfg.strategyName = "per channel"
	} else {
		cfg.strategy = cfg.totalBalanceStrategy
		cfg.strategyName = "total balance"
	}

	if cfg.minBalance.IsZero() {
		if cfg.Type != boltz.ReverseSwap {
			return errors.New("min balance must be set for normal swaps")
		}
		cfg.strategyName += fmt.Sprintf(" (max %s)", cfg.maxBalance)
	} else if cfg.maxBalance.IsZero() {
		if cfg.Type != boltz.NormalSwap {
			return errors.New("max balance must be set for reverse swaps")
		}
		cfg.strategyName += fmt.Sprintf(" (min %s)", cfg.minBalance)
	} else {
		cfg.strategyName += fmt.Sprintf(" (min %s, max %s)", cfg.minBalance, cfg.maxBalance)
	}

	switch strings.ToUpper(string(cfg.Currency)) {
	case string(boltz.CurrencyBtc):
		cfg.Currency = boltz.CurrencyBtc
	case string(boltz.CurrencyLiquid), "":
		cfg.Currency = boltz.CurrencyLiquid
	default:
		return errors.New("invalid currency")
	}

	return nil
}

func (cfg *Config) GetAddress(network *boltz.Network) (address string, err error) {
	if cfg.Currency == boltz.CurrencyLiquid && cfg.LiquidAddress != "" {
		address = cfg.LiquidAddress
	} else if cfg.Currency == boltz.CurrencyBtc && cfg.BitcoinAddress != "" {
		address = cfg.BitcoinAddress
	}
	if address == "" {
		return "", errors.New("No address for Currency " + string(cfg.Currency))
	}
	err = boltz.ValidateAddress(network, address, cfg.Currency)
	if err != nil {
		return "", errors.New("Invalid address for Currency " + string(cfg.Currency) + " :" + err.Error())
	}
	return address, nil
}

func (cfg *Config) getField(field string) *reflect.Value {
	ps := reflect.ValueOf(cfg)
	// struct
	s := ps.Elem()
	for i := 0; i < s.NumField(); i++ {
		structField := s.Type().Field(i)
		f := s.Field(i)
		if f.IsValid() && structField.IsExported() && strings.EqualFold(field, structField.Name) {
			return &f
		}
	}
	return nil
}

func (cfg *Config) GetValue(field string) (any, error) {
	f := cfg.getField(field)
	if f == nil {
		return "", errors.New("Unknown field")
	}
	return f.Interface(), nil
}

func (cfg *Config) SetValue(field string, value any) error {
	f := cfg.getField(field)
	if f == nil {
		return errors.New("Unknown field")
	}
	// A Value can be changed only if it is
	// addressable and was not obtained by
	// the use of unexported struct fields.
	stringValue := fmt.Sprint(value)
	if f.CanSet() {
		if reflect.ValueOf(value).IsZero() {
			f.SetZero()
			return nil
		}
		input := reflect.ValueOf(value)
		if f.Type() == input.Type() {
			f.Set(input)
			return nil
		}
		// change value of N
		switch f.Kind() {
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			value, err := strconv.ParseInt(stringValue, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			if !f.OverflowInt(value) {
				f.SetInt(value)
			} else {
				return fmt.Errorf("too large integer: %w", err)
			}
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
			value, err := strconv.ParseUint(stringValue, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			if !f.OverflowUint(value) {
				f.SetUint(value)
			} else {
				return fmt.Errorf("too large integer: %w", err)
			}
		case reflect.Float32, reflect.Float64:
			value, err := strconv.ParseFloat(stringValue, 64)
			if err != nil {
				return fmt.Errorf("invalid float: %w", err)
			}
			if !f.OverflowFloat(value) {
				f.SetFloat(value)
			} else {
				return fmt.Errorf("too large float: %w", err)
			}
		case reflect.Bool:
			value, err := strconv.ParseBool(stringValue)
			if err != nil {
				return fmt.Errorf("invalid boolean value: %w", err)
			}
			f.SetBool(value)
		case reflect.String:
			switch f.Interface().(type) {
			case boltz.Currency:
				currency, err := boltz.ParseCurrency(stringValue)
				if err != nil {
					return fmt.Errorf("invalid currency value: %w", err)
				}
				f.Set(reflect.ValueOf(currency))
			case boltz.SwapType:
				swapType, err := boltz.ParseSwapType(stringValue)
				if err != nil {
					return fmt.Errorf("invalid swap type value: %w", err)
				}
				f.Set(reflect.ValueOf(swapType))
			default:
				f.SetString(stringValue)
			}
		default:
			return errors.New("Unknown field type")
		}
	}
	return nil
}

func (cfg *Config) Write(path string) error {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0666)
}

func (cfg *Config) StrategyName() string {
	return cfg.strategyName
}
