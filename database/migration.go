package database

import (
	"database/sql"
	"errors"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"strconv"
)

const latestSchemaVersion = 1

func (database *Database) migrate() error {
	version, err := database.queryVersion()

	if err != nil {
		// Insert the latest schema version when no row is found
		if err == sql.ErrNoRows {
			logger.Info("No database schema version found")
			logger.Info("Inserting latest database schema version: " + strconv.Itoa(latestSchemaVersion))

			_, err = database.db.Exec("INSERT INTO version (version) VALUES (?)", latestSchemaVersion)

			return err
		} else {
			return err
		}
	}

	switch version {
	case latestSchemaVersion:
		logger.Info("Database already at latest schema version: " + strconv.Itoa(latestSchemaVersion))

	default:
		err = errors.New("found unexpected database schema version: " + strconv.Itoa(version))
	}

	return err
}

func (database *Database) queryVersion() (int, error) {
	row := database.db.QueryRow("SELECT * FROM version")

	var version int

	err := row.Scan(
		&version,
	)

	return version, err
}
