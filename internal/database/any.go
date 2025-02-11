package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
)

type AnySwap struct {
	Id                  string
	Type                boltz.SwapType
	Pair                boltz.Pair
	State               boltzrpc.SwapState
	Error               string
	Status              boltz.SwapUpdateEvent
	Amount              uint64
	IsAuto              bool
	ServiceFee          *uint64
	OnchainFee          *uint64
	CreatedAt           time.Time
	LockupTransactionid string
	ClaimTransactionid  string
	RefundTransactionid string
	TenantId            Id
}

func parseAnySwap(rows *sql.Rows) (*AnySwap, error) {
	var anySwap AnySwap

	var status string
	var createdAt, serviceFee, onchainFee sql.NullInt64

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &anySwap.Id,
			"type":                &anySwap.Type,
			"amount":              &anySwap.Amount,
			"fromCurrency":        &anySwap.Pair.From,
			"toCurrency":          &anySwap.Pair.To,
			"state":               &anySwap.State,
			"error":               &anySwap.Error,
			"status":              &status,
			"isAuto":              &anySwap.IsAuto,
			"serviceFee":          &serviceFee,
			"onchainFee":          &onchainFee,
			"createdAt":           &createdAt,
			"claimTransactionId":  &anySwap.ClaimTransactionid,
			"lockupTransactionId": &anySwap.LockupTransactionid,
			"refundTransactionId": &anySwap.RefundTransactionid,
			"tenantId":            &anySwap.TenantId,
		},
	)

	if err != nil {
		return nil, err
	}

	anySwap.ServiceFee = parseNullInt(serviceFee)
	anySwap.OnchainFee = parseNullInt(onchainFee)
	anySwap.Status = boltz.ParseEvent(status)
	anySwap.CreatedAt = parseTime(createdAt.Int64)

	return &anySwap, err
}

func (database *Database) GetAnySwap(id string) (*AnySwap, error) {
	database.lock.Lock()
	defer database.lock.Unlock()
	rows, err := database.Query("SELECT * FROM allSwaps WHERE id = ?", id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		return parseAnySwap(rows)
	}
	return nil, fmt.Errorf("could not find swap %s", id)
}
func (database *Database) queryAllSwaps(query string, values ...any) (swaps []*AnySwap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, values...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		swap, err := parseAnySwap(rows)

		if err != nil {
			return nil, err
		}

		swaps = append(swaps, swap)
	}

	return swaps, err
}

func (database *Database) QueryAllSwaps(args SwapQuery) ([]*AnySwap, error) {
	where, values := args.ToWhereClause()
	return database.queryAllSwaps("SELECT * FROM allSwaps"+where, values...)
}

func (database *Database) QuerySwapsByTransactions(args SwapQuery, transactionIds []string) ([]*AnySwap, error) {
	var placeholders []string
	for i := range transactionIds {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	placeholder := strings.Join(placeholders, ", ")
	values := make([]any, len(transactionIds))
	for i, txId := range transactionIds {
		values[i] = txId
	}
	where, values := args.ToWhereClauseWithExisting(
		[]string{fmt.Sprintf("lockupTransactionid IN (%s) OR claimTransactionid IN (%s) OR refundTransactionid IN (%s)", placeholder, placeholder, placeholder)},
		values,
	)
	return database.queryAllSwaps("SELECT * FROM allSwaps"+where, values...)
}
