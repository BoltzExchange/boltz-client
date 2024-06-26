package autoswap

import (
	"errors"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"time"

	"github.com/BoltzExchange/boltz-client/database"
)

type Budget struct {
	database.BudgetInterval
	Amount int64
	Total  uint64
	Stats  *boltzrpc.SwapStats
}

type budgetConfig interface {
	GetBudgetInterval() uint64
	GetBudget() uint64
}

func (c *shared) GetCurrentBudget(
	createIfMissing bool,
	swapperType SwapperType,
	cfg budgetConfig,
	entityId database.Id,
) (*Budget, error) {
	budgetDuration := time.Duration(cfg.GetBudgetInterval()) * time.Second
	totalBudget := cfg.GetBudget()

	currentInterval, err := c.database.QueryCurrentBudgetInterval(string(swapperType), entityId)
	if err != nil {
		return nil, errors.New("Could not get budget period: " + err.Error())
	}

	now := time.Now()
	if currentInterval == nil || now.After(currentInterval.EndDate) {
		if createIfMissing {
			if currentInterval == nil {
				currentInterval = &database.BudgetInterval{
					StartDate: now,
					EndDate:   now.Add(budgetDuration),
					Name:      string(swapperType),
					EntityId:  entityId,
				}
			}
			for now.After(currentInterval.EndDate) {
				currentInterval.StartDate = currentInterval.EndDate
				currentInterval.EndDate = currentInterval.EndDate.Add(budgetDuration)
			}
			if err := c.database.CreateBudget(*currentInterval); err != nil {
				return nil, errors.New("Could not create budget period: " + err.Error())
			}
		} else {
			return nil, nil
		}
	}

	isAuto := true
	var swapTypes []boltz.SwapType
	if swapperType == Lightning {
		swapTypes = []boltz.SwapType{boltz.NormalSwap, boltz.ReverseSwap}
	} else {
		swapTypes = []boltz.SwapType{boltz.ChainSwap}
	}
	stats, err := c.database.QueryStats(database.SwapQuery{
		Since:    currentInterval.StartDate,
		IsAuto:   &isAuto,
		EntityId: &entityId,
	}, swapTypes)
	if err != nil {
		return nil, errors.New("Could not get past fees: " + err.Error())
	}

	budget := int64(totalBudget) - int64(stats.TotalFees)

	return &Budget{
		BudgetInterval: *currentInterval,
		Amount:         budget,
		Total:          totalBudget,
		Stats:          stats,
	}, nil
}
