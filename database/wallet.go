package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/BoltzExchange/boltz-client/onchain/wallet"
)

func (d *Database) InsertWalletCredentials(credentials *wallet.Credentials) error {
	query := "INSERT INTO wallets (name, currency, xpub, coreDescriptor, mnemonic, subaccount, salt) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := d.Exec(
		query,
		credentials.Name,
		credentials.Currency,
		credentials.Xpub,
		credentials.CoreDescriptor,
		credentials.Mnemonic,
		credentials.Subaccount,
		credentials.Salt,
	)
	return err
}

func (d *Database) UpdateWalletCredentials(credentials *wallet.Credentials) error {
	query := "UPDATE wallets SET currency = ?, xpub = ?, coreDescriptor = ?, mnemonic = ?, subaccount = ?, salt = ? WHERE name = ?"
	_, err := d.Exec(
		query,
		credentials.Currency,
		credentials.Xpub,
		credentials.CoreDescriptor,
		credentials.Mnemonic,
		credentials.Subaccount,
		credentials.Salt,
		credentials.Name,
	)
	return err
}

func parseWalletCredentials(rows *sql.Rows) (*wallet.Credentials, error) {
	credentials := &wallet.Credentials{}
	err := rows.Scan(
		&credentials.Name,
		&credentials.Currency,
		&credentials.Xpub,
		&credentials.CoreDescriptor,
		&credentials.Mnemonic,
		&credentials.Subaccount,
		&credentials.Salt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wallet credentials: %w", err)
	}
	return credentials, nil
}

func (d *Database) GetWalletCredentials(name string) (*wallet.Credentials, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets WHERE name = ?"
	rows, err := d.Query(query, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return parseWalletCredentials(rows)
	}
	return nil, errors.New("not found")
}

func (d *Database) QueryWalletCredentials() ([]*wallet.Credentials, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets"
	rows, err := d.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()

	var credentials []*wallet.Credentials
	for rows.Next() {
		credential, err := parseWalletCredentials(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}

	return credentials, nil
}

func (d *Database) DeleteWalletCredentials(name string) error {
	query := "DELETE FROM wallets WHERE name = ?"
	_, err := d.Exec(query, name)
	return err
}

func (d *Database) SetWalletSubaccount(name string, currency string, subaccount uint64) error {
	query := "UPDATE wallets SET subaccount = ? WHERE name = ? AND currency = ?"
	_, err := d.Exec(query, subaccount, name, currency)
	return err
}
