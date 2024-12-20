package utils

import (
	"github.com/BoltzExchange/boltz-client/v2/boltz"
	"github.com/BoltzExchange/boltz-client/v2/boltzrpc"
)

func CalculateFeeEstimate(fees *boltzrpc.SwapFees, amount uint64) uint64 {
	serviceFee := boltz.Percentage(fees.Percentage).Calculate(amount)
	return serviceFee + fees.MinerFees
}
