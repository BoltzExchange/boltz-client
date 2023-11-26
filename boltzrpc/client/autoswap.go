package client

import (
	"context"
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

func (autoSwap *AutoSwap) GetConfig(key string) (*autoswaprpc.Config, error) {
	return autoSwap.Client.GetConfig(autoSwap.Ctx, &autoswaprpc.GetConfigRequest{Key: &key})
}

func (autoSwap *AutoSwap) ReloadConfig() (*autoswaprpc.Config, error) {
	return autoSwap.Client.ReloadConfig(autoSwap.Ctx, &empty.Empty{})
}

func (autoSwap *AutoSwap) ResetConfig() (*autoswaprpc.Config, error) {
	return autoSwap.Client.ResetConfig(autoSwap.Ctx, &empty.Empty{})
}

func (autoSwap *AutoSwap) SetConfigValue(key string, value string) (*autoswaprpc.Config, error) {
	return autoSwap.Client.SetConfigValue(autoSwap.Ctx, &autoswaprpc.SetConfigValueRequest{Key: key, Value: value})
}

func (autoSwap *AutoSwap) Enable() (*autoswaprpc.Config, error) {
	return autoSwap.SetConfigValue("enabled", "true")
}

func (autoSwap *AutoSwap) Disable() (*autoswaprpc.Config, error) {
	return autoSwap.SetConfigValue("enabled", "false")
}
