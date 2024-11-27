package client

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"strconv"
	"strings"
)

type AutoSwap struct {
	Connection
	Client autoswaprpc.AutoSwapClient
}

func NewAutoSwapClient(conn Connection) AutoSwap {
	return AutoSwap{
		Connection: conn,
		Client:     autoswaprpc.NewAutoSwapClient(conn.ClientConn),
	}
}

func (autoSwap *AutoSwap) GetRecommendations() (*autoswaprpc.GetRecommendationsResponse, error) {
	return autoSwap.Client.GetRecommendations(autoSwap.Ctx, &autoswaprpc.GetRecommendationsRequest{})
}

func (autoSwap *AutoSwap) ExecuteRecommendations(request *autoswaprpc.ExecuteRecommendationsRequest) (*autoswaprpc.ExecuteRecommendationsResponse, error) {
	return autoSwap.Client.ExecuteRecommendations(autoSwap.Ctx, request)
}

func (autoSwap *AutoSwap) GetStatus() (*autoswaprpc.GetStatusResponse, error) {
	return autoSwap.Client.GetStatus(autoSwap.Ctx, &autoswaprpc.GetStatusRequest{})
}

func (autoSwap *AutoSwap) GetConfig() (*autoswaprpc.Config, error) {
	return autoSwap.Client.GetConfig(autoSwap.Ctx, &autoswaprpc.GetConfigRequest{})
}

func (autoSwap *AutoSwap) GetLightningConfig() (*autoswaprpc.LightningConfig, error) {
	config, err := autoSwap.GetConfig()
	if err != nil {
		return nil, err
	}
	if config.Lightning == nil {
		return nil, errors.New("lightning config not set")
	}
	return config.Lightning[0], nil
}

func (autoSwap *AutoSwap) GetChainConfig() (*autoswaprpc.ChainConfig, error) {
	config, err := autoSwap.GetConfig()
	if err != nil {
		return nil, err
	}
	if config.Chain == nil {
		return nil, errors.New("chain config not set")
	}
	return config.Chain[0], nil
}

func (autoSwap *AutoSwap) ReloadConfig() (*autoswaprpc.Config, error) {
	return autoSwap.Client.ReloadConfig(autoSwap.Ctx, &empty.Empty{})
}

func (autoSwap *AutoSwap) SetConfigValue(swapper AutoSwapType, key string, value any) (*autoswaprpc.Config, error) {
	if swapper == LnAutoSwap {
		config := &autoswaprpc.LightningConfig{}
		mask, err := setValue(config, key, value)
		if err != nil {
			return nil, err
		}
		return autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{
			Config:    config,
			FieldMask: mask,
		})
	} else {
		config := &autoswaprpc.ChainConfig{}
		mask, err := setValue(config, key, value)
		if err != nil {
			return nil, err
		}
		return autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{
			Config:    config,
			FieldMask: mask,
		})
	}
}

func (autoSwap *AutoSwap) SetLightningConfigValue(key string, value any) (*autoswaprpc.Config, error) {
	return autoSwap.SetConfigValue(LnAutoSwap, key, value)
}

func (autoSwap *AutoSwap) SetChainConfigValue(key string, value any) (*autoswaprpc.Config, error) {
	return autoSwap.SetConfigValue(ChainAutoSwap, key, value)
}

func getField(message protoreflect.ProtoMessage, name string) (protoreflect.FieldDescriptor, error) {
	fields := message.ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if strings.EqualFold(name, field.JSONName()) {
			return field, nil
		}
	}
	return nil, errors.New("Unknown field")
}

func setValue(message protoreflect.ProtoMessage, name string, value any) (*fieldmaskpb.FieldMask, error) {
	field, err := getField(message, name)
	if err != nil {
		return nil, err
	}

	stringValue := fmt.Sprint(value)
	var setValue any

	switch field.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		value, err := strconv.ParseInt(stringValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer: %w", err)
		}
		setValue = value
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		value, err := strconv.ParseUint(stringValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer: %w", err)
		}
		setValue = value
	case protoreflect.FloatKind:
		value, err := strconv.ParseFloat(stringValue, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %w", err)
		}
		setValue = value
	case protoreflect.BoolKind:
		value, err := strconv.ParseBool(stringValue)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean value: %w", err)
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
			return nil, fmt.Errorf("invalid enum value: %s", stringValue)
		}

	default:
		return nil, errors.New("Unknown field type")
	}
	message.ProtoReflect().Set(field, protoreflect.ValueOf(setValue))
	return fieldmaskpb.New(message, string(field.Name()))
}

func (autoSwap *AutoSwap) UpdateLightningConfig(request *autoswaprpc.UpdateLightningConfigRequest) (*autoswaprpc.Config, error) {
	return autoSwap.Client.UpdateLightningConfig(autoSwap.Ctx, request)
}

func (autoSwap *AutoSwap) UpdateChainConfig(request *autoswaprpc.UpdateChainConfigRequest) (*autoswaprpc.Config, error) {
	return autoSwap.Client.UpdateChainConfig(autoSwap.Ctx, request)
}

func (autoSwap *AutoSwap) ResetConfig(swapper AutoSwapType) (*autoswaprpc.Config, error) {
	reset := true
	if swapper == LnAutoSwap {
		return autoSwap.Client.UpdateLightningConfig(autoSwap.Ctx, &autoswaprpc.UpdateLightningConfigRequest{Reset_: &reset})
	} else {
		return autoSwap.Client.UpdateChainConfig(autoSwap.Ctx, &autoswaprpc.UpdateChainConfigRequest{Reset_: &reset})
	}
}

func (autoSwap *AutoSwap) Enable() (any, error) {
	return autoSwap.SetLightningConfigValue("enabled", true)
}

func (autoSwap *AutoSwap) Disable() (any, error) {
	return autoSwap.SetLightningConfigValue("enabled", false)
}
