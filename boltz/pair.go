package boltz

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
