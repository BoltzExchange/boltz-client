package autoswap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/BurntSushi/toml"
	"google.golang.org/protobuf/encoding/protojson"
	"os"
	"slices"
)

type shared struct {
	onchain  *onchain.Onchain
	database *database.Database
	rpc      RpcProvider
}

type swapper[T commonConfig] struct {
	shared
	stop        chan bool
	err         error
	swapperType SwapperType
	cfg         T
}

type commonConfig interface {
	Init() error
	GetEnabled() bool
	GetBudgetInterval() uint64
	GetBudget() uint64
	run(stop <-chan bool)
}

type RpcProvider interface {
	GetAutoSwapPairInfo(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error)
	GetLightningChannels() ([]*lightning.LightningChannel, error)
	GetBlockUpdates(currency boltz.Currency) (<-chan *onchain.BlockEpoch, func())
	WalletSendFee(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error)

	CreateAutoSwap(tenant *database.Tenant, request *boltzrpc.CreateSwapRequest) error
	CreateAutoReverseSwap(tenant *database.Tenant, request *boltzrpc.CreateReverseSwapRequest) error
	CreateAutoChainSwap(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error
}

type SwapperType string

const (
	Lightning SwapperType = "lightning"
	Chain     SwapperType = "chain"
)

type Config = autoswaprpc.Config

type AutoSwap struct {
	cfg        *Config
	configPath string

	lnSwapper     *LightningSwapper
	chainSwappers map[database.Id]*ChainSwapper
	err           error

	shared
}

func (autoSwap *AutoSwap) Init(db *database.Database, onchain *onchain.Onchain, configPath string, rpc RpcProvider) {
	autoSwap.shared = shared{
		onchain:  onchain,
		database: db,
		rpc:      rpc,
	}
	autoSwap.configPath = configPath
	autoSwap.chainSwappers = make(map[database.Id]*ChainSwapper)
	autoSwap.cfg = &Config{}

	if onchain != nil {
		go func() {
			for range onchain.OnWalletChange.Get() {
				logger.Debugf("Restarting all auto swappers because of wallet change")
				if swapper := autoSwap.lnSwapper; swapper != nil {
					swapper.start()
				}

				for _, swapper := range autoSwap.chainSwappers {
					swapper.start()
				}
			}
		}()
	}
}

func (autoSwap *AutoSwap) handleErr(err error) error {
	autoSwap.err = err
	return err
}

func (autoSwap *AutoSwap) UpdateLightningConfig(request *autoswaprpc.UpdateLightningConfigRequest) error {
	config := request.Config
	if request.GetReset_() {
		config = DefaultLightningConfig()
	}
	lnSwapper := autoSwap.lnSwapper
	var base *SerializedLnConfig
	if lnSwapper == nil || request.GetReset_() {
		autoSwap.lnSwapper = &LightningSwapper{
			shared:      autoSwap.shared,
			swapperType: Lightning,
		}
		base = DefaultLightningConfig()
	} else {
		base = autoSwap.lnSwapper.cfg.SerializedLnConfig
	}
	if config == nil {
		config = base
	} else {
		updated, err := overwrite(config, base, request.FieldMask)
		if err != nil {
			return err
		}
		config = updated.(*SerializedLnConfig)
	}

	if err := autoSwap.lnSwapper.setConfig(NewLightningConfig(config, autoSwap.shared)); err != nil {
		return err
	}
	return autoSwap.saveConfig()
}

func (autoSwap *AutoSwap) UpdateChainConfig(request *autoswaprpc.UpdateChainConfigRequest, tenant database.Tenant) error {
	chainSwapper, ok := autoSwap.chainSwappers[tenant.Id]
	if request.GetReset_() {
		if ok {
			chainSwapper.Stop()
			delete(autoSwap.chainSwappers, tenant.Id)
		}
	} else {
		config := request.Config
		var base *SerializedChainConfig
		if !ok {
			if request.FieldMask != nil {
				return fmt.Errorf("chain autoSwap needs to be initialized with full config first")
			}
			chainSwapper = &ChainSwapper{
				shared:      autoSwap.shared,
				swapperType: Chain,
			}
			base = &SerializedChainConfig{}
		} else {
			base = chainSwapper.cfg.SerializedChainConfig
		}
		updated, err := overwrite(config, base, request.FieldMask)
		if err != nil {
			return err
		}
		config = updated.(*SerializedChainConfig)
		if tenant.Name != database.DefaultTenantName {
			config.Tenant = &tenant.Name
		}

		if err := chainSwapper.setConfig(NewChainConfig(config, autoSwap.shared)); err != nil {
			return err
		}

		autoSwap.chainSwappers[tenant.Id] = chainSwapper
	}
	return autoSwap.saveConfig()
}

func (autoSwap *AutoSwap) LoadConfig() error {
	var err error

	if !utils.FileExists(autoSwap.configPath) {
		return nil
	}
	serialized := &Config{}
	var cfgToml any
	_, err = toml.DecodeFile(autoSwap.configPath, &cfgToml)
	if err == nil {
		// cant go from toml to proto directly, so we need to marshal again
		cfgJson, _ := json.Marshal(cfgToml)
		if err = protojson.Unmarshal(cfgJson, serialized); err != nil {
			old := &SerializedLnConfig{}
			if errOld := protojson.Unmarshal(cfgJson, old); errOld == nil {
				serialized.Lightning = append(serialized.Lightning, old)
				err = nil
			}
		}
	}

	if err != nil {
		return autoSwap.handleErr(fmt.Errorf("could not load config: %w", err))
	}

	for tenant, chainSwapper := range autoSwap.chainSwappers {
		chainSwapper.Stop()
		delete(autoSwap.chainSwappers, tenant)
	}

	if autoSwap.lnSwapper != nil {
		autoSwap.lnSwapper.Stop()
		autoSwap.lnSwapper = nil
	}

	request := &autoswaprpc.UpdateLightningConfigRequest{}
	if len(serialized.Lightning) > 0 {
		request.Config = serialized.Lightning[0]
		if err := autoSwap.UpdateLightningConfig(request); err != nil {
			logger.Errorf("could not update lightning config: %v", err)
		}
	}

	for _, chainConfig := range serialized.Chain {
		tenant := &database.DefaultTenant
		if chainConfig.Tenant != nil {
			tenant, err = autoSwap.database.GetTenantByName(*chainConfig.Tenant)
			if err != nil {
				logger.Errorf("could not get tenant %s: %v", *chainConfig.Tenant, err)
				continue
			}
		}
		request := &autoswaprpc.UpdateChainConfigRequest{Config: chainConfig}
		if err := autoSwap.UpdateChainConfig(request, *tenant); err != nil {
			logger.Errorf("could not update chain config: %v", err)
		}
	}
	return autoSwap.handleErr(nil)
}

func (autoSwap *AutoSwap) saveConfig() error {
	buf := new(bytes.Buffer)
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true,
	}

	cfg := &Config{}
	if autoSwap.lnSwapper != nil {
		cfg.Lightning = append(cfg.Lightning, autoSwap.lnSwapper.cfg.SerializedLnConfig)
	}
	for _, chainSwapper := range autoSwap.chainSwappers {
		cfg.Chain = append(cfg.Chain, chainSwapper.cfg.SerializedChainConfig)
	}
	marshalled, _ := marshaler.Marshal(cfg)
	var asJson any
	// cant go from json to toml directly, so we need to unmarshal again
	_ = json.Unmarshal(marshalled, &asJson)
	if err := toml.NewEncoder(buf).Encode(asJson); err != nil {
		return autoSwap.handleErr(fmt.Errorf("could not encode config: %w", err))
	}
	autoSwap.cfg = cfg
	if err := os.WriteFile(autoSwap.configPath, buf.Bytes(), 0666); err != nil {
		return autoSwap.handleErr(fmt.Errorf("could not write config to disk: %w", err))
	}
	return autoSwap.handleErr(nil)
}

