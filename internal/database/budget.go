package database

import (
	"database/sql"
	"errors"
	"time"
)

type BudgetInterval struct {
	StartDate time.Time
	EndDate   time.Time
	Name      string
	TenantId  Id
}

func (database *Database) QueryCurrentBudgetInterval(name string, tenantId Id) (*BudgetInterval, error) {
	row := database.QueryRow("SELECT * FROM autobudget WHERE name = ? AND tenantId = ? ORDER BY startDate DESC LIMIT 1", name, tenantId)

	var period BudgetInterval
	var startDate, endDate int64

	err := row.Scan(&startDate, &endDate, &period.Name, &period.TenantId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	period.StartDate = parseTime(startDate)
	period.EndDate = parseTime(endDate)

	return &period, nil
}

func (database *Database) CreateBudget(period BudgetInterval) error {
	insertStatement := "INSERT INTO autobudget (startDate, endDate, name, tenantId) VALUES (?, ?, ?, ?)"
	_, err := database.Exec(
		insertStatement,
		FormatTime(period.StartDate), FormatTime(period.EndDate), period.Name, period.TenantId,
	)
	return err
}
