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

type SerializedChainConfig = autoswaprpc.ChainConfig

type ChainConfig struct {
	*SerializedChainConfig

	entity        *database.Entity
	maxFeePercent utils.Percentage
	fromWallet    onchain.Wallet
	toWallet      onchain.Wallet
	pair          boltz.Pair
	description   string
}

func withChainBase(config *SerializedChainConfig) *SerializedChainConfig {
	return merge(&SerializedChainConfig{MaxFeePercent: 1}, config)
}

func NewChainConfig(serialized *SerializedChainConfig) *ChainConfig {
	return &ChainConfig{SerializedChainConfig: withChainBase(serialized)}
}

func (cfg *ChainConfig) Request(amount uint64) *boltzrpc.CreateChainSwapRequest {
	fromWalletId := cfg.fromWallet.GetWalletInfo().Id
	request := &boltzrpc.CreateChainSwapRequest{
		Amount:       amount,
		Pair:         utils.SerializePair(cfg.pair),
		FromWalletId: &fromWalletId,
	}
	if cfg.ToAddress != "" {
		request.ToAddress = &cfg.ToAddress
	} else {
		toWalletId := cfg.toWallet.GetWalletInfo().Id
		request.ToWalletId = &toWalletId
	}
	return request
}

func (cfg *ChainConfig) Description() string {
	return cfg.description
}

func (cfg *ChainConfig) entityId() *database.Id {
	if cfg.entity != nil {
		return &cfg.entity.Id
	}
	return nil
}

func (cfg *ChainConfig) Init(database *database.Database, chain *onchain.Onchain) (err error) {
	cfg.maxFeePercent = utils.Percentage(cfg.MaxFeePercent)
	if cfg.FromThreshold == 0 {
		return errors.New("FromThreshold must be set")
	}

	if cfg.Entity != nil {
		cfg.entity, err = database.GetEntityByName(*cfg.Entity)
		if err != nil {
			return fmt.Errorf("entity %s does not exist: %w", *cfg.Entity, err)
		}
	}

	cfg.fromWallet, err = chain.GetAnyWallet(onchain.WalletChecker{
		Name:          cfg.FromWallet,
		AllowReadonly: false,
		EntityId:      cfg.entityId(),
	})
	if err != nil {
		return fmt.Errorf("could not get from wallet: %w", err)
	}

	fromInfo := cfg.fromWallet.GetWalletInfo()
	cfg.pair.From = fromInfo.Currency

	cfg.description = fmt.Sprintf("From wallet %s (%s) to ", fromInfo.Name, fromInfo.Currency)

	if cfg.ToAddress != "" {
		err := boltz.ValidateAddress(chain.Network, cfg.ToAddress, boltz.CurrencyBtc)
		if err != nil {
			err := boltz.ValidateAddress(chain.Network, cfg.ToAddress, boltz.CurrencyLiquid)
			if err != nil {
				return fmt.Errorf("configured ToAddress %s is not a valid BTC or Liquid address: %w", cfg.ToAddress, err)
			} else {
				cfg.pair.To = boltz.CurrencyLiquid
			}
		} else {
			cfg.pair.To = boltz.CurrencyBtc
		}
		cfg.description += fmt.Sprintf("static %s address %s", cfg.pair.To, cfg.ToAddress)
	} else if cfg.ToWallet != "" {
		cfg.toWallet, err = chain.GetAnyWallet(onchain.WalletChecker{
			Name:          cfg.ToWallet,
			AllowReadonly: true,
			EntityId:      cfg.entityId(),
		})
		if err != nil {
			return fmt.Errorf("could not get to wallet: %w", err)
		}
		toInfo := cfg.toWallet.GetWalletInfo()
		cfg.pair.To = toInfo.Currency
		cfg.description += fmt.Sprintf("wallet %s (%s)", toInfo.Name, toInfo.Currency)
	} else {
		return fmt.Errorf("one of ToAddress or ToWallet must be set")
	}

	if cfg.pair.From == cfg.pair.To {
		return fmt.Errorf("from and to currency must be different")
	}

	return nil
}