func (autoSwap *AutoSwap) WalletUsed(id database.Id) bool {
	if autoSwap.lnSwapper != nil {
		used := autoSwap.lnSwapper.cfg.walletId()
		if used != nil && *used == id {
			return true
		}
	}
	for _, chainSwapper := range autoSwap.chainSwappers {
		if chainSwapper.cfg.fromWallet.GetWalletInfo().Id == id {
			return true
		}
	}
	return false
}

func (autoSwap *AutoSwap) GetConfig(tenantId *database.Id) *Config {
	if tenantId == nil {
		return autoSwap.cfg
	}
	scoped := &Config{}
	for tenant, chainSwapper := range autoSwap.chainSwappers {
		if *tenantId == tenant {
			scoped.Chain = append(scoped.Chain, chainSwapper.cfg.SerializedChainConfig)
		}
	}
	if autoSwap.lnSwapper != nil && *tenantId == database.DefaultTenantId {
		scoped.Lightning = []*SerializedLnConfig{autoSwap.lnSwapper.cfg.SerializedLnConfig}
	}
	return scoped
}

func (autoSwap *AutoSwap) GetLnSwapper() *LightningSwapper {
	return autoSwap.lnSwapper
}

func (autoSwap *AutoSwap) GetChainSwapper(tenantId database.Id) *ChainSwapper {
	return autoSwap.chainSwappers[tenantId]
}

func (autoSwap *AutoSwap) Error() string {
	if autoSwap.err != nil {
		return autoSwap.err.Error()
	}
	return ""
}

func (c *swapper[T]) Running() bool {
	return c.stop != nil
}

func (c *swapper[T]) Error() string {
	if c.err != nil {
		return c.err.Error()
	}
	return ""
}

func (c *swapper[T]) setConfig(cfg T) error {
	logger.Debugf("Setting %s autoswap config: %+v", c.swapperType, cfg)
	c.cfg = cfg
	c.start()
	return c.err
}

func (c *swapper[T]) Stop() {
	if c.stop != nil {
		logger.Infof("Stopping %s auto swapper", c.swapperType)
		c.stop <- true
		c.stop = nil
		c.err = nil
	}
}

func (c *swapper[T]) start() {
	c.Stop()
	c.err = c.cfg.Init()
	if c.err != nil {
		logger.Errorf("Autoswap wallet configuration has become invalid: %s", c.err)
		return
	}
	if c.cfg.GetEnabled() {
		logger.Infof("Starting %s auto swapper", c.swapperType)
		c.stop = make(chan bool)
		go c.cfg.run(c.stop)
	}
}
func (c *swapper[T]) GetConfig() T {
	return c.cfg
}

func checkAcceptedReasons(accepted, current []string) error {
	// we don't care if `current` has fewer reasons than `accepted` as long as every reason
	// in `current` is also in `accepted`
	for _, reason := range current {
		if !slices.Contains(accepted, reason) {
			return fmt.Errorf("swap recommendation does not match: %s not in %v", reason, accepted)
		}
	}
	return nil
}
