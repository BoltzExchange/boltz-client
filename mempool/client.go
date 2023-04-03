package mempool

import (
	"encoding/json"
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"net/http"
	"strings"
)

const feeRecommendationEndpoint = "/v1/fees/recommended"

type feeEstimation struct {
	FastestFee  int64 `json:"fastestFee"`
	HalfHourFee int64 `json:"halfHourFee"`
	HourFee     int64 `json:"hourFee"`
	EconomyFee  int64 `json:"economyFee"`
	MinimumFee  int64 `json:"minimumFee"`
}

type client struct {
	api string
}

func initClient(endpoint string) *client {
	endpointStripped := strings.TrimSuffix(endpoint, "/")
	logger.Info("mempool.space API for fee estimations: " + endpointStripped)

	return &client{
		api: endpointStripped,
	}
}

func (c *client) getFeeRecommendation() (*feeEstimation, error) {
	req, err := http.NewRequest(http.MethodGet, c.api+feeRecommendationEndpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with status: %d", res.StatusCode)
	}

	var fees feeEstimation
	err = json.NewDecoder(res.Body).Decode(&fees)
	if err != nil {
		return nil, err
	}

	return &fees, nil
}
