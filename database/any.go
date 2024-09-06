package database

import (
	"database/sql"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"time"
)

type AnySwap struct {
	Id         string
	Type       boltz.SwapType
	Pair       boltz.Pair
	State      boltzrpc.SwapState
	Error      string
	Status     boltz.SwapUpdateEvent
	Amount     uint64
	IsAuto     bool
	ServiceFee *uint64
	OnchainFee *uint64
	CreatedAt  time.Time
	TenantId   Id
}

func parseAnySwap(rows *sql.Rows) (*AnySwap, error) {
	var anySwap AnySwap

	var status string
	var createdAt, serviceFee, onchainFee sql.NullInt64

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":           &anySwap.Id,
			"type":         &anySwap.Type,
			"amount":       &anySwap.Amount,
			"fromCurrency": &anySwap.Pair.From,
			"toCurrency":   &anySwap.Pair.To,
			"state":        &anySwap.State,
			"error":        &anySwap.Error,
			"status":       &status,
			"isAuto":       &anySwap.IsAuto,
			"serviceFee":   &serviceFee,
			"onchainFee":   &onchainFee,
			"createdAt":    &createdAt,
			"tenantId":     &anySwap.TenantId,
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
	rows, err := database.Query("SELECT * FROM reverseSwaps WHERE id = ?", id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		return parseAnySwap(rows)
	}
	return nil, fmt.Errorf("could not find Reverse Swap %s", id)
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
