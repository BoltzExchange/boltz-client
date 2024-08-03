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
FROM (SELECT state,
             serviceFee,
             onchainFee,
             expectedAmount amount,
             isAuto,
             createdAt,
             'submarine'    type,
             tenantId
      FROM swaps
      UNION ALL
      SELECT state,
             serviceFee,
             COALESCE(onchainFee, 0) + COALESCE(routingFeeMsat / 1000, 0) onchainFee,
             expectedAmount amount,
             isAuto,
             createdAt,
             'reverse'      type,
             tenantId
      FROM reverseSwaps
      UNION ALL
      SELECT state,
             serviceFee,
             onchainFee,
             data.amount amount,
             isAuto,
             createdAt,
             'chain'     type,
             tenantId
      FROM chainSwaps
               JOIN chainSwapsData data ON chainSwaps.id = data.id AND data.currency = chainSwaps.fromCurrency) stats

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
