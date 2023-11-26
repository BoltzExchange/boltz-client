package database

import (
	"database/sql"
	"encoding/hex"
	"golang.org/x/exp/constraints"
	"time"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/btcsuite/btcd/btcec/v2"
	_ "github.com/mattn/go-sqlite3"
)

// TODO: prepare insert statements only once

type Database struct {
	Path string `long:"database.path" description:"Path to the database file"`

	db *sql.DB
}

func (database *Database) Connect() error {
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

func (database *Database) createTables() error {
	_, err := database.db.Exec("CREATE TABLE IF NOT EXISTS version (version INT)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS macaroons (id VARCHAR PRIMARY KEY, rootKey VARCHAR)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS swaps (id VARCHAR PRIMARY KEY, pairId VARCHAR, chanId INT, state INT, error VARCHAR, status VARCHAR, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, address VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, refundTransactionId VARCHAR, refundAddress VARCHAR, blindingKey VARCHAR, isAuto BOOLEAN, serviceFee INT, serviceFeePercent REAL, onchainFee INT, createdAt INT, autoSend BOOLEAN)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS reverseSwaps (id VARCHAR PRIMARY KEY, pairId VARCHAR, chanId INT, state INT, error VARCHAR, status VARCHAR, acceptZeroConf BOOLEAN, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, claimAddress VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, claimTransactionId VARCHAR, blindingKey VARCHAR, isAuto BOOLEAN, routingFeeMsat INT, serviceFee INT, serviceFeePercent REAL, onchainFee INT, createdAt INT)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS channelCreations (swapId VARCHAR PRIMARY KEY, status VARCHAR, inboundLiquidity INT, private BOOLEAN, fundingTransactionId VARCHAR, fundingTransactionVout INT)")
	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS autobudget (startDate INTEGER PRIMARY KEY, endDate INTEGER)")

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
