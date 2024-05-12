package autoswap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/BurntSushi/toml"
	"google.golang.org/protobuf/encoding/protojson"
	"os"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
)

var ErrorNotConfigured = errors.New("autoswap not configured")

type common struct {
	onchain     *onchain.Onchain
	database    *database.Database
	stop        chan bool
	err         error
	swapperType SwapperType
}

type RpcProvider interface {
	GetAutoSwapPairInfo(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error)
	GetLightningChannels() ([]*lightning.LightningChannel, error)
	GetBlockUpdates(currency boltz.Currency) (<-chan *onchain.BlockEpoch, func())

	CreateAutoSwap(entity *database.Entity, request *boltzrpc.CreateSwapRequest) error
	CreateAutoReverseSwap(entity *database.Entity, request *boltzrpc.CreateReverseSwapRequest) error
	CreateAutoChainSwap(entity *database.Entity, request *boltzrpc.CreateChainSwapRequest) error
}

type SwapperType string

const (
	Lightning SwapperType = "lightning"
	Chain     SwapperType = "chain"
)

type Config = autoswaprpc.Config

type AutoSwapper struct {
	common
	cfg        *Config
	configPath string

	lnSwapper     *LightningSwapper
	chainSwappers map[database.Id]*ChainSwapper

	Rpc RpcProvider
}

func (swapper *AutoSwapper) Init(db *database.Database, onchain *onchain.Onchain, configPath string) {
	swapper.common = common{database: db, onchain: onchain}
	swapper.configPath = configPath
	swapper.chainSwappers = make(map[database.Id]*ChainSwapper)

	if onchain != nil {
		go func() {
			for range onchain.OnWalletChange.Get() {
				logger.Info("Restarting all auto swappers because of wallet change")
				if swapper := swapper.lnSwapper; swapper != nil {
					swapper.Restart()
				}

				for _, swapper := range swapper.chainSwappers {
					swapper.Restart()
				}
			}
		}()
	}
}

