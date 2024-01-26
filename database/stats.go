package database

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc"
)

func (database *Database) QueryStats(args SwapQuery) (*boltzrpc.SwapStats, error) {
	where, values := args.ToWhereClause()
	query := `
		SELECT COALESCE(SUM(serviceFee + onchainFee), 0), COALESCE(SUM(expectedAmount), 0), COUNT(*)
		FROM (SELECT serviceFee, onchainFee, expectedAmount, isAuto, createdAt
			  FROM swaps
			  UNION ALL
			  SELECT serviceFee, onchainFee + (routingFeeMsat / 1000), expectedAmount, isAuto, createdAt
			  FROM reverseSwaps) stats
		` + where

	rows := database.QueryRow(query, values...)

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
