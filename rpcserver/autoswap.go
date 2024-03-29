package rpcserver

import (
	"context"
	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/macaroons"
	"github.com/golang/protobuf/ptypes/empty"
)

type routedAutoSwapServer struct {
	autoswaprpc.AutoSwapServer

	database *database.Database
	swapper  *autoswap.AutoSwapper
}

func (server *routedAutoSwapServer) lnSwapper(ctx context.Context) *autoswap.LightningSwapper {
	if isAdmin(ctx) {
		return server.swapper.GetLnSwapper()
	}
	return nil
}

func (server *routedAutoSwapServer) chainSwapper(ctx context.Context) *autoswap.ChainSwapper {
	return server.swapper.GetChainSwapper(requireEntity(ctx))
}

func (server *routedAutoSwapServer) GetRecommendations(ctx context.Context, request *autoswaprpc.GetRecommendationsRequest) (*autoswaprpc.GetRecommendationsResponse, error) {
	if err := server.swapper.RequireConfig(); err != nil {
		return nil, err
	}
	response := &autoswaprpc.GetRecommendationsResponse{}
	if lnSwapper := server.lnSwapper(ctx); lnSwapper != nil {
		recommendations, err := lnSwapper.GetSwapRecommendations()

		if err != nil {
			return nil, handleError(err)
		}

		for _, recommendation := range recommendations {
			noDismissed := request.GetNoDismissed()
			if !noDismissed || !recommendation.Dismissed() {
				response.Lightning = append(response.Lightning, &autoswaprpc.LightningRecommendation{
					Type:             string(recommendation.Type),
					Amount:           recommendation.Amount,
					Channel:          serializeLightningChannel(recommendation.Channel),
					FeeEstimate:      recommendation.FeeEstimate,
					DismissedReasons: recommendation.DismissedReasons,
				})
			}
		}
	}
	if chainSwapper := server.chainSwapper(ctx); chainSwapper != nil {
		recommendation, err := chainSwapper.GetRecommendation()
		if err != nil {
			return nil, handleError(err)
		}
		if recommendation != nil && (!request.GetNoDismissed() || !recommendation.Dismissed()) {
			response.Chain = append(response.Chain, &autoswaprpc.ChainRecommendation{
				Amount:           recommendation.Amount,
				FeeEstimate:      recommendation.FeeEstimate,
				DismissedReasons: recommendation.DismissedReasons,
			})
		}
	}

	return response, nil
}

func (server *routedAutoSwapServer) GetStatus(ctx context.Context, _ *autoswaprpc.GetStatusRequest) (*autoswaprpc.GetStatusResponse, error) {
	response := &autoswaprpc.GetStatusResponse{
		Error: serializeOptionalString(server.swapper.Error()),
	}
	if isAdmin(ctx) {
		lnSwapper := server.swapper.GetLnSwapper()
		if lnSwapper != nil {
			cfg := lnSwapper.GetConfig()
			status := &autoswaprpc.Status{
				Running:     lnSwapper.Running(),
				Error:       serializeOptionalString(lnSwapper.Error()),
				Description: cfg.Description(),
			}

			budget, err := lnSwapper.GetCurrentBudget(false)
			if err != nil {
				return nil, err
			}

			if budget != nil {
				status.Budget = &autoswaprpc.Budget{
					Total:     budget.Total,
					StartDate: serializeTime(budget.StartDate),
					EndDate:   serializeTime(budget.EndDate),
					Remaining: budget.Amount,
				}

				auto := true
				status.Stats, err = server.database.QueryStats(database.SwapQuery{Since: budget.StartDate, IsAuto: &auto}, false)
				if err != nil {
					return nil, err
				}
			}
			response.Lightning = status
		}
	}

	chainSwapper := server.swapper.GetChainSwapper(requireEntity(ctx))
	if chainSwapper != nil {
		cfg := chainSwapper.GetConfig()
		status := &autoswaprpc.Status{
			Running:     chainSwapper.Running(),
			Error:       serializeOptionalString(chainSwapper.Error()),
			Description: cfg.Description(),
		}

		response.Chain = status
	}

	return response, nil
}

func (server *routedAutoSwapServer) GetConfig(ctx context.Context, request *autoswaprpc.GetConfigRequest) (*autoswaprpc.Config, error) {
	var err error

	config, err := server.swapper.GetConfig(requireEntity(ctx))
	if err != nil {
		return nil, handleError(err)
	}
	return config, nil
}

func (server *routedAutoSwapServer) ReloadConfig(ctx context.Context, _ *empty.Empty) (*autoswaprpc.Config, error) {
	err := server.swapper.LoadConfig()
	if err != nil {
		return nil, err
	}
	return server.GetConfig(ctx, nil)
}

func (server *routedAutoSwapServer) UpdateLightningConfig(ctx context.Context, request *autoswaprpc.UpdateLightningConfigRequest) (*autoswaprpc.Config, error) {
	if err := server.swapper.UpdateLightningConfig(request); err != nil {
		return nil, err
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}

func (server *routedAutoSwapServer) UpdateChainConfig(ctx context.Context, request *autoswaprpc.UpdateChainConfigRequest) (*autoswaprpc.Config, error) {
	if err := server.swapper.UpdateChainConfig(request, macaroons.EntityFromContext(ctx)); err != nil {
		return nil, handleError(err)
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}
