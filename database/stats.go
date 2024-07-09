package database

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"strings"
)

const statsQuery = `
SELECT COALESCE(SUM(serviceFee + onchainFee), 0), COALESCE(SUM(expectedAmount), 0), COUNT(*)
FROM (SELECT serviceFee, onchainFee, expectedAmount, isAuto, createdAt, 'submarine' type, tenantId
      FROM swaps
      UNION ALL
      SELECT serviceFee, onchainFee + (routingFeeMsat / 1000), expectedAmount, isAuto, createdAt, 'reverse' type, tenantId
      FROM reverseSwaps
      UNION ALL
      SELECT serviceFee, onchainFee, data.amount, isAuto, createdAt, 'chain' type, tenantId
      FROM chainSwaps
      JOIN chainSwapsData data ON chainSwaps.id = data.id AND data.currency = chainSwaps.fromCurrency
      ) stats 
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

	err := rows.Scan(&stats.TotalFees, &stats.TotalAmount, &stats.Count)
	if err != nil {
		return nil, err
	}
	if stats.Count != 0 {
		stats.AvgFees = stats.TotalFees / stats.Count
		stats.AvgAmount = stats.TotalAmount / stats.Count
	}
	return &stats, nil
}
