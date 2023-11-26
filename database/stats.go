package database

import (
	"time"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
)

func (database *Database) QueryStats(since time.Time, isAuto bool) (*boltzrpc.SwapStats, error) {
	query := `
		SELECT COALESCE(SUM(serviceFee + onchainFee), 0), COALESCE(SUM(expectedAmount), 0), COUNT(*)
		FROM (SELECT serviceFee, onchainFee, expectedAmount, isAuto, createdAt
			  FROM swaps
			  UNION ALL
			  SELECT serviceFee, onchainFee + (routingFeeMsat / 1000), expectedAmount, isAuto, createdAt
			  FROM reverseSwaps) stats
		WHERE createdAt >= ? AND isAuto = ?`

	ts := since.Unix()

	rows := database.db.QueryRow(query, ts, isAuto)

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
