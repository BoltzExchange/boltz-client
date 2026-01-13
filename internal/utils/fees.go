package utils

import (
	"fmt"
	"math"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
)

func CalculateFeeEstimate(fees *boltzrpc.SwapFees, amount uint64) uint64 {
	serviceFee := boltz.Percentage(fees.Percentage).Calculate(amount)
	return serviceFee + fees.MinerFees
}

type SwapQuote struct {
	SendAmount    uint64
	ReceiveAmount uint64
	BoltzFee      uint64
	NetworkFee    uint64
}

func CalculateSwapQuote(swapType boltz.SwapType, sendAmount, receiveAmount uint64, fees *boltzrpc.SwapFees) (*SwapQuote, error) {
	percentage := boltz.Percentage(fees.Percentage)
	minerFee := fees.MinerFees

	quote := &SwapQuote{
		NetworkFee: minerFee,
	}

	if sendAmount > 0 && receiveAmount > 0 {
		return nil, fmt.Errorf("cant have both send and receive amount")
	}
	if sendAmount == 0 && receiveAmount == 0 {
		return nil, fmt.Errorf("either send or receive amount have to be specified")
	}

	if sendAmount > 0 {
		quote.SendAmount = sendAmount

		switch swapType {
		case boltz.NormalSwap:
			// Submarine: service fee on receive, so receive = (send - minerFee) / (1 + rate)
			rate := 1 + percentage.Ratio()
			receive := max(0, float64(sendAmount)-float64(minerFee)) / rate
			quote.ReceiveAmount = uint64(receive)
			quote.BoltzFee = percentage.Calculate(quote.ReceiveAmount)
		case boltz.ReverseSwap, boltz.ChainSwap:
			// Reverse/Chain: service fee on send
			quote.BoltzFee = percentage.Calculate(sendAmount)
			quote.ReceiveAmount = uint64(max(0, int64(sendAmount)-int64(quote.BoltzFee)-int64(minerFee)))
		}
	} else {
		quote.ReceiveAmount = receiveAmount

		switch swapType {
		case boltz.NormalSwap:
			// Submarine: service fee on receive
			quote.BoltzFee = percentage.Calculate(receiveAmount)
			quote.SendAmount = receiveAmount + quote.BoltzFee + minerFee
		case boltz.ReverseSwap, boltz.ChainSwap:
			// Reverse/Chain: service fee on send, so send = (receive + minerFee) / (1 - rate)
			rate := 1 - percentage.Ratio()
			if rate == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			sendAmount := math.Ceil(float64(receiveAmount+minerFee) / rate)
			quote.SendAmount = uint64(sendAmount)
			quote.BoltzFee = percentage.Calculate(quote.SendAmount)
		}
	}

	return quote, nil
}
