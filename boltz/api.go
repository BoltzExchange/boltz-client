package boltz

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/r3labs/sse"
	"io"
	"io/ioutil"
	"net/http"
)

type Boltz struct {
	URL string `long:"boltz.url" description:"URL endpoint of the Boltz API"`
}

// Types for Boltz API
type symbolMinerFees struct {
	Normal  int `json:"normal"`
	Reverse struct {
		Lockup int `json:"lockup"`
		Claim  int `json:"claim"`
	} `json:"reverse"`
}

type GetPairsResponse struct {
	Warnings []string `json:"warnings"`
	Pairs    map[string]struct {
		Rate   float32 `json:"rate"`
		Limits struct {
			Maximal int `json:"maximal"`
			Minimal int `json:"minimal"`
		} `json:"limits"`
		Fees struct {
			Percentage int `json:"percentage"`
			MinerFees  struct {
				BaseAsset  symbolMinerFees `json:"baseAsset"`
				QuoteAsset symbolMinerFees `json:"quoteAsset"`
			} `json:"minerFees"`
		} `json:"fees"`
	} `json:"pairs"`
}

type GetNodesResponse struct {
	Nodes map[string]struct {
		NodeKey string   `json:"nodeKey"`
		URIs    []string `json:"uris"`
	} `json:"nodes"`
}

type SwapStatusRequest struct {
	Id string `json:"id"`
}

type SwapStatusResponse struct {
	Status string `json:"status"`

	Error string `json:"error"`
}

type CreateSwapRequest struct {
	Type            string `json:"type"`
	PairId          string `json:"pairId"`
	OrderSide       string `json:"orderSide"`
	RefundPublicKey string `json:"refundPublicKey"`
	Invoice         string `json:"invoice"`
}

type CreateSwapResponse struct {
	Id                 string `json:"id"`
	AcceptZeroConf     bool   `json:"acceptZeroConf"`
	ExpectedAmount     int    `json:"expectedAmount"`
	TimeoutBlockHeight int    `json:"timeoutBlockHeight"`
	Address            string `json:"address"`
	Bip21              string `json:"bip21"`
	RedeemScript       string `json:"redeemScript"`

	Error string `json:"error"`
}

func (boltz *Boltz) GetPairs() (*GetPairsResponse, error) {
	var response GetPairsResponse
	err := boltz.sendGetRequest("/getpairs", &response)

	return &response, err
}

func (boltz *Boltz) GetFeeEstimation() (*map[string]int, error) {
	var response map[string]int
	err := boltz.sendGetRequest("/getfeeestimation", &response)

	return &response, err
}

func (boltz *Boltz) GetNodes() (*GetNodesResponse, error) {
	var response GetNodesResponse
	err := boltz.sendGetRequest("/getnodes", &response)

	return &response, err
}

func (boltz *Boltz) SwapStatus(id string) (*SwapStatusResponse, error) {
	var response SwapStatusResponse
	err := boltz.sendPostRequest("/swapstatus", SwapStatusRequest{
		Id: id,
	}, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) StreamSwapStatus(id string, channel chan *sse.Event) (*sse.Client, error) {
	client := sse.NewClient(boltz.URL + "/streamswapstatus?id=" + id)
	err := client.SubscribeChan("data", channel)

	return client, err
}

func (boltz *Boltz) CreateSwap(request CreateSwapRequest) (*CreateSwapResponse, error) {
	var response CreateSwapResponse
	err := boltz.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) sendGetRequest(endpoint string, response interface{}) error {
	res, err := http.Get(boltz.URL + endpoint)

	if err != nil {
		return err
	}

	return unmarshalJson(res.Body, &response)
}

func (boltz *Boltz) sendPostRequest(endpoint string, requestBody interface{}, response interface{}) error {
	rawBody, err := json.Marshal(requestBody)

	if err != nil {
		return err
	}

	res, err := http.Post(boltz.URL+endpoint, "application/json", bytes.NewBuffer(rawBody))

	if err != nil {
		return err
	}

	return unmarshalJson(res.Body, &response)
}

func unmarshalJson(body io.ReadCloser, response interface{}) error {
	rawBody, err := ioutil.ReadAll(body)

	if err != nil {
		return err
	}

	return json.Unmarshal(rawBody, &response)
}
