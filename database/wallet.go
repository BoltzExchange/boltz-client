package database

import (
	"database/sql"
	"fmt"

	onchainWallet "github.com/BoltzExchange/boltz-client/onchain/wallet"
)

type Wallet struct {
	*onchainWallet.Credentials
	NodePubkey *string
	EntityId   *int64
}

func (d *Database) CreateWallet(wallet *Wallet) error {
	query := "INSERT INTO wallets (name, currency, xpub, coreDescriptor, mnemonic, subaccount, salt, entityId, nodePubkey) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id"
	row := d.QueryRow(
		query,
		wallet.Name,
		wallet.Currency,
		wallet.Xpub,
		wallet.CoreDescriptor,
		wallet.Mnemonic,
		wallet.Subaccount,
		wallet.Salt,
		wallet.EntityId,
		wallet.NodePubkey,
	)
	return row.Scan(&wallet.Id)
}

func (d *Database) UpdateWalletCredentials(credentials *onchainWallet.Credentials) error {
	query := "UPDATE wallets SET currency = ?, xpub = ?, coreDescriptor = ?, mnemonic = ?, subaccount = ?, salt = ? WHERE id = ?"
	_, err := d.Exec(
		query,
		credentials.Currency,
		credentials.Xpub,
		credentials.CoreDescriptor,
		credentials.Mnemonic,
		credentials.Subaccount,
		credentials.Salt,
		credentials.Id,
	)
	return err
}

func parseWallet(rows *sql.Rows) (*Wallet, error) {
	wallet := &Wallet{Credentials: &onchainWallet.Credentials{}}
	err := rows.Scan(
		&wallet.Id,
		&wallet.Name,
		&wallet.Currency,
		&wallet.NodePubkey,
		&wallet.Xpub,
		&wallet.CoreDescriptor,
		&wallet.Mnemonic,
		&wallet.Subaccount,
		&wallet.Salt,
		&wallet.EntityId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wallet wallet: %w", err)
	}
	return wallet, nil
}

func (d *Database) GetWalletByName(name string, entity *int64) (*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets WHERE name = ?"
	args := []any{name}
	if entity != nil {
		query = "SELECT * FROM wallets WHERE name = ? AND (entityId = ?)"
		args = append(args, entity)
	}
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}

	defer rows.Close()
	if rows.Next() {
		return parseWallet(rows)
	}
	return nil, fmt.Errorf("wallet with name %s not found for entity %v", name, entity)
}

func (d *Database) GetNodeWallet(nodePubkey string) (*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets WHERE nodePubkey = ?"
	rows, err := d.Query(query, nodePubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return parseWallet(rows)
	}
	return nil, fmt.Errorf("walle with nodePubkey %s not found", nodePubkey)
}

func (d *Database) QueryWalletCredentials() ([]*onchainWallet.Credentials, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets"
	rows, err := d.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer rows.Close()

	var credentials []*onchainWallet.Credentials
	for rows.Next() {
		wallet, err := parseWallet(rows)
		if err != nil {
			return nil, err
		}
		if wallet.NodePubkey == nil {
			credentials = append(credentials, wallet.Credentials)
		}
	}

	return credentials, nil
}

func (d *Database) DeleteWallet(id int64) error {
	query := "DELETE FROM wallets WHERE id = ?"
	result, err := d.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("failed to delete wallet with id %d", id)
	}
	return nil
}

func (d *Database) SetWalletSubaccount(id int64, subaccount uint64) error {
	query := "UPDATE wallets SET subaccount = ? WHERE id = ?"
	_, err := d.Exec(query, subaccount, id)
	return err
}
