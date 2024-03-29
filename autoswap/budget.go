package autoswap

import (
	"errors"
	"time"

	"github.com/BoltzExchange/boltz-client/database"
)

type Budget struct {
	database.BudgetInterval
	Amount int64
	Total  uint64
}

func (swapper *LightningSwapper) GetCurrentBudget(createIfMissing bool) (*Budget, error) {
	budgetInterval, err := swapper.database.QueryCurrentBudgetInterval()
	if err != nil {
		return nil, errors.New("Could not get budget period: " + err.Error())
	}

	now := time.Now()
	if budgetInterval == nil || now.After(budgetInterval.EndDate) {
		if createIfMissing {
			budgetDuration := time.Duration(swapper.cfg.BudgetInterval) * time.Second
			if budgetInterval == nil {
				budgetInterval = &database.BudgetInterval{
					StartDate: now,
					EndDate:   now.Add(budgetDuration),
				}
			}
			for now.After(budgetInterval.EndDate) {
				budgetInterval.StartDate = budgetInterval.EndDate
				budgetInterval.EndDate = budgetInterval.EndDate.Add(budgetDuration)
			}
			if err := swapper.database.CreateBudget(*budgetInterval); err != nil {
				return nil, errors.New("Could not create budget period: " + err.Error())
			}
		} else {
			return nil, nil
		}
	}

	isAuto := true
	stats, err := swapper.database.QueryStats(database.SwapQuery{
		Since:  budgetInterval.StartDate,
		IsAuto: &isAuto,
	}, false)
	if err != nil {
		return nil, errors.New("Could not get past fees: " + err.Error())
	}

	budget := int64(swapper.cfg.Budget) - int64(stats.TotalFees)

	return &Budget{
		BudgetInterval: *budgetInterval,
		Amount:         budget,
		Total:          swapper.cfg.Budget,
	}, nil
}
