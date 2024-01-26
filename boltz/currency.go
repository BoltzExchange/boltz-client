package boltz

import (
	"fmt"
	"strings"
)

type Currency string

const (
	CurrencyBtc    Currency = "BTC"
	CurrencyLiquid Currency = "L-BTC"
)

func ParseCurrency(currency string) (Currency, error) {
	switch strings.ToUpper(currency) {
	case string(CurrencyBtc):
		return CurrencyBtc, nil
	case string(CurrencyLiquid):
		return CurrencyLiquid, nil
	default:
		return "", fmt.Errorf("invalid currency: %v", currency)
	}
}

func CurrencyForPair(pair Pair) Currency {
	switch pair {
	case PairBtc:
		return CurrencyBtc
	case PairLiquid:
		return CurrencyLiquid
	default:
		return ""
	}
}
