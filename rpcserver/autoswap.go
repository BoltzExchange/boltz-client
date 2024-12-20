package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/v2/autoswap"
	"github.com/BoltzExchange/boltz-client/v2/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/v2/database"
	"github.com/BoltzExchange/boltz-client/v2/macaroons"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			return nil, err
		}
		response.Lightning = recommendations
	}
	if chainSwapper := server.chainSwapper(ctx); chainSwapper != nil {
		recommendation, err := chainSwapper.GetConfig().GetRecommendation()
		if err != nil {
			return nil, err
		}
		response.Chain = append(response.Chain, recommendation)
	}

	return response, nil
}
func (server *routedAutoSwapServer) ExecuteRecommendations(ctx context.Context, request *autoswaprpc.ExecuteRecommendationsRequest) (*autoswaprpc.ExecuteRecommendationsResponse, error) {
	if err := server.requireSwapper(ctx); err != nil {
		return nil, err
	}

	if len(request.Lightning) > 0 {
		if lnSwapper := server.lnSwapper(ctx); lnSwapper != nil {
			err := lnSwapper.GetConfig().CheckAndExecute(request.Lightning, request.GetForce())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "lightning swaps not configured")
		}
	}

	if len(request.Chain) > 0 {
		if chainSwapper := server.chainSwapper(ctx); chainSwapper != nil {
			for _, recommendation := range request.Chain {
				err := chainSwapper.GetConfig().CheckAndExecute(recommendation.Swap, request.GetForce())
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "chain swaps not configured")
		}
	}

	return &autoswaprpc.ExecuteRecommendationsResponse{}, nil
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
		return nil, err
	}

	return server.GetConfig(ctx, &autoswaprpc.GetConfigRequest{})
}
