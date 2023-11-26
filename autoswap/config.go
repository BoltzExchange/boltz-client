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
	ChannelImbalanceThreshold: 25,
	FailureBackoff:            24 * 60 * 60,
	Enabled:                   false,
	LocalBalanceReserve:       25,
	MaxFeePercent:             1,
	ChannelPollInterval:       30,
	Type:                      "",
	Pair:                      boltz.PairBtc,
	AutoBudget:                100000,
	AutoBudgetPeriod:          7 * 60 * 60 * 24,
}

type Config struct {
	ChannelImbalanceThreshold utils.Percentage `long:"swap.channel-imbalance-threshold" description:"Threshold to determine wheter or not a swap should be initiated"`
	Enabled                   bool             `long:"swap.enabled" description:"Automatically initiate swaps when a channel is inbalanced"`
	ChannelPollInterval       uint64           `long:"swap.channel-poll-interval" description:"Interval at which to poll for channel recommendations"`
	LiquidAddress             string           `long:"swap.liquid-address" description:"Address of an external liquid wallet to use for swaps"`
	BitcoinAddress            string           `long:"swap.bitcoin-address" description:"Address of an external bitcoin wallet to use for swaps"`
	LocalBalanceThreshold     uint64           `long:"swap.local-balance-threshold" description:"Threshold for combined local balance"`
	LocalBalanceReserve       utils.Percentage `long:"swap.local-balance-reserve" description:"Minimum Percentage of capacity to reserve for local balance when performing out-going swaps"`
	MaxFeePercent             utils.Percentage `long:"swap.max-fee-percent" description:"Maximum percentage of the swap amount to be paid as fees"`
	AcceptZeroConf            bool             `long:"swap.accept-zero-conf" description:"Whether to accept zero conf on auto swaps"`
	FailureBackoff            uint64           `long:"swap.failure-backoff" description:"Time in seconds to wait before retrying a swap through a channel that failed"`
	AutoBudget                uint64           `long:"swap.auto-budget" description:"Maximum amount of sats the auto swapper can spend on fees"`
	AutoBudgetPeriod          uint64           `long:"swap.auto-budget-period" description:"Period in seconds for the auto budget"`
	Pair                      boltz.Pair       `long:"swap.pair" description:"Pair to use for swaps"`
	Type                      boltz.SwapType   `long:"swap.type" description:"Type of swaps to perform"`
	PerChannel                bool             `long:"swap.per-channel" description:"Whether to check for swaps on a per-channel basis"`

	strategy     Strategy
	strategyName string
}

type DismissedChannels map[lightning.ChanId][]string

func (dismissed DismissedChannels) addChannel(id lightning.ChanId, reason string) {
	if !slices.Contains(dismissed[id], reason) {
		dismissed[id] = append(dismissed[id], reason)
	}
}

func (cfg *Config) Init() error {
	if cfg.PerChannel {
		if cfg.ChannelImbalanceThreshold == 0 {
			return errors.New("individual channel rebalancing strategy requires a channel imbalance threshold")
		}
		cfg.strategy = cfg.perChannelStrategy
		cfg.strategyName = "per channel"
	} else {
		if cfg.LocalBalanceThreshold == 0 && cfg.ChannelImbalanceThreshold == 0 {
			return errors.New("local balance strategy requires either a local balance or channel imbalance threshold")
		}
		cfg.strategy = cfg.totalBalanceStrategy
		cfg.strategyName = "total balance"
		if cfg.LocalBalanceThreshold > 0 {
			cfg.Type = boltz.ReverseSwap
			cfg.strategyName += fmt.Sprintf(" (absolute threshold %s)", utils.Satoshis(cfg.LocalBalanceThreshold))
		} else {
			cfg.strategyName += fmt.Sprintf(" (relative threshold %v)", cfg.ChannelImbalanceThreshold)
		}
	}

	return nil
}

func (cfg *Config) GetAddress(network *boltz.Network) (address string, err error) {
	if cfg.Pair == boltz.PairLiquid && cfg.LiquidAddress != "" {
		address = cfg.LiquidAddress
	} else if cfg.Pair == boltz.PairBtc && cfg.BitcoinAddress != "" {
		address = cfg.BitcoinAddress
	}
	if address == "" {
		return "", errors.New("No address for pair " + string(cfg.Pair))
	}
	err = boltz.ValidateAddress(network, address, cfg.Pair)
	if err != nil {
		return "", errors.New("Invalid address for pair " + string(cfg.Pair) + " :" + err.Error())
	}
	return address, nil
}

func (cfg *Config) getField(field string) *reflect.Value {
	ps := reflect.ValueOf(cfg)
	// struct
	s := ps.Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		name := s.Type().Field(i).Name
		if f.IsValid() && strings.EqualFold(field, name) {
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

func (cfg *Config) SetValue(field string, value string) error {
	newCfg := *cfg
	f := newCfg.getField(field)
	if f == nil {
		return errors.New("Unknown field")
	}
	// A Value can be changed only if it is
	// addressable and was not obtained by
	// the use of unexported struct fields.
	if f.CanSet() {
		// change value of N
		switch f.Kind() {
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			value, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			if !f.OverflowInt(value) {
				f.SetInt(value)
			} else {
				return fmt.Errorf("too large integer: %w", err)
			}
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
			value, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			if !f.OverflowUint(value) {
				f.SetUint(value)
			} else {
				return fmt.Errorf("too large integer: %w", err)
			}
		case reflect.Float32, reflect.Float64:
			value, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid float: %w", err)
			}
			if !f.OverflowFloat(value) {
				f.SetFloat(value)
			} else {
				return fmt.Errorf("too large float: %w", err)
			}
		case reflect.Bool:
			value, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid boolean value: %w", err)
			}
			f.SetBool(value)
		case reflect.String:
			f.SetString(value)
		default:
			return errors.New("Unknown field type")
		}
	}
	*cfg = newCfg
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
