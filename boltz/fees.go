package boltz

import (
	"errors"
	"fmt"
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

func calcNetworkFee(swapType SwapType, pair Pair, estimations FeeEstimations, includeClaim bool) float64 {
	switch swapType {
	case NormalSwap:
		return float64(Sizes[pair.From].NormalClaim) * estimations[pair.From]
	case ReverseSwap:
		size := Sizes[pair.To].ReverseLockup
		if includeClaim {
			size += Sizes[pair.To].ReverseClaim
		}
		return float64(size) * estimations[pair.To]
	case ChainSwap:
		return calcNetworkFee(NormalSwap, pair, estimations, includeClaim) + calcNetworkFee(ReverseSwap, pair, estimations, includeClaim)
	default:
		return 0
	}
}

var ErrInvalidOnchainFee = errors.New("onchain fee way above expectation")

const FeeTolerance = 1.25

func CheckAmounts(swapType SwapType, pair Pair, sendAmount uint64, receiveAmount uint64, serviceFee Percentage, estimations FeeEstimations, includeClaim bool) error {
	totalFees := sendAmount - receiveAmount
	onchainFees := totalFees
	if swapType == NormalSwap {
		onchainFees -= serviceFee.Calculate(receiveAmount)
	} else {
		onchainFees -= serviceFee.Calculate(sendAmount)
	}
	expected := calcNetworkFee(swapType, pair, estimations, includeClaim)
	if float64(onchainFees) > expected*FeeTolerance {
		return fmt.Errorf(
			"%w: %d > %d*%.2f (service fee: %s, totalFees: %d)",
			ErrInvalidOnchainFee, onchainFees, uint64(expected), FeeTolerance, serviceFee, totalFees,
		)
	}
	return nil
}
