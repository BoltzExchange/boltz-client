package database

import (
	"database/sql"
	"encoding/hex"
	"github.com/BoltzExchange/boltz-lnd/logger"
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

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS swaps (id VARCHAR PRIMARY KEY, state INT, error VARCHAR, status VARCHAR, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, address VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, refundTransactionId VARCHAR)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS reverseSwaps (id VARCHAR PRIMARY KEY, state INT, error VARCHAR, status VARCHAR, acceptZeroConf BOOLEAN, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, claimAddress VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, claimTransactionId VARCHAR)")

	if err != nil {
		return err
	}

	_, err = database.db.Exec("CREATE TABLE IF NOT EXISTS channelCreations (swapId VARCHAR PRIMARY KEY, status VARCHAR, inboundLiquidity INT, private BOOLEAN, fundingTransactionId VARCHAR, fundingTransactionVout INT)")

	return err
}

func parsePrivateKey(privateKeyBytes []byte) (*btcec.PrivateKey, *btcec.PublicKey) {
	return btcec.PrivKeyFromBytes(privateKeyBytes)
}

func formatPrivateKey(key *btcec.PrivateKey) string {
	return hex.EncodeToString(key.Serialize())
}
