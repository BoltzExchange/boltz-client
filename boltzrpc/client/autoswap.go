package client

import (
	"context"
	"encoding/json"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/golang/protobuf/ptypes/empty"
)

type AutoSwap struct {
	Client autoswaprpc.AutoSwapClient
	Ctx    context.Context
}

func NewAutoSwapClient(conn Connection) AutoSwap {
	return AutoSwap{
		Ctx:    conn.Ctx,
		Client: autoswaprpc.NewAutoSwapClient(conn.ClientConn),
	}
}

func (autoSwap *AutoSwap) GetSwapRecommendations(noDismissed bool) (*autoswaprpc.GetSwapRecommendationsResponse, error) {
	return autoSwap.Client.GetSwapRecommendations(autoSwap.Ctx, &autoswaprpc.GetSwapRecommendationsRequest{NoDismissed: &noDismissed})
}

func (autoSwap *AutoSwap) GetStatus() (*autoswaprpc.GetStatusResponse, error) {
	return autoSwap.Client.GetStatus(autoSwap.Ctx, &autoswaprpc.GetStatusRequest{})
}

func decodeConfig(config *autoswaprpc.Config, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	var result any
	err = json.Unmarshal([]byte(config.Json), &result)
	return result, err
}

func (autoSwap *AutoSwap) GetConfig(key string) (any, error) {
	return decodeConfig(autoSwap.Client.GetConfig(autoSwap.Ctx, &autoswaprpc.GetConfigRequest{Key: &key}))
}

func (autoSwap *AutoSwap) ReloadConfig() (any, error) {
	return decodeConfig(autoSwap.Client.ReloadConfig(autoSwap.Ctx, &empty.Empty{}))
}

func (autoSwap *AutoSwap) ResetConfig() (any, error) {
	return decodeConfig(autoSwap.Client.ResetConfig(autoSwap.Ctx, &empty.Empty{}))
}

func (autoSwap *AutoSwap) SetConfig(config any) (any, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return decodeConfig(autoSwap.Client.SetConfig(autoSwap.Ctx, &autoswaprpc.Config{Json: string(encoded)}))
}

func (autoSwap *AutoSwap) SetConfigValue(key string, value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return decodeConfig(autoSwap.Client.SetConfigValue(autoSwap.Ctx, &autoswaprpc.SetConfigValueRequest{
		Key:   key,
		Value: string(encoded),
	}))
}

func (autoSwap *AutoSwap) Enable() (any, error) {
	return autoSwap.SetConfigValue("enabled", true)
}

func (autoSwap *AutoSwap) Disable() (any, error) {
	return autoSwap.SetConfigValue("enabled", false)
}
