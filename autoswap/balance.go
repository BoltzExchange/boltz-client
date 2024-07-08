package autoswap

import (
	"github.com/BoltzExchange/boltz-client/utils"
	"math"
)

type Balance struct {
	Absolute uint64
	Relative utils.Percentage
}

func (b Balance) IsZero() bool {
	return b.Absolute == 0 && b.Relative == 0
}

func (b Balance) IsAbsolute() bool {
	return b.Absolute != 0
}

func (b Balance) Get(capacity uint64) uint64 {
	if b.IsAbsolute() {
		return uint64(math.Min(float64(b.Absolute), float64(capacity)))
	}
	return b.Relative.Calculate(capacity)
}

func (b Balance) String() string {
	if b.IsAbsolute() {
		return utils.Satoshis(b.Absolute)
	}
	return b.Relative.String()
}
