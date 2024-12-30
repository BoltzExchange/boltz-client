package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestScanRow(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.Nil(t, err)

	_, err = db.Exec("CREATE TABLE test (id VARCHAR, test VARCHAR)")
	assert.Nil(t, err)

	_, err = db.Exec("INSERT INTO test (id, test) VALUES (\"some\", \"values\")")
	assert.Nil(t, err)

	row, err := db.Query("SELECT * FROM test")
	assert.Nil(t, err)
	assert.True(t, row.Next())

	var idValue string
	var testValue string

	err = scanRow(row, map[string]interface{}{
		"id":   &idValue,
		"test": &testValue,
	})
	assert.Nil(t, err)

	assert.Equal(t, "some", idValue)
	assert.Equal(t, "values", testValue)

	assert.Nil(t, db.Close())
}
