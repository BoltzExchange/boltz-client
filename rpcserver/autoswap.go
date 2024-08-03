package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/macaroons"
	"github.com/golang/protobuf/ptypes/empty"
)

type routedAutoSwapServer struct {
	autoswaprpc.AutoSwapServer

	database *database.Database
	swapper  *autoswap.AutoSwap
}

func (server *routedAutoSwapServer) lnSwapper(ctx context.Context) *autoswap.LightningSwapper {
	if isAdmin(ctx) {
		return server.swapper.GetLnSwapper()
	}
	return nil
}

func (server *routedAutoSwapServer) chainSwapper(ctx context.Context) *autoswap.ChainSwapper {
	return server.swapper.GetChainSwapper(requireTenantId(ctx))
}

func (server *routedAutoSwapServer) requireSwapper(ctx context.Context) error {
	if err := server.swapper.Error(); err != "" {
		return fmt.Errorf("autoswap: %s", err)
	}
	if server.lnSwapper(ctx) == nil && server.chainSwapper(ctx) == nil {
		return errors.New("autoswap not configured")
	}
	return nil
}

func (server *routedAutoSwapServer) GetRecommendations(ctx context.Context, _ *autoswaprpc.GetRecommendationsRequest) (*autoswaprpc.GetRecommendationsResponse, error) {
	if err := server.requireSwapper(ctx); err != nil {
		return nil, err
	}
	response := &autoswaprpc.GetRecommendationsResponse{}
	if lnSwapper := server.lnSwapper(ctx); lnSwapper != nil {
		recommendations, err := lnSwapper.GetConfig().GetSwapRecommendations(true)

		if err != nil {
			return nil, handleError(err)
		}

		for _, recommendation := range recommendations {
			response.Lightning = append(response.Lightning, &autoswaprpc.LightningRecommendation{
				Swap:    serializeLightningSwap(recommendation.Swap),
				Channel: serializeLightningChannel(recommendation.Channel),
			})
		}
	}
	if chainSwapper := server.chainSwapper(ctx); chainSwapper != nil {
		recommendation, err := chainSwapper.GetConfig().GetRecommendation()
		if err != nil {
			return nil, handleError(err)
		}
		if recommendation != nil {
			response.Chain = append(response.Chain, &autoswaprpc.ChainRecommendation{
				Swap:          serializeAutoChainSwap(recommendation.Swap),
				WalletBalance: serializeWalletBalance(recommendation.FromBalance),
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
		budget, err := lnSwapper.GetConfig().GetCurrentBudget(false)
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
		budget, err := chainSwapper.GetConfig().GetCurrentBudget(false)
		if err != nil {
			return nil, err
		}
		chain.Running = chainSwapper.Running()
		chain.Error = serializeOptionalString(chainSwapper.Error())
		chain.Description = cfg.Description()
		chain.Budget = serializeBudget(budget)
	}

	return &autoswaprpc.GetStatusResponse{
		Lightning: ln,
		Chain:     chain,
	}, nil
}

func (server *routedAutoSwapServer) GetConfig(ctx context.Context, _ *autoswaprpc.GetConfigRequest) (*autoswaprpc.Config, error) {
	return server.swapper.GetConfig(macaroons.TenantIdFromContext(ctx)), nil
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
	if err := server.swapper.UpdateChainConfig(request, requireTenant(ctx)); err != nil {
		return nil, handleError(err)
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}
