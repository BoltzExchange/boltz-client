package database

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"strings"
)

const statsQuery = `
SELECT COALESCE(SUM(COALESCE(serviceFee, 0) + COALESCE(onchainFee, 0)), 0),
       COALESCE(SUM(CASE WHEN state == 1 THEN amount END), 0),
       COUNT(*),
       COUNT(CASE WHEN state == 1 THEN 1 END)
FROM allSwaps
`

func (database *Database) QueryStats(args SwapQuery, swapTypes []boltz.SwapType) (*boltzrpc.SwapStats, error) {
	var placeholders []string
	var values []any
	for _, swapType := range swapTypes {
		placeholders = append(placeholders, "?")
		values = append(values, swapType)
	}
	where, values := args.ToWhereClauseWithExisting([]string{fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ", "))}, values)

	rows := database.QueryRow(statsQuery+where, values...)

	stats := boltzrpc.SwapStats{}

	err := rows.Scan(&stats.TotalFees, &stats.TotalAmount, &stats.Count, &stats.SuccessCount)
	if err != nil {
		return nil, err
	}
	if stats.SuccessCount != 0 {
		stats.AvgFees = stats.TotalFees / stats.SuccessCount
		stats.AvgAmount = stats.TotalAmount / stats.SuccessCount
	}
	return &stats, nil
}
