package autoswap

import (
	"github.com/BoltzExchange/boltz-client/database"
)

type Budget struct {
	database.BudgetPeriod
	Amount int64
	Total  uint64
}

/*
func (cfg *Config) GetCurrentBudget() (*Budget, error) {
	budgetPeriod, err := cfg.database.QueryCurrentBudgetPeriod()
	if err != nil {
		return nil, errors.New("Could not get budget period: " + err.Error())
	}

	now := time.Now()
	if budgetPeriod == nil || now.After(budgetPeriod.EndDate) {
		budgetDuration := time.Duration(cfg.AutoBudgetPeriod) * time.Second
		if budgetPeriod == nil {
			budgetPeriod = &database.BudgetPeriod{
				StartDate: now,
				EndDate:   now.Add(budgetDuration),
			}
		}
		for now.After(budgetPeriod.EndDate) {
			budgetPeriod.StartDate = budgetPeriod.EndDate
			budgetPeriod.EndDate = budgetPeriod.EndDate.Add(budgetDuration)
		}
		if err := cfg.database.CreateBudget(*budgetPeriod); err != nil {
			return nil, errors.New("Could not create budget period: " + err.Error())
		}
	}

	isAuto := true
	stats, err := cfg.database.QueryStats(database.SwapQuery{
		Since:  budgetPeriod.StartDate,
		IsAuto: &isAuto,
	})
	if err != nil {
		return nil, errors.New("Could not get past fees: " + err.Error())
	}

	budget := int64(cfg.AutoBudget) - int64(stats.TotalFees)

	return &Budget{
		BudgetPeriod: *budgetPeriod,
		Amount:       budget,
		Total:        cfg.AutoBudget,
	}, nil
}
*/
