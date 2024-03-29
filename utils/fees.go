package utils

import "github.com/BoltzExchange/boltz-client/boltzrpc"

func CalculateFeeEstimate(fees *boltzrpc.SwapFees, amount uint64) uint64 {
	serviceFee := Percentage(fees.Percentage).Calculate(amount)
	return serviceFee + fees.MinerFees
}
