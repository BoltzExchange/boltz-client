package mempool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/btcsuite/websocket"
)

const feeRecommendationEndpoint = "/v1/fees/recommended"

var upgrader = websocket.Upgrader{}

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

func (c *client) startBlockStream() (chan *BlockEpoch, error) {

	mempool, err := url.Parse(c.api)

	ws := url.URL{
		Scheme: "wss",
		Host:   mempool.Host,
		Path:   "/api/v1/ws",
	}

	fmt.Printf("connecting to %s\n", ws.String())

	conn, _, err := websocket.DefaultDialer.Dial(ws.String(), nil)
	if err != nil {
		return nil, err
	}

	rcv := make(chan *BlockEpoch)

	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"want", "data": ["blocks", 'stats', 'mempool-blocks']}`))

	if err != nil {
		return nil, err
	}

	go func() {
		defer close(rcv)
		for {
			fmt.Println("waiting for message")
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}
			rcv <- &BlockEpoch{}
			fmt.Printf("recv: %s", message)
		}
	}()

	return rcv, nil

}
