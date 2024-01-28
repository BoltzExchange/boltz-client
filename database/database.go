package database

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"golang.org/x/exp/constraints"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/btcsuite/btcd/btcec/v2"
	_ "github.com/mattn/go-sqlite3"
)

// TODO: prepare insert statements only once

type Database struct {
	Path string `long:"database.path" description:"Path to the database file"`

	db *sql.DB
	tx *sql.Tx
}

type Transaction struct {
	Database
}

type JsonScanner[T any] struct {
	Value T
}

func (j *JsonScanner[T]) Scan(src any) error {
	if str, ok := src.(string); ok {
		return json.Unmarshal([]byte(str), &j.Value)
	}
	return fmt.Errorf("unsupported type: %T", src)
}

func (database *Database) BeginTx() (*Transaction, error) {
	tx, err := database.db.Begin()
	if err != nil {
		return nil, err
	}
	return &Transaction{
		Database{tx: tx},
	}, nil
}

func (transaction *Transaction) Commit() error {
	return transaction.tx.Commit()
}

func (transaction *Transaction) Rollback(cause error) error {
	if err := transaction.tx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback: %w: %w", err, cause)
	}
	return cause
}

type SwapQuery struct {
	Pair   *boltz.Pair
	State  *boltzrpc.SwapState
	IsAuto *bool
	Since  time.Time
}

func (query *SwapQuery) ToWhereClause() (where string, values []any) {
	var conditions []string
	if query.Pair != nil {
		conditions = append(conditions, "pairId = ?")
		values = append(values, *query.Pair)
	}
	if query.State != nil {
		conditions = append(conditions, "state = ?")
		values = append(values, *query.State)
	}
	if query.IsAuto != nil {
		conditions = append(conditions, "isAuto = ?")
		values = append(values, *query.IsAuto)
	}
	if !query.Since.IsZero() {
		conditions = append(conditions, "createdAt >= ?")
		values = append(values, query.Since.Unix())
	}
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	return
}

func (database *Database) Connect() error {
	if database.db == nil {
		logger.Info("Opening database: " + database.Path)
		db, err := sql.Open("sqlite3", database.Path)

		if err != nil {
			return err
		}

		database.db = db

		err = database.createTables()

		if err != nil {
			return err
		}

		return database.migrate()
	}
	return nil
}

func (database *Database) Exec(query string, args ...any) (sql.Result, error) {
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.Exec(query, args...)
	}
	return database.db.Exec(query, args...)
}

func (database *Database) Query(query string, args ...any) (*sql.Rows, error) {
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.Query(query, args...)
	}
	return database.db.Query(query, args...)
}

func (database *Database) QueryRow(query string, args ...any) *sql.Row {
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.QueryRow(query, args...)
	}
	return database.db.QueryRow(query, args...)
}

func (database *Database) createTables() error {
	_, err := database.Exec("CREATE TABLE IF NOT EXISTS version (version INT)")

	if err != nil {
		return err
	}

	_, err = database.Exec("CREATE TABLE IF NOT EXISTS macaroons (id VARCHAR PRIMARY KEY, rootKey VARCHAR)")

	if err != nil {
		return err
	}

	_, err = database.Exec("CREATE TABLE IF NOT EXISTS swaps (id VARCHAR PRIMARY KEY, pairId VARCHAR, chanIds JSON, state INT, error VARCHAR, status VARCHAR, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, address VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, refundTransactionId VARCHAR, refundAddress VARCHAR, blindingKey VARCHAR, isAuto BOOLEAN, serviceFee INT, serviceFeePercent REAL, onchainFee INT, createdAt INT, autoSend BOOLEAN)")

	if err != nil {
		return err
	}

	_, err = database.Exec("CREATE TABLE IF NOT EXISTS reverseSwaps (id VARCHAR PRIMARY KEY, pairId VARCHAR, chanIds JSON, state INT, error VARCHAR, status VARCHAR, acceptZeroConf BOOLEAN, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, claimAddress VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, claimTransactionId VARCHAR, blindingKey VARCHAR, isAuto BOOLEAN, routingFeeMsat INT, serviceFee INT, serviceFeePercent REAL, onchainFee INT, createdAt INT)")

	if err != nil {
		return err
	}

	_, err = database.Exec("CREATE TABLE IF NOT EXISTS autobudget (startDate INTEGER PRIMARY KEY, endDate INTEGER)")
	if err != nil {
		return err
	}

	// create table for wallet credentials
	_, err = database.Exec("CREATE TABLE IF NOT EXISTS wallets (name VARCHAR PRIMARY KEY, currency VARCHAR, xpub VARCHAR, coreDescriptor VARCHAR, mnemonic VARCHAR, subaccount INT, salt VARCHAR)")

	return err
}

func ParsePrivateKey(privateKeyHex string) (*btcec.PrivateKey, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, err
	}
	priv, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	return priv, nil
}

func formatPrivateKey(key *btcec.PrivateKey) string {
	if key == nil {
		return ""
	}
	return hex.EncodeToString(key.Serialize())
}

func ParseTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

func FormatTime(t time.Time) int64 {
	if t.IsZero() {
		return time.Now().Unix()
	}
	return t.Unix()
}

func parseNullInt(value sql.NullInt64) *uint64 {
	if value.Valid {
		value := uint64(value.Int64)
		return &value
	}
	return nil
}

func addToOptional[V constraints.Integer](value *V, add V) *V {
	if value != nil {
		*value += add
	} else {
		value = &add
	}
	return value
}

func formatJson(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		logger.Errorf("Could not marshal json value %v: %v", value, err)
	}
	return string(encoded)
}
