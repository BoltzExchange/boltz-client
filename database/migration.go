package database

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
)

type swapStatus struct {
	id     string
	status string
}

const latestSchemaVersion = 3

func (database *Database) migrate() error {
	version, err := database.queryVersion()

	if err != nil {
		// Insert the latest schema version when no row is found
		if err == sql.ErrNoRows {
			logger.Info("No database schema version found")
			logger.Info("Inserting latest database schema version: " + strconv.Itoa(latestSchemaVersion))

			_, err = database.Exec("INSERT INTO version (version) VALUES (?)", latestSchemaVersion)

			return err
		} else {
			return err
		}
	}

	return database.performMigration(version)
}

func (database *Database) performMigration(fromVersion int) error {
	switch fromVersion {
	case 1:
		logger.Info("Updating database from version 1 to 2")

		logger.Info("Migrating table \"swaps\"")

		_, err := database.Exec("ALTER TABLE swaps ADD COLUMN state INT")

		if err != nil {
			return err
		}

		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN error VARCHAR")

		if err != nil {
			return err
		}

		swapRows, err := database.Query("SELECT id, status FROM swaps")

		if err != nil {
			return err
		}

		var swapsToUpdate []swapStatus

		for swapRows.Next() {
			swapToUpdate := swapStatus{}

			err = swapRows.Scan(
				&swapToUpdate.id,
				&swapToUpdate.status,
			)

			if err != nil {
				return err
			}

			swapsToUpdate = append(swapsToUpdate, swapToUpdate)
		}

		if err = swapRows.Close(); err != nil {
			return err
		}

		for _, swapToUpdate := range swapsToUpdate {
			status := boltz.ParseEvent(swapToUpdate.status)

			var newState boltzrpc.SwapState

			if status.IsCompletedStatus() {
				newState = boltzrpc.SwapState_SUCCESSFUL
			} else if status.IsFailedStatus() {
				newState = boltzrpc.SwapState_SERVER_ERROR
			} else {
				// Handle deprecated events
				switch swapToUpdate.status {
				case "swap.refunded":
					newState = boltzrpc.SwapState_REFUNDED

				case "swap.abandoned":
					newState = boltzrpc.SwapState_ABANDONED

				default:
					newState = boltzrpc.SwapState_PENDING
				}
			}

			err = database.UpdateSwapState(&Swap{
				Id: swapToUpdate.id,
			}, newState, "")

			if err != nil {
				return err
			}
		}

		logger.Info("Migrating table \"reverseSwaps\"")

		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN state INT")

		if err != nil {
			return err
		}

		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN error VARCHAR")

		if err != nil {
			return err
		}

		reverseSwapRows, err := database.Query("SELECT id, status FROM reverseSwaps")

		if err != nil {
			return err
		}

		var reverseSwapsToUpdate []swapStatus

		for reverseSwapRows.Next() {
			reverseSwapToUpdate := swapStatus{}

			err = reverseSwapRows.Scan(
				&reverseSwapToUpdate.id,
				&reverseSwapToUpdate.status,
			)

			if err != nil {
				return err
			}

			reverseSwapsToUpdate = append(reverseSwapsToUpdate, reverseSwapToUpdate)
		}

		if err = swapRows.Close(); err != nil {
			return err
		}

		for _, reverseSwapToUpdate := range reverseSwapsToUpdate {
			status := boltz.ParseEvent(reverseSwapToUpdate.status)

			var newState boltzrpc.SwapState

			if status.IsCompletedStatus() {
				newState = boltzrpc.SwapState_SUCCESSFUL
			} else if status.IsFailedStatus() {
				newState = boltzrpc.SwapState_SERVER_ERROR
			} else {
				newState = boltzrpc.SwapState_PENDING
			}

			err = database.UpdateReverseSwapState(&ReverseSwap{
				Id: reverseSwapToUpdate.id,
			}, newState, "")

			if err != nil {
				return err
			}
		}

		_, err = database.Exec("UPDATE version SET version = 2 WHERE version = 1")
		if err != nil {
			return err
		}

		logger.Info("Update to database version 2 completed")
		return database.postMigration(fromVersion)

	case 2:
		logger.Info("Updating database from version 2 to 3")

		_, err := database.Exec("ALTER TABLE swaps ADD COLUMN pairId VARCHAR")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN chanId INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN blindingKey VARCHAR")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN isAuto BOOLEAN")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN serviceFee INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN serviceFeePercent REAL")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN onchainFee INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN createdAt INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN autoSend BOOLEAN")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE swaps ADD COLUMN refundAddress VARCHAR")
		if err != nil {
			return err
		}

		_, err = database.Exec("UPDATE swaps SET pairId = 'BTC/BTC' WHERE pairId IS NULL")
		if err != nil {
			return err
		}

		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN pairId VARCHAR")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN chanId INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN blindingKey VARCHAR")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN isAuto BOOLEAN")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN routingFeeMsat INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN serviceFee INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN serviceFeePercent REAL")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN onchainFee INT")
		if err != nil {
			return err
		}
		_, err = database.Exec("ALTER TABLE reverseSwaps ADD COLUMN createdAt INT")
		if err != nil {
			return err
		}

		_, err = database.Exec("UPDATE reverseSwaps SET pairId = 'BTC/BTC' WHERE pairId IS NULL")
		if err != nil {
			return err
		}

		_, err = database.Exec("CREATE TABLE IF NOT EXISTS autobudget (startDate INT PRIMARY KEY, endDate INT)")
		if err != nil {
			return err
		}

		_, err = database.Exec("CREATE TABLE IF NOT EXISTS wallets (name VARCHAR PRIMARY KEY, currency VARCHAR, xpub VARCHAR, coreDescriptor VARCHAR, mnemonic VARCHAR, subaccount INT, salt VARCHAR)")
		if err != nil {
			return err
		}

		_, err = database.Exec("DROP TABLE channelCreations")
		if err != nil {
			return err
		}

	case latestSchemaVersion:
		logger.Info("Database already at latest schema version: " + strconv.Itoa(latestSchemaVersion))

	default:
		return errors.New("found unexpected database schema version: " + strconv.Itoa(fromVersion))
	}

	return nil
}

func (database *Database) postMigration(fromVersion int) error {
	if fromVersion+1 < latestSchemaVersion {
		logger.Info("Running migration again")
		return database.migrate()
	}

	return nil
}

func (database *Database) queryVersion() (int, error) {
	row := database.QueryRow("SELECT version FROM version")

	var version int

	err := row.Scan(
		&version,
	)

	return version, err
}
