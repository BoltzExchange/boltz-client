package autoswap

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/serializers"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync"
)

type ChainSwap = checks

type ChainSwapper = swapper[*ChainConfig]
type SerializedChainConfig = autoswaprpc.ChainConfig

type ChainRecommendation struct {
	Swap        *ChainSwap
	FromBalance *onchain.Balance
}

const MinReserve = uint64(10000)

type ChainConfig struct {
	*SerializedChainConfig
	shared

	tenant        *database.Tenant
	maxFeePercent boltz.Percentage
	fromWallet    onchain.Wallet
	toWallet      onchain.Wallet
	pair          boltz.Pair
	description   string

	executeLock sync.Mutex
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
	cfg.maxFeePercent = boltz.Percentage(cfg.MaxFeePercent)
	if cfg.MaxBalance == 0 {
		return errors.New("MaxBalance must be set")
	}

	if cfg.MaxBalance < cfg.ReserveBalance {
		return fmt.Errorf("reserve balance %d is greater than max balance %d", cfg.ReserveBalance, cfg.MaxBalance)
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

func (cfg *ChainConfig) GetRecommendation() (*autoswaprpc.ChainRecommendation, error) {
	cfg.executeLock.Lock()
	defer cfg.executeLock.Unlock()
	return cfg.getRecommendation()
}

func (cfg *ChainConfig) getRecommendation() (*autoswaprpc.ChainRecommendation, error) {
	balance, err := cfg.fromWallet.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("could not get wallet balance: %w", err)
	}

	pairInfo, err := cfg.rpc.GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, serializers.SerializePair(cfg.pair))
	if err != nil {
		return nil, fmt.Errorf("could not get pair info: %w", err)
	}

	budget, err := cfg.GetCurrentBudget(true)
	if err != nil {
		return nil, fmt.Errorf("could not get current budget: %w", err)
	}

	recommendation := &ChainRecommendation{FromBalance: balance}
	if balance.Confirmed > cfg.MaxBalance {
		sendAll := cfg.ReserveBalance == 0
		amount := balance.Confirmed - cfg.ReserveBalance
		sendFee, err := cfg.rpc.WalletSendFee(&boltzrpc.WalletSendRequest{SendAll: &sendAll, Amount: amount, Id: cfg.fromWallet.GetWalletInfo().Id})
		if err != nil {
			if status.Code(err) == codes.InvalidArgument {
				if sendAll {
					amount -= MinReserve
				}
			} else {
				return nil, fmt.Errorf("could not get send fee: %w", err)
			}
		} else {
			amount = sendFee.Amount
		}

		checked := check(amount, checkParams{Pair: pairInfo, MaxFeePercent: cfg.maxFeePercent, Budget: &budget.Amount})

		pendingSwaps, err := cfg.database.QueryChainSwaps(database.SwapQuery{
			States:   []boltzrpc.SwapState{boltzrpc.SwapState_PENDING},
			TenantId: &cfg.tenant.Id,
		})
		if err != nil {
			return nil, fmt.Errorf("could not query pending swaps: %w", err)
		}
		if len(pendingSwaps) > 0 {
			checked.Dismiss(ReasonPendingSwap)
		}
		recommendation.Swap = &checked
	}
	return &autoswaprpc.ChainRecommendation{
		Swap:          serializeAutoChainSwap(recommendation.Swap),
		WalletBalance: serializers.SerializeWalletBalance(recommendation.FromBalance),
	}, nil
}

func (cfg *ChainConfig) CheckAndExecute(accepted *autoswaprpc.ChainSwap, force bool) error {
	cfg.executeLock.Lock()
	defer cfg.executeLock.Unlock()
	logger.Debugf("Checking for chain swap recommendation")
	recommendation, err := cfg.getRecommendation()
	if err != nil {
		return fmt.Errorf("could not get swap recommendation: %w", err)
	}
	return cfg.execute(recommendation.Swap, accepted, force)
}

func (cfg *ChainConfig) execute(swap *autoswaprpc.ChainSwap, accepted *autoswaprpc.ChainSwap, force bool) error {
	if swap != nil {
		if accepted != nil {
			if err := checkAcceptedReasons(accepted.DismissedReasons, swap.DismissedReasons); err != nil {
				return err
			}
		}
		if !force && len(swap.DismissedReasons) > 0 {
			logger.Debugf("Skipping swap recommendation %+v", swap)
			return nil
		}
		logger.Infof("Executing Swap recommendation: %+v", swap)
		fromWalletId := cfg.fromWallet.GetWalletInfo().Id
		request := &boltzrpc.CreateChainSwapRequest{
			Amount:       &swap.Amount,
			Pair:         serializers.SerializePair(cfg.pair),
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
				if err := cfg.CheckAndExecute(nil, false); err != nil {
					logger.Errorf("Chain autoswap: %s", err)
				}
			}
		}
	}
}
