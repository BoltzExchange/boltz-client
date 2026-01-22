package database

import (
	"database/sql"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
)

type FundingAddress struct {
	Id                  string
	Currency            boltz.Currency
	Address             string
	TimeoutBlockHeight  uint32
	BoltzPublicKey      *btcec.PublicKey
	PrivateKey          *btcec.PrivateKey
	BlindingKey         *btcec.PrivateKey
	Status              string
	LockupTransactionId string
	SwapId              string
	CreatedAt           time.Time
	TenantId            Id
}

func (f *FundingAddress) GetFundingTree() (*boltz.FundingTree, error) {
	return boltz.NewFundingTree(f.Currency, f.PrivateKey, f.BoltzPublicKey, f.TimeoutBlockHeight)
}

func parseFundingAddress(rows *sql.Rows) (*FundingAddress, error) {
	var fa FundingAddress
	privateKey := PrivateKeyScanner{}
	boltzPublicKey := PublicKeyScanner{}
	blindingKey := PrivateKeyScanner{}
	var createdAt sql.NullInt64

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &fa.Id,
			"currency":            &fa.Currency,
			"address":             &fa.Address,
			"timeoutBlockheight":  &fa.TimeoutBlockHeight,
			"boltzPublicKey":      &boltzPublicKey,
			"privateKey":          &privateKey,
			"blindingKey":         &blindingKey,
			"status":              &fa.Status,
			"lockupTransactionId": &fa.LockupTransactionId,
			"swapId":              &fa.SwapId,
			"createdAt":           &createdAt,
			"tenantId":            &fa.TenantId,
		},
	)

	if err != nil {
		return nil, err
	}

	fa.PrivateKey = privateKey.Value
	fa.BoltzPublicKey = boltzPublicKey.Value
	fa.BlindingKey = blindingKey.Value
	fa.CreatedAt = parseTime(createdAt.Int64)

	return &fa, nil
}

func (database *Database) QueryFundingAddress(id string) (*FundingAddress, error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM fundingAddresses WHERE id = ?", id)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	if rows.Next() {
		return parseFundingAddress(rows)
	}

	return nil, sql.ErrNoRows
}

func (database *Database) QueryFundingAddressesBySwapId(swapId string) (*FundingAddress, error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM fundingAddresses WHERE swapId = ?", swapId)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	if rows.Next() {
		return parseFundingAddress(rows)
	}

	return nil, sql.ErrNoRows
}

func (database *Database) queryFundingAddresses(query string, args ...any) ([]*FundingAddress, error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, args...)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	var addresses []*FundingAddress
	for rows.Next() {
		fa, err := parseFundingAddress(rows)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, fa)
	}

	return addresses, nil
}

type FundingAddressQuery struct {
	Currency *boltz.Currency
	TenantId *Id
}

func (database *Database) QueryFundingAddresses(query FundingAddressQuery) ([]*FundingAddress, error) {
	sql := "SELECT * FROM fundingAddresses"
	var conditions []string
	var values []any

	if query.Currency != nil {
		conditions = append(conditions, "currency = ?")
		values = append(values, *query.Currency)
	}
	if query.TenantId != nil {
		conditions = append(conditions, "tenantId = ?")
		values = append(values, *query.TenantId)
	}

	if len(conditions) > 0 {
		sql += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				sql += " AND "
			}
			sql += cond
		}
	}

	sql += " ORDER BY createdAt DESC"

	return database.queryFundingAddresses(sql, values...)
}

func (database *Database) QueryPendingFundingAddresses() ([]*FundingAddress, error) {
	return database.queryFundingAddresses("SELECT * FROM fundingAddresses WHERE status NOT IN ('spent', 'expired', 'refunded')")
}

const insertFundingAddressStatement = `
INSERT INTO fundingAddresses (id, currency, address, timeoutBlockheight, boltzPublicKey, privateKey, blindingKey, status, lockupTransactionId, swapId, createdAt, tenantId)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateFundingAddress(fa FundingAddress) error {
	_, err := database.Exec(
		insertFundingAddressStatement,
		fa.Id,
		fa.Currency,
		fa.Address,
		fa.TimeoutBlockHeight,
		formatPublicKey(fa.BoltzPublicKey),
		formatPrivateKey(fa.PrivateKey),
		formatPrivateKey(fa.BlindingKey),
		fa.Status,
		fa.LockupTransactionId,
		fa.SwapId,
		FormatTime(fa.CreatedAt),
		fa.TenantId,
	)
	return err
}

func (database *Database) UpdateFundingAddressStatus(fa *FundingAddress, status string) error {
	fa.Status = status

	_, err := database.Exec("UPDATE fundingAddresses SET status = ? WHERE id = ?", status, fa.Id)
	return err
}

func (database *Database) SetFundingAddressLockupTransactionId(fa *FundingAddress, lockupTransactionId string) error {
	fa.LockupTransactionId = lockupTransactionId

	_, err := database.Exec("UPDATE fundingAddresses SET lockupTransactionId = ? WHERE id = ?", lockupTransactionId, fa.Id)
	return err
}

func (database *Database) SetFundingAddressSwapId(fa *FundingAddress, swapId string) error {
	fa.SwapId = swapId

	_, err := database.Exec("UPDATE fundingAddresses SET swapId = ? WHERE id = ?", swapId, fa.Id)
	return err
}
