package autoswap

import (
	"github.com/BoltzExchange/boltz-client/database"
)

type Budget struct {
	database.BudgetInterval
	Amount int64
	Total  uint64
}

/*
func (cfg *Config) GetCurrentBudget() (*Budget, error) {
	budgetInterval, err := cfg.database.QueryCurrentBudgetInterval()
	if err != nil {
		return nil, errors.New("Could not get budget period: " + err.Error())
	}

	now := time.Now()
	if budgetInterval == nil || now.After(budgetInterval.EndDate) {
		budgetDuration := time.Duration(cfg.AutoBudgetInterval) * time.Second
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
		if err := cfg.database.CreateBudget(*budgetInterval); err != nil {
			return nil, errors.New("Could not create budget period: " + err.Error())
		}
	}

	isAuto := true
	stats, err := cfg.database.QueryStats(database.SwapQuery{
		Since:  budgetInterval.StartDate,
		IsAuto: &isAuto,
	})
	if err != nil {
		return nil, errors.New("Could not get past fees: " + err.Error())
	}

	budget := int64(cfg.AutoBudget) - int64(stats.TotalFees)

	return &Budget{
		BudgetInterval: *budgetInterval,
		Amount:       budget,
		Total:        cfg.AutoBudget,
	}, nil
}
*/
