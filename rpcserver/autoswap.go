package rpcserver

import (
	"context"
	"errors"
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
	return server.swapper.GetChainSwapper(requireEntityId(ctx))
}

func (server *routedAutoSwapServer) requireSwapper(ctx context.Context) error {
	if server.lnSwapper(ctx) == nil && server.chainSwapper(ctx) == nil {
		return errors.New("autoswap not configured")
	}
	return nil
}

func (server *routedAutoSwapServer) GetRecommendations(ctx context.Context, request *autoswaprpc.GetRecommendationsRequest) (*autoswaprpc.GetRecommendationsResponse, error) {
	if err := server.requireSwapper(ctx); err != nil {
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

func serializeBudget(budget *autoswap.Budget) *autoswaprpc.Budget {
	if budget == nil {
		return nil
	}
	return &autoswaprpc.Budget{
		Total:     budget.Total,
		StartDate: serializeTime(budget.StartDate),
		EndDate:   serializeTime(budget.EndDate),
		Remaining: budget.Amount,
		Stats:     budget.Stats,
	}
}

func (server *routedAutoSwapServer) GetStatus(ctx context.Context, _ *autoswaprpc.GetStatusRequest) (*autoswaprpc.GetStatusResponse, error) {
	if err := server.requireSwapper(ctx); err != nil {
		return nil, err
	}
	ln := &autoswaprpc.Status{Running: false}
	chain := &autoswaprpc.Status{Running: false}

	if lnSwapper := server.lnSwapper(ctx); lnSwapper != nil {
		cfg := lnSwapper.GetConfig()
		budget, err := lnSwapper.GetCurrentBudget(false)
		if err != nil {
			return nil, err
		}
		ln.Running = lnSwapper.Running()
		ln.Error = serializeOptionalString(lnSwapper.Error())
		ln.Description = cfg.Description()
		ln.Budget = serializeBudget(budget)
	}

	if chainSwapper := server.chainSwapper(ctx); chainSwapper != nil {
		cfg := chainSwapper.GetConfig()
		budget, err := chainSwapper.GetCurrentBudget(false)
		if err != nil {
			return nil, err
		}
		chain.Running = chainSwapper.Running()
		chain.Error = serializeOptionalString(chainSwapper.Error())
		chain.Description = cfg.Description()
		chain.Budget = serializeBudget(budget)
	}

	return &autoswaprpc.GetStatusResponse{
		Error:     serializeOptionalString(server.swapper.Error()),
		Lightning: ln,
		Chain:     chain,
	}, nil
}

func (server *routedAutoSwapServer) GetConfig(ctx context.Context, _ *autoswaprpc.GetConfigRequest) (*autoswaprpc.Config, error) {
	if err := server.requireSwapper(ctx); err != nil {
		return nil, err
	}
	return server.swapper.GetConfig(macaroons.EntityIdFromContext(ctx)), nil
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
	if err := server.swapper.UpdateChainConfig(request, requireEntity(ctx)); err != nil {
		return nil, handleError(err)
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}
