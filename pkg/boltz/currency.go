package boltz

import (
	"fmt"
	"strings"
)

type Currency string

const (
	CurrencyBtc       Currency = "BTC"
	CurrencyLiquid    Currency = "L-BTC"
	CurrencyRootstock Currency = "RBTC"
)

func ParseCurrency(currency string) (Currency, error) {
	switch strings.ToUpper(currency) {
	case string(CurrencyBtc):
		return CurrencyBtc, nil
	case string(CurrencyLiquid):
		return CurrencyLiquid, nil
	case string(CurrencyRootstock):
		return CurrencyRootstock, nil
	default:
		return "", fmt.Errorf("invalid currency: %v", currency)
	}
}

func (currency *Currency) UnmarshalText(data []byte) (err error) {
	*currency, err = ParseCurrency(string(data))
	return err
}
