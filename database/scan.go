package database

import (
	"database/sql"
)

// TODO: optimize (query columns only once per query (improves speed by like a microsecond per row so low priority))
func scanRow(row *sql.Rows, rowValues map[string]interface{}) error {
	columns, err := row.Columns()
	if err != nil {
		return err
	}

	var values []interface{}

	for _, column := range columns {
		values = append(values, rowValues[column])
	}

	return row.Scan(values...)
}
