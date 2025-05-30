package database

import (
	"fmt"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
)

type Wallet struct {
	*onchain.WalletCredentials
	NodePubkey *string
}

func (d *Database) CreateWallet(wallet *Wallet) error {
	query := "INSERT INTO wallets (name, currency, xpub, coreDescriptor, mnemonic, subaccount, salt, tenantId, nodePubkey) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id"
	row := d.QueryRow(
		query,
		wallet.Name,
		wallet.Currency,
		wallet.Xpub,
		wallet.CoreDescriptor,
		wallet.Mnemonic,
		wallet.Subaccount,
		wallet.Salt,
		wallet.TenantId,
		wallet.NodePubkey,
	)
	return row.Scan(&wallet.Id)
}

func (d *Database) UpdateWalletCredentials(credentials *onchain.WalletCredentials) error {
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

func parseWallet(rows row) (*Wallet, error) {
	wallet := &Wallet{WalletCredentials: &onchain.WalletCredentials{}}
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
		&wallet.TenantId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wallet wallet: %w", err)
	}
	return wallet, nil
}

func (d *Database) GetWallet(id Id) (*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	row := d.QueryRow("SELECT * FROM wallets WHERE id = ?", id)
	return parseWallet(row)
}

func (d *Database) GetWalletByName(name string, tenant Id) (*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	row := d.QueryRow("SELECT * FROM wallets WHERE name = ? AND tenantId = ?", name, tenant)
	return parseWallet(row)
}

func (d *Database) GetNodeWallet(nodePubkey string) (*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets WHERE nodePubkey = ?"
	rows, err := d.Query(query, nodePubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer closeRows(rows)
	if rows.Next() {
		return parseWallet(rows)
	}
	return nil, fmt.Errorf("walle with nodePubkey %s not found", nodePubkey)
}

func (d *Database) QueryWalletCredentials() ([]*onchain.WalletCredentials, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets"
	rows, err := d.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer closeRows(rows)

	var credentials []*onchain.WalletCredentials
	for rows.Next() {
		wallet, err := parseWallet(rows)
		if err != nil {
			return nil, err
		}
		if wallet.NodePubkey == nil {
			credentials = append(credentials, wallet.WalletCredentials)
		}
	}

	return credentials, nil
}

func (d *Database) DeleteWallet(id Id) error {
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

func (d *Database) SetWalletSubaccount(id Id, subaccount uint64) error {
	query := "UPDATE wallets SET subaccount = ? WHERE id = ?"
	_, err := d.Exec(query, subaccount, id)
	return err
}
