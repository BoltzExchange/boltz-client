package database

import (
	"database/sql"
	"fmt"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
)

// WalletPersister implements the liquid_wallet.Persister interface using the database
type WalletPersister struct {
	db *Database
}

// NewWalletPersister creates a new database persister
func NewWalletPersister(db *Database) *WalletPersister {
	return &WalletPersister{db: db}
}

func (p *WalletPersister) LoadLastIndex(walletId uint64) (*uint32, error) {
	var lastIndex sql.NullInt64
	query := "SELECT lastIndex FROM wallets WHERE id = ?"
	err := p.db.QueryRow(query, walletId).Scan(&lastIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to query lastIndex: %w", err)
	}
	if !lastIndex.Valid {
		return nil, nil
	}
	idx := uint32(lastIndex.Int64)
	return &idx, nil
}

func (p *WalletPersister) PersistLastIndex(walletId uint64, index uint32) error {
	query := "UPDATE wallets SET lastIndex = ? WHERE id = ?"
	_, err := p.db.Exec(query, index, walletId)
	if err != nil {
		return fmt.Errorf("failed to persist lastIndex: %w", err)
	}
	return nil
}

type Wallet struct {
	onchain.WalletInfo
	*onchain.WalletCredentials
	NodePubkey *string
	LastIndex  *uint32
}

func (d *Database) CreateWallet(wallet *Wallet) error {
	query := "INSERT INTO wallets (name, currency, xpub, coreDescriptor, mnemonic, subaccount, salt, tenantId, nodePubkey, lastIndex) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id"
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
		wallet.LastIndex,
	)
	return row.Scan(&wallet.Id)
}

func (d *Database) UpdateWallet(wallet *Wallet) error {
	query := "UPDATE wallets SET xpub = ?, coreDescriptor = ?, mnemonic = ?, subaccount = ?, salt = ? WHERE id = ?"
	_, err := d.Exec(
		query,
		wallet.Xpub,
		wallet.CoreDescriptor,
		wallet.Mnemonic,
		wallet.Subaccount,
		wallet.Salt,
		wallet.Id,
	)
	return err
}

func parseWallet(rows row) (*Wallet, error) {
	wallet := &Wallet{WalletCredentials: &onchain.WalletCredentials{}}
	var lastIndex sql.NullInt64
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
		&wallet.Legacy,
		&lastIndex, // scan into sql.NullInt64
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wallet: %w", err)
	}
	if lastIndex.Valid {
		idx := uint32(lastIndex.Int64)
		wallet.LastIndex = &idx
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

func (d *Database) QueryWalletCredentials() ([]*Wallet, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM wallets"
	rows, err := d.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets: %w", err)
	}
	defer closeRows(rows)

	var wallets []*Wallet
	for rows.Next() {
		wallet, err := parseWallet(rows)
		if err != nil {
			return nil, err
		}
		if wallet.NodePubkey == nil {
			wallets = append(wallets, wallet)
		}
	}

	return wallets, nil
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
