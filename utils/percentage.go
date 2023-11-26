package utils

import "fmt"

type Percentage float64

func (p Percentage) String() string {
	return fmt.Sprintf("%.2f%%", float64(p))
}

func (p Percentage) Ratio() float64 {
	return float64(p / 100)
}

func (p Percentage) Calculate(value float64) float64 {
	return value * p.Ratio()
}
