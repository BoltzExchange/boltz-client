package autoswap

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
)

type ChainRecommendation = checks
type ChainSwapper = swapper[*ChainConfig]
type SerializedChainConfig = autoswaprpc.ChainConfig

type ChainConfig struct {
	*SerializedChainConfig
	shared

	tenant        *database.Tenant
	maxFeePercent utils.Percentage
	fromWallet    onchain.Wallet
	toWallet      onchain.Wallet
	pair          boltz.Pair
	description   string
}

func (cfg *ChainConfig) GetTenantId() database.Id {
	return cfg.tenant.Id
}

func withChainBase(config *SerializedChainConfig) *SerializedChainConfig {
	return merge(&SerializedChainConfig{
		MaxFeePercent:  1,
		Budget:         100000,
		BudgetInterval: 7 * 60 * 60 * 24,
	}, config)
}

func NewChainConfig(serialized *SerializedChainConfig, shared shared) *ChainConfig {
	return &ChainConfig{SerializedChainConfig: withChainBase(serialized), shared: shared}
}

func (cfg *ChainConfig) Description() string {
	return cfg.description
}

func (cfg *ChainConfig) Init() (err error) {
	if cfg.Tenant == nil {
		cfg.tenant = &database.DefaultTenant
	} else {
		cfg.tenant, err = cfg.database.GetTenantByName(cfg.GetTenant())
		if err != nil {
			return fmt.Errorf("could not get tenant: %w", err)
		}
	}
	cfg.maxFeePercent = utils.Percentage(cfg.MaxFeePercent)
	if cfg.MaxBalance == 0 {
		return errors.New("MaxBalance must be set")
	}

	cfg.fromWallet, err = cfg.onchain.GetAnyWallet(onchain.WalletChecker{
		Name:          &cfg.FromWallet,
		AllowReadonly: false,
		TenantId:      &cfg.tenant.Id,
	})
	if err != nil {
		return fmt.Errorf("could not get from wallet: %w", err)
	}

	fromInfo := cfg.fromWallet.GetWalletInfo()
	cfg.pair.From = fromInfo.Currency

	cfg.description = fmt.Sprintf("From wallet %s (%s, max balance %d sats) to ", fromInfo.Name, fromInfo.Currency, cfg.MaxBalance)

	if cfg.ToAddress != "" {
		cfg.pair.To, err = boltz.GetAddressCurrency(cfg.onchain.Network, cfg.ToAddress)
		if err != nil {
			return fmt.Errorf("configured ToAddress %s is not a valid BTC or Liquid address: %w", cfg.ToAddress, err)
		}
		cfg.description += fmt.Sprintf("static %s address %s", cfg.pair.To, cfg.ToAddress)
	} else if cfg.ToWallet != "" {
		cfg.toWallet, err = cfg.onchain.GetAnyWallet(onchain.WalletChecker{
			Name:          &cfg.ToWallet,
			AllowReadonly: true,
			TenantId:      &cfg.tenant.Id,
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

func (cfg *ChainConfig) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	return cfg.shared.GetCurrentBudget(createIfMissing, Chain, cfg, cfg.tenant.Id)
}

func (cfg *ChainConfig) GetRecommendation() (*ChainRecommendation, error) {
	balance, err := cfg.fromWallet.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("could not get wallet balance: %w", err)
	}

	pairInfo, err := cfg.rpc.GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, utils.SerializePair(cfg.pair))
	if err != nil {
		return nil, fmt.Errorf("could not get pair info: %w", err)
	}

	budget, err := cfg.GetCurrentBudget(true)
	if err != nil {
		return nil, fmt.Errorf("could not get current budget: %w", err)
	}

	if balance.Confirmed > cfg.MaxBalance {
		// TODO: properly sweep wallet
		amount := balance.Confirmed - 10000

		checked := check(amount, checkParams{Pair: pairInfo, MaxFeePercent: cfg.maxFeePercent, Budget: &budget.Amount})

		state := boltzrpc.SwapState_PENDING
		pendingSwaps, err := cfg.database.QueryChainSwaps(database.SwapQuery{
			State:    &state,
			TenantId: &cfg.tenant.Id,
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

func (cfg *ChainConfig) execute(recommendation *ChainRecommendation) error {
	if recommendation != nil {
		if recommendation.Dismissed() {
			logger.Debugf("Skipping swap recommendation %+v", recommendation)
			return nil
		}
		logger.Infof("Executing Swap recommendation: %+v", recommendation)
		fromWalletId := cfg.fromWallet.GetWalletInfo().Id
		request := &boltzrpc.CreateChainSwapRequest{
			Amount:       recommendation.Amount,
			Pair:         utils.SerializePair(cfg.pair),
			FromWalletId: &fromWalletId,
		}
		if cfg.ToAddress != "" {
			request.ToAddress = &cfg.ToAddress
		} else {
			toWalletId := cfg.toWallet.GetWalletInfo().Id
			request.ToWalletId = &toWalletId
		}

		return cfg.rpc.CreateAutoChainSwap(cfg.tenant, request)
	}
	return nil
}

func (cfg *ChainConfig) run(stop <-chan bool) {
	updates, stopUpdates := cfg.rpc.GetBlockUpdates(cfg.pair.From)
	defer stopUpdates()
	for {
		select {
		case <-stop:
			return
		case _, ok := <-updates:
			if ok {
				logger.Debugf("Checking for chain swap recommendation")
				recommendation, err := cfg.GetRecommendation()
				if err != nil {
					logger.Warn("Could not get swap recommendation: " + err.Error())
					continue
				}

				if err := cfg.execute(recommendation); err != nil {
					logger.Errorf("Could not act on swap recommendation: %s", err)
				}
			}
		}
	}
}