func (swapper *AutoSwapper) UpdateLightningConfig(request *autoswaprpc.UpdateLightningConfigRequest) error {
	config := request.Config
	if request.GetReset_() {
		config = DefaultLightningConfig()
	}
	var base *SerializedLnConfig
	if swapper.lnSwapper == nil || request.GetReset_() {
		swapper.lnSwapper = &LightningSwapper{
			rpc: swapper.Rpc,
			common: common{
				onchain:     swapper.onchain,
				database:    swapper.database,
				swapperType: Lightning,
			},
		}
		base = DefaultLightningConfig()
	} else {
		base = swapper.lnSwapper.cfg.SerializedLnConfig
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

	cfg := NewConfig(config)
	if err := cfg.Init(swapper.onchain); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	swapper.lnSwapper.setConfig(cfg)
	return swapper.saveConfig()
}

func (swapper *AutoSwapper) UpdateChainConfig(request *autoswaprpc.UpdateChainConfigRequest, entity *database.Entity) error {
	entityId := database.DefaultEntityId
	var entityName *string
	if entity != nil {
		entityId = entity.Id
		entityName = &entity.Name
	}
	if request.GetReset_() {
		delete(swapper.chainSwappers, entityId)
	} else {
		chainSwapper, ok := swapper.chainSwappers[entityId]
		config := request.Config
		var base *SerializedChainConfig
		if !ok {
			if request.FieldMask != nil {
				return fmt.Errorf("chain swapper needs to be initialized with full config first")
			}
			chainSwapper = &ChainSwapper{
				rpc: swapper.Rpc,
				common: common{
					onchain:     swapper.onchain,
					database:    swapper.database,
					swapperType: Chain,
				},
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
		config.Entity = entityName

		cfg := NewChainConfig(config)
		if err := cfg.Init(swapper.database, swapper.onchain); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}

		chainSwapper.setConfig(cfg)

		swapper.chainSwappers[entityId] = chainSwapper
	}
	return swapper.saveConfig()
}

func (swapper *AutoSwapper) LoadConfig() error {
	var err error

	if !utils.FileExists(swapper.configPath) {
		return nil
	}
	serialized := &Config{}
	var cfgToml any
	if _, err = toml.DecodeFile(swapper.configPath, &cfgToml); err != nil {
		err = fmt.Errorf("Could not decode autoswap config: " + err.Error())
	}
	if err == nil {
		// cant go from toml to proto directly, so we need to marshal again
		cfgJson, _ := json.Marshal(cfgToml)
		if err = protojson.Unmarshal(cfgJson, serialized); err != nil {
			old := &SerializedLnConfig{}
			if errOld := protojson.Unmarshal(cfgJson, old); errOld != nil {
				err = fmt.Errorf("Could not decode autoswap config: " + err.Error())
			} else {
				serialized.Lightning = append(serialized.Lightning, old)
				err = nil
			}
		}
	}

	handleErr := func(err error) error {
		// only set error if we dont have a config yet
		if swapper.cfg == nil {
			swapper.err = err
		}
		return err
	}

	if err != nil {
		return handleErr(err)
	}

	request := &autoswaprpc.UpdateLightningConfigRequest{}
	if len(serialized.Lightning) > 0 {
		request.Config = serialized.Lightning[0]
	}
	if err := swapper.UpdateLightningConfig(request); err != nil {
		return handleErr(fmt.Errorf("could not update lightning config: %v", err))
	}

	for _, chainConfig := range serialized.Chain {
		name := database.DefaultEntityName
		if chainConfig.Entity != nil {
			name = *chainConfig.Entity
		}
		entity, err := swapper.database.GetEntityByName(name)
		if err != nil {
			return handleErr(fmt.Errorf("could not get entity %s: %v", *chainConfig.Entity, err))
		}
		request := &autoswaprpc.UpdateChainConfigRequest{Config: chainConfig}
		if err := swapper.UpdateChainConfig(request, entity); err != nil {
			return handleErr(fmt.Errorf("could not update chain config: %v", err))
		}
	}
	return nil
}

func (swapper *AutoSwapper) saveConfig() error {
	buf := new(bytes.Buffer)
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated:   true,
		EmitDefaultValues: true,
	}

	cfg := &Config{}
	if swapper.lnSwapper != nil {
		cfg.Lightning = append(cfg.Lightning, swapper.lnSwapper.cfg.SerializedLnConfig)
	}
	for _, chainSwapper := range swapper.chainSwappers {
		cfg.Chain = append(cfg.Chain, chainSwapper.cfg.SerializedChainConfig)
	}
	marshalled, _ := marshaler.Marshal(cfg)
	var asJson any
	// cant go from json to toml directly, so we need to unmarshal again
	_ = json.Unmarshal(marshalled, &asJson)
	if err := toml.NewEncoder(buf).Encode(asJson); err != nil {
		return err
	}
	swapper.cfg = cfg
	return os.WriteFile(swapper.configPath, buf.Bytes(), 0666)
}

func (swapper *AutoSwapper) RequireConfig() error {
	if swapper.cfg == nil {
		if swapper.err != nil {
			return fmt.Errorf("%w: %w", ErrorNotConfigured, swapper.err)
		}
		return ErrorNotConfigured
	}
	return nil
}

func (swapper *AutoSwapper) WalletUsed(name string) bool {
	if swapper.cfg.Lightning != nil {
		for _, ln := range swapper.cfg.Lightning {
			if ln.Wallet == name {
				return true
			}
		}
	}
	if swapper.cfg.Chain != nil {
		for _, cfg := range swapper.cfg.Chain {
			if cfg.FromWallet == name || cfg.ToWallet == name {
				return true
			}
		}
	}
	return false
}

func (swapper *AutoSwapper) GetConfig(entityId database.Id) (*Config, error) {
	if err := swapper.RequireConfig(); err != nil {
		return nil, err
	}
	scoped := &Config{}
	for entity, chainSwapper := range swapper.chainSwappers {
		if entityId == entity {
			scoped.Chain = append(scoped.Chain, chainSwapper.cfg.SerializedChainConfig)
		}
	}
	if entityId == database.DefaultEntityId && swapper.lnSwapper != nil {
		scoped.Lightning = []*SerializedLnConfig{swapper.lnSwapper.cfg.SerializedLnConfig}
	}
	return scoped, nil
}

func (swapper *AutoSwapper) GetLnSwapper() *LightningSwapper {
	return swapper.lnSwapper
}

func (swapper *AutoSwapper) GetChainSwapper(entityId database.Id) *ChainSwapper {
	return swapper.chainSwappers[entityId]
}

func (c *common) Running() bool {
	return c.stop != nil
}

func (c *common) Error() string {
	if c.err != nil {
		return c.err.Error()
	}
	return ""
}

func (c *common) Stop() {
	if c.stop != nil {
		logger.Infof("Stopping %s auto swapper", c.swapperType)
		c.stop <- true
		c.stop = nil
		c.err = nil
	}
}
