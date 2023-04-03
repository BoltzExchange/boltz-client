package mempool

import (
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"math"
)

type lndFee interface {
	EstimateFee(confTarget int32) (*walletrpc.EstimateFeeResponse, error)
}

type Mempool struct {
	mc  *client
	lnd lndFee
}

func Init(lnd lndFee, mcEndpoint string) *Mempool {
	var mc *client

	if mcEndpoint != "" {
		mc = initClient(mcEndpoint)
	} else {
		logger.Info("Disabled mempool.space integration")
	}

	return &Mempool{
		mc:  mc,
		lnd: lnd,
	}
}

func (m *Mempool) GetFeeEstimation() (int64, error) {
	if m.mc != nil {
		feeRecommendation, err := m.mc.getFeeRecommendation()
		if err == nil {
			return maxInt64(feeRecommendation.HalfHourFee, 2), nil
		}

		logger.Warning("Mempool fee estimation failed: " + err.Error())
		logger.Info("Falling back to LND fee estimation")
	}

	return m.getLndFeeEstimation()
}

func (m *Mempool) getLndFeeEstimation() (int64, error) {
	feeResponse, err := m.lnd.EstimateFee(2)
	if err != nil {
		return 0, err
	}

	return maxInt64(int64(math.Round(float64(feeResponse.SatPerKw)/1000)), 2), nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}

	return b
}
