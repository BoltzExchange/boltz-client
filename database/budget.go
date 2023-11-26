package database

import (
	"time"
)

type BudgetPeriod struct {
	StartDate time.Time
	EndDate   time.Time
}

func (database *Database) QueryCurrentBudgetPeriod() (*BudgetPeriod, error) {
	rows, err := database.db.Query("SELECT * FROM autobudget ORDER BY startDate DESC LIMIT 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var period BudgetPeriod
	var startDate, endDate int64

	if rows.Next() {
		err = rows.Scan(&startDate, &endDate)

		if err == nil {
			period.StartDate = ParseTime(startDate)
			period.EndDate = ParseTime(endDate)
		}
	} else {
		return nil, nil
	}

	return &period, err
}

func (database *Database) CreateBudget(period BudgetPeriod) error {
	insertStatement := "INSERT INTO autobudget (startDate, endDate) VALUES (?, ?)"
	_, err := database.db.Exec(insertStatement, FormatTime(period.StartDate), FormatTime(period.EndDate))
	return err
}
