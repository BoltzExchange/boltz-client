package rpcserver

import (
	"context"
	"encoding/json"

	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/golang/protobuf/ptypes/empty"
)

type routedAutoSwapServer struct {
	autoswaprpc.AutoSwapServer

	database *database.Database
	swapper  *autoswap.AutoSwapper
}

func (server *routedAutoSwapServer) GetSwapRecommendations(_ context.Context, request *autoswaprpc.GetSwapRecommendationsRequest) (*autoswaprpc.GetSwapRecommendationsResponse, error) {
	recommendations, err := server.swapper.GetSwapRecommendations()

	if err != nil {
		return nil, handleError(err)
	}

	var swaps []*autoswaprpc.SwapRecommendation
	for _, recommendation := range recommendations {
		noDismissed := request.NoDismissed != nil && *request.NoDismissed
		if !noDismissed || !recommendation.Dismissed() {
			swaps = append(swaps, &autoswaprpc.SwapRecommendation{
				Type:             string(recommendation.Type),
				Amount:           recommendation.Amount,
				Channel:          serializeLightningChannel(recommendation.Channel),
				FeeEstimate:      recommendation.FeeEstimate,
				DismissedReasons: recommendation.DismissedReasons,
			})
		}
	}

	return &autoswaprpc.GetSwapRecommendationsResponse{
		Swaps: swaps,
	}, nil
}

func (server *routedAutoSwapServer) GetStatus(_ context.Context, request *autoswaprpc.GetStatusRequest) (*autoswaprpc.GetStatusResponse, error) {
	response := &autoswaprpc.GetStatusResponse{
		Running: server.swapper.Running(),
		Error:   server.swapper.Error(),
	}
	cfg, err := server.swapper.GetConfig()
	if err == nil {
		response.Strategy = cfg.StrategyName()

		budget, err := server.swapper.GetCurrentBudget(false)
		if err != nil {
			return nil, err
		}

		if budget != nil {
			response.Budget = &autoswaprpc.Budget{
				Total:     budget.Total,
				StartDate: serializeTime(budget.StartDate),
				EndDate:   serializeTime(budget.EndDate),
				Remaining: budget.Amount,
			}

			auto := true
			stats, err := server.database.QueryStats(database.SwapQuery{Since: budget.StartDate, IsAuto: &auto})
			if err != nil {
				return nil, err
			}
			response.Stats = stats
		}
	}

	return response, nil
}

func (server *routedAutoSwapServer) GetConfig(ctx context.Context, request *autoswaprpc.GetConfigRequest) (*autoswaprpc.Config, error) {
	var err error

	config, err := server.swapper.GetConfig()
	if err != nil {
		return nil, handleError(err)
	}
	return config.SerializedConfig, nil
}

func (server *routedAutoSwapServer) ResetConfig(ctx context.Context, _ *empty.Empty) (*autoswaprpc.Config, error) {
	if err := server.swapper.SetConfig(autoswap.DefaultConfig()); err != nil {
		return nil, handleError(err)
	}
	return server.GetConfig(ctx, nil)
}

func (server *routedAutoSwapServer) ReloadConfig(ctx context.Context, _ *empty.Empty) (*autoswaprpc.Config, error) {
	err := server.swapper.LoadConfig()
	if err != nil {
		return nil, err
	}
	return server.GetConfig(ctx, nil)
}

func (server *routedAutoSwapServer) SetConfig(ctx context.Context, request *autoswaprpc.Config) (*autoswaprpc.Config, error) {
	if err := server.swapper.SetConfig(request); err != nil {
		return nil, err
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}

func (server *routedAutoSwapServer) SetConfigValue(ctx context.Context, request *autoswaprpc.SetConfigValueRequest) (*autoswaprpc.Config, error) {
	var value any
	if err := json.Unmarshal([]byte(request.Value), &value); err != nil {
		return nil, err
	}

	if err := server.swapper.SetConfigValue(request.Key, value); err != nil {
		return nil, err
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{Key: &request.Key})
}
