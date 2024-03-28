package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/logger"
)

type swapStatus struct {
	id     string
	status string
}

const latestSchemaVersion = 7

func (database *Database) migrate() error {
	version, err := database.queryVersion()

	if err != nil {
		// Insert the latest schema version when no row is found
		logger.Infof("No database schema version found, inserting latest schema version %d", latestSchemaVersion)
		if err := database.createTables(); err != nil {
			return err
		}

		_, err = database.Exec("INSERT INTO version (version) VALUES (?)", latestSchemaVersion)

		return err
	}

	tx, err := database.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to start transaction for migration: %w", err)
	}

	if err = database.performMigration(tx, version); err != nil {
		return tx.Rollback(err)
	}
	return tx.Commit()
}

func (database *Database) performMigration(tx *Transaction, oldVersion int) error {
	switch oldVersion {
	case 1:
		logMigration(oldVersion)

		logger.Info("Migrating table \"swaps\"")

		_, err := tx.Exec("ALTER TABLE swaps ADD COLUMN state INT")

		if err != nil {
			return err
		}

		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN error VARCHAR")

		if err != nil {
			return err
		}

		swapRows, err := tx.Query("SELECT id, status FROM swaps")

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

			err = tx.UpdateSwapState(&Swap{
				Id: swapToUpdate.id,
			}, newState, "")

			if err != nil {
				return err
			}
		}

		logger.Info("Migrating table \"reverseSwaps\"")

		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN state INT")

		if err != nil {
			return err
		}

		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN error VARCHAR")

		if err != nil {
			return err
		}

		reverseSwapRows, err := tx.Query("SELECT id, status FROM reverseSwaps")

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

			err = tx.UpdateReverseSwapState(&ReverseSwap{
				Id: reverseSwapToUpdate.id,
			}, newState, "")

			if err != nil {
				return err
			}
		}

	case 2:
		logMigration(oldVersion)

		_, err := tx.Exec("ALTER TABLE swaps ADD COLUMN pairId VARCHAR")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN chanIds JSON")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN blindingKey VARCHAR")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN isAuto BOOLEAN DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN serviceFee INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN serviceFeePercent REAL DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN onchainFee INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN createdAt INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN autoSend BOOLEAN DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE swaps ADD COLUMN refundAddress VARCHAR DEFAULT ''")
		if err != nil {
			return err
		}

		_, err = tx.Exec("UPDATE swaps SET pairId = 'BTC/BTC' WHERE pairId IS NULL")
		if err != nil {
			return err
		}

		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN pairId VARCHAR")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN chanIds JSON")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN blindingKey VARCHAR")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN isAuto BOOLEAN DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN routingFeeMsat INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN serviceFee INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN serviceFeePercent REAL DEFAULT 0")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN onchainFee INT")
		if err != nil {
			return err
		}
		_, err = tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN createdAt INT")
		if err != nil {
			return err
		}

		_, err = tx.Exec("UPDATE reverseSwaps SET pairId = 'BTC/BTC' WHERE pairId IS NULL")
		if err != nil {
			return err
		}

		_, err = tx.Exec("CREATE TABLE IF NOT EXISTS autobudget (startDate INT PRIMARY KEY, endDate INT)")
		if err != nil {
			return err
		}

		_, err = tx.Exec("CREATE TABLE IF NOT EXISTS wallets (name VARCHAR PRIMARY KEY, currency VARCHAR, xpub VARCHAR, coreDescriptor VARCHAR, mnemonic VARCHAR, subaccount INT, salt VARCHAR)")
		if err != nil {
			return err
		}

		_, err = tx.Exec("DROP TABLE channelCreations")
		if err != nil {
			return err
		}

		_, err = tx.Exec("UPDATE version SET version = 3")
		if err != nil {
			return err
		}

	case 3:
		logMigration(oldVersion)

		rows, err := tx.Query("SELECT id FROM swaps WHERE state = ?", boltzrpc.SwapState_PENDING)
		if err != nil {
			return err
		}
		if rows.Next() {
			return errors.New("database migration failed: found pending swaps")
		}

		rows, err = tx.Query("SELECT id FROM reverseSwaps WHERE state = ?", boltzrpc.SwapState_PENDING)
		if err != nil {
			return err
		}
		if rows.Next() {
			return errors.New("database migration failed: found pending reverse swaps")
		}

		var migration = `
		ALTER TABLE swaps ADD COLUMN swapTree JSON;
		ALTER TABLE swaps ADD COLUMN claimPubKey VARCHAR;
		ALTER TABLE swaps ADD COLUMN fromCurrency VARCHAR;
		ALTER TABLE swaps ADD COLUMN toCurrency VARCHAR;

		ALTER TABLE reverseSwaps ADD COLUMN swapTree JSON;
		ALTER TABLE reverseSwaps ADD COLUMN refundPubKey VARCHAR;
		ALTER TABLE reverseSwaps ADD COLUMN fromCurrency VARCHAR;
		ALTER TABLE reverseSwaps ADD COLUMN toCurrency VARCHAR;
		`
		if _, err := tx.Exec(migration); err != nil {
			return err
		}

		updatePairs := func(table string) error {
			rows, err = tx.Query("SELECT id, pairId FROM " + table)
			if err != nil {
				return err
			}
			var ids, pairs []string
			for rows.Next() {
				var id, pair string
				if err := rows.Scan(&id, &pair); err != nil {
					return err
				}
				ids = append(ids, id)
				pairs = append(pairs, pair)
			}
			rows.Close()
			for i, id := range ids {
				split := strings.Split(pairs[i], "/")
				from := split[0]
				to := split[1]
				if table == "reverseSwaps" {
					to = split[0]
					from = split[1]
				}
				if _, err := tx.Exec(fmt.Sprintf("UPDATE %s SET fromCurrency = ?, toCurrency = ? WHERE id = ?", table), from, to, id); err != nil {
					return err
				}
			}
			return nil
		}
		if err := updatePairs("swaps"); err != nil {
			return err
		}
		if err := updatePairs("reverseSwaps"); err != nil {
			return err
		}

		migration = `
		ALTER TABLE swaps DROP COLUMN pairId;
		ALTER TABLE reverseSwaps DROP COLUMN pairId;
		`
		if _, err := tx.Exec(migration); err != nil {
			return err
		}
	case 4:
		logMigration(oldVersion)

		if _, err := tx.Exec("ALTER TABLE swaps ADD COLUMN wallet VARCHAR"); err != nil {
			return err
		}
		if _, err := tx.Exec("ALTER TABLE swaps DROP COLUMN autoSend"); err != nil {
			return err
		}
	case 5:
		logMigration(oldVersion)

		if _, err := tx.Exec("ALTER TABLE reverseSwaps ADD COLUMN externalPay BOOLEAN"); err != nil {
			return err
		}
	case 6:
		logMigration(oldVersion)

		migration := `
		CREATE TABLE entities
		(
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR UNIQUE
		);
		ALTER TABLE wallets RENAME TO old_wallets;
		CREATE TABLE wallets
		(
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           VARCHAR,
			currency       VARCHAR,
			nodePubkey     VARCHAR,
			xpub           VARCHAR,
			coreDescriptor VARCHAR,
			mnemonic       VARCHAR,
			subaccount     INT,
			salt           VARCHAR,
			entityId       INT REFERENCES entities (id),

			UNIQUE (name, entityId, nodePubkey),
			UNIQUE (xpub, coreDescriptor, mnemonic, nodePubkey)
		);
		INSERT INTO wallets (name, currency, xpub, coreDescriptor, mnemonic, subaccount, salt)
		SELECT name, currency,  xpub, coreDescriptor, mnemonic, subaccount, salt FROM old_wallets;
		DROP TABLE old_wallets;
		ALTER TABLE swaps ADD COLUMN walletId INT REFERENCES wallets(id) ON DELETE SET NULL;
		ALTER TABLE swaps ADD COLUMN entityId INT REFERENCES entities(id);
		ALTER TABLE reverseSwaps ADD COLUMN walletId INT REFERENCES wallets(id) ON DELETE SET NULL;
		ALTER TABLE reverseSwaps ADD COLUMN entityId INT REFERENCES entities(id);
		ALTER TABLE swaps DROP COLUMN wallet;
		`

		if _, err := tx.Exec(migration); err != nil {
			return err
		}
	case latestSchemaVersion:
		logger.Info("database already at latest schema version: " + strconv.Itoa(latestSchemaVersion))
		return nil

	default:
		return errors.New("found unexpected database schema version: " + strconv.Itoa(oldVersion))
	}

	newVersion := oldVersion + 1

	if _, err := tx.Exec("UPDATE version SET version = ?", newVersion); err != nil {
		return err
	}

	logger.Infof("Update to database version %d completed", newVersion)

	if oldVersion+1 < latestSchemaVersion {
		logger.Info("Running migration again")
		return database.performMigration(tx, newVersion)
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

func logMigration(oldVersion int) {
	logger.Infof("Updating database from version %d to %d", oldVersion, oldVersion+1)
}
