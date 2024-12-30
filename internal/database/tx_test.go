package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction(t *testing.T) {
	db := Database{Path: ":memory:"}
	err := db.Connect()
	assert.Nil(t, err)

	statements := func(t *testing.T, tx *Transaction) {
		_, err = tx.Exec("CREATE TABLE test (id INTEGER)")
		assert.Nil(t, err)

		_, err = tx.Exec("INSERT INTO test (id) VALUES (1)")
		assert.Nil(t, err)

		row, err := tx.Query("SELECT * FROM test")
		assert.Nil(t, err)
		assert.True(t, row.Next())
	}

	t.Run("Rollback", func(t *testing.T) {
		tx, err := db.BeginTx()
		require.NoError(t, err)
		statements(t, tx)

		assert.NoError(t, tx.Rollback(nil))
		row := db.QueryRow("SELECT * FROM test")
		assert.Error(t, row.Err())
	})

	t.Run("Commit", func(t *testing.T) {
		tx, err := db.BeginTx()
		require.NoError(t, err)

		statements(t, tx)
		row := db.QueryRow("SELECT * FROM test")
		assert.Error(t, row.Err())

		assert.NoError(t, tx.Commit())
		row = db.QueryRow("SELECT * FROM test")
		assert.NoError(t, row.Err())

	})
}
