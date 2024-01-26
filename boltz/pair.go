package boltz

import (
	"errors"
	"strings"
)

type Pair string

const (
	PairBtc    Pair = "BTC/BTC"
	PairLiquid Pair = "L-BTC/BTC"
)

func ParsePair(pairId string) (Pair, error) {
	switch strings.ToUpper(pairId) {
	case string(PairBtc):
		return PairBtc, nil
	case string(PairLiquid):
		return PairLiquid, nil
	case "BTC":
		return PairBtc, nil
	case "L-BTC":
		return PairLiquid, nil
	// backwards compatibility
	case "":
		return PairBtc, nil
	default:
		return "", errors.New("invalid pair id")
	}
}
