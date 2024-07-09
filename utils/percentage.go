package utils

import (
	"fmt"
	"math"
	"strconv"
)

type Percentage float64

func (p Percentage) String() string {
	return fmt.Sprintf("%.2f%%", float64(p))
}

func (p Percentage) Ratio() float64 {
	return float64(p / 100)
}

func (p Percentage) Calculate(value uint64) uint64 {
	return uint64(math.Ceil(float64(value) * p.Ratio()))
}

func (p *Percentage) UnmarshalJSON(text []byte) error {
	str := StripQuotes(text)

	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	*p = Percentage(parsed)
	return nil
}
