package utils

import (
	"bytes"
	"golang.org/x/exp/constraints"
	"os"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
)

func CurrencyFromPair(pair boltz.Pair) string {
	return strings.Split(string(pair), "/")[0]
}

// TODO: test this on real network
func FormatMilliSat(milliSat int64) string {
	return strconv.FormatFloat(float64(milliSat)/1000, 'f', 3, 64)
}

func Satoshis[V constraints.Integer](sat V) string {
	return strconv.Itoa(int(sat)) + " satoshis"
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func StripQuotes(text []byte) string {
	return string(bytes.Trim(text, "\""))
}
