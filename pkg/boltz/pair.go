package boltz

import "fmt"

type Pair struct {
	From Currency
	To   Currency
}

var (
	PairBtc = Pair{From: CurrencyBtc, To: CurrencyBtc}
)

func (p Pair) String() string {
	return string(p.From) + "/" + string(p.To)
}

func FindPair[T any](pair Pair, nested map[Currency]map[Currency]T) (*T, error) {
	from, hasPair := nested[pair.From]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair from %v", pair)
	}
	result, hasPair := from[pair.To]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair to %v", pair)
	}
	return &result, nil
}
