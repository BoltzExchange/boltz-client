package database

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc"
)

const statsQuery = `
SELECT COALESCE(SUM(serviceFee + onchainFee), 0), COALESCE(SUM(expectedAmount), 0), COUNT(*)
FROM (SELECT serviceFee, onchainFee, expectedAmount, isAuto, createdAt, false isChain
      FROM swaps
      UNION ALL
      SELECT serviceFee, onchainFee + (routingFeeMsat / 1000), expectedAmount, isAuto, createdAt, false isChain
      FROM reverseSwaps
      ) stats
`

func (database *Database) QueryStats(args SwapQuery, chainOnly bool) (*boltzrpc.SwapStats, error) {
	where, values := args.ToWhereClause()
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
