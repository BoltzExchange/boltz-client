package database

import (
	"database/sql"
	"time"
)

type BudgetInterval struct {
	StartDate time.Time
	EndDate   time.Time
}

func (database *Database) QueryCurrentBudgetInterval() (*BudgetInterval, error) {
	row := database.QueryRow("SELECT * FROM autobudget ORDER BY startDate DESC LIMIT 1")

	var period BudgetInterval
	var startDate, endDate int64

	err := row.Scan(&startDate, &endDate)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	period.StartDate = ParseTime(startDate)
	period.EndDate = ParseTime(endDate)
	return &period, nil
}

func (database *Database) CreateBudget(period BudgetInterval) error {
	insertStatement := "INSERT INTO autobudget (startDate, endDate) VALUES (?, ?)"
	_, err := database.Exec(insertStatement, FormatTime(period.StartDate), FormatTime(period.EndDate))
	return err
}
