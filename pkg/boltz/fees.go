package boltz

import (
	"errors"
	"fmt"
	"math"
)

type TxSizes struct {
	NormalClaim   uint64
	ReverseLockup uint64
	ReverseClaim  uint64
}

var BtcSizes = TxSizes{}
var LiquidSizes = TxSizes{}

var Sizes = map[Currency]TxSizes{
	CurrencyBtc: {
		NormalClaim:   151,
		ReverseLockup: 154,
		ReverseClaim:  111,
	},
	CurrencyLiquid: {
		NormalClaim:   1337,
		ReverseLockup: 2503,
		ReverseClaim:  1309,
	},
}

type FeeEstimations map[Currency]float64

func calcNetworkFee(swapType SwapType, pair Pair, estimations FeeEstimations, includeClaim bool) uint64 {
	result := 0.0
	switch swapType {
	case NormalSwap:
		result = float64(Sizes[pair.From].NormalClaim) * estimations[pair.From]
	case ReverseSwap:
		size := Sizes[pair.To].ReverseLockup
		if includeClaim {
			size += Sizes[pair.To].ReverseClaim
		}
		result = float64(size) * estimations[pair.To]
	case ChainSwap:
		return calcNetworkFee(NormalSwap, pair, estimations, includeClaim) + calcNetworkFee(ReverseSwap, pair, estimations, includeClaim)
	}
	return uint64(math.Ceil(result))
}

var ErrInvalidOnchainFee = errors.New("onchain fee way above expectation")

const RelativeFeeTolerance = Percentage(25)
const AbsoluteFeeToleranceSat = 1500

func checkTolerance(expected uint64, actual uint64) error {
	tolerance := max(AbsoluteFeeToleranceSat, CalculatePercentage(RelativeFeeTolerance, expected))
	if actual > expected+tolerance {
		return fmt.Errorf("%w: %d > %d+%d", ErrInvalidOnchainFee, actual, expected, tolerance)
	}
	return nil
}

func RequiredEstimations(swapType SwapType, pair Pair) []Currency {
	currencies := make([]Currency, 0)
	switch swapType {
	case NormalSwap:
		currencies = append(currencies, pair.From)
	case ReverseSwap:
		currencies = append(currencies, pair.To)
	case ChainSwap:
		currencies = append(currencies, pair.From, pair.To)
	}
	return currencies
}

func CheckAmounts(swapType SwapType, pair Pair, sendAmount uint64, receiveAmount uint64, serviceFee Percentage, estimations FeeEstimations, includeClaim bool) error {
	totalFees := sendAmount - receiveAmount
	networkFees := totalFees
	if swapType == NormalSwap {
		networkFees -= CalculatePercentage(serviceFee, receiveAmount)
	} else {
		networkFees -= CalculatePercentage(serviceFee, sendAmount)
	}
	currencies := RequiredEstimations(swapType, pair)
	for _, currency := range currencies {
		if _, ok := estimations[currency]; !ok {
			return fmt.Errorf("no estimation for currency %s", currency)
		}
	}
	return checkTolerance(calcNetworkFee(swapType, pair, estimations, includeClaim), networkFees)
}
