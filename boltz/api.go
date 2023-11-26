package boltz

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type Boltz struct {
	URL string `long:"boltz.url" description:"URL endpoint of the Boltz API"`
}

type SwapType string

const (
	NormalSwap  SwapType = "submarine"
	ReverseSwap SwapType = "reverseSubmarine"
)

type Error error

// Types for Boltz API
type GetVersionResponse struct {
	Version string `json:"version"`
}

type symbolMinerFees struct {
	Normal  uint64 `json:"normal"`
	Reverse struct {
		Lockup uint64 `json:"lockup"`
		Claim  uint64 `json:"claim"`
	} `json:"reverse"`
}

type GetPairsResponse struct {
	Warnings []string `json:"warnings"`
	Pairs    map[string]struct {
		Rate   float32 `json:"rate"`
		Limits struct {
			Maximal uint64 `json:"maximal"`
			Minimal uint64 `json:"minimal"`
		} `json:"limits"`
		Fees struct {
			Percentage float32 `json:"percentage"`
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
	Status      string `json:"status"`
	Transaction struct {
		Id  string `json:"id"`
		Hex string `json:"hex"`
	} `json:"transaction"`

	Error string `json:"error"`
}

type GetSwapTransactionRequest struct {
	Id string `json:"id"`
}

type GetSwapTransactionResponse struct {
	TransactionHex     string `json:"transactionHex"`
	TimeoutBlockHeight uint32 `json:"timeoutBlockHeight"`
	TimeoutEta         uint64 `json:"timeoutEta"`

	Error string `json:"error"`
}

type BroadcastTransactionRequest struct {
	Currency       string `json:"currency"`
	TransactionHex string `json:"transactionHex"`
}

type BroadcastTransactionResponse struct {
	TransactionId string `json:"transactionId"`

	Error string `json:"error"`
}

type CreateSwapRequest struct {
	Type            SwapType `json:"type"`
	PairId          string   `json:"pairId"`
	OrderSide       string   `json:"orderSide"`
	RefundPublicKey string   `json:"refundPublicKey"`
	Invoice         string   `json:"invoice"`
	PreimageHash    string   `json:"preimageHash"`
}

type CreateSwapResponse struct {
	Id                 string `json:"id"`
	Bip21              string `json:"bip21"`
	Address            string `json:"address"`
	RedeemScript       string `json:"redeemScript"`
	AcceptZeroConf     bool   `json:"acceptZeroConf"`
	ExpectedAmount     uint64 `json:"expectedAmount"`
	TimeoutBlockHeight uint32 `json:"timeoutBlockHeight"`
	BlindingKey        string `json:"blindingKey"`

	Error string `json:"error"`
}

type SwapRatesRequest struct {
	Id string `json:"id"`
}

type SwapRatesResponse struct {
	OnchainAmount uint64 `json:"onchainAmount"`
	SubmarineSwap struct {
		InvoiceAmount uint64 `json:"invoiceAmount"`
	} `json:"submarineSwap"`

	Error string `json:"error"`
}

type SetInvoiceRequest struct {
	Id      string `json:"id"`
	Invoice string `json:"invoice"`
}

type SetInvoiceResponse struct {
	Error string `json:"error"`
}

type CreateReverseSwapRequest struct {
	Type           SwapType `json:"type"`
	PairId         string   `json:"pairId"`
	OrderSide      string   `json:"orderSide"`
	InvoiceAmount  uint64   `json:"invoiceAmount"`
	PreimageHash   string   `json:"preimageHash"`
	ClaimPublicKey string   `json:"claimPublicKey"`
}

type CreateReverseSwapResponse struct {
	Id                 string `json:"id"`
	Invoice            string `json:"invoice"`
	OnchainAmount      uint64 `json:"onchainAmount"`
	RedeemScript       string `json:"redeemScript"`
	LockupAddress      string `json:"lockupAddress"`
	TimeoutBlockHeight uint32 `json:"TimeoutBlockHeight"`
	BlindingKey        string `json:"blindingKey"`

	Error string `json:"error"`
}

func (boltz *Boltz) GetVersion() (*GetVersionResponse, error) {
	var response GetVersionResponse
	err := boltz.sendGetRequest("/version", &response)

	return &response, err
}

func (boltz *Boltz) GetPairs() (*GetPairsResponse, error) {
	var response GetPairsResponse
	err := boltz.sendGetRequest("/getpairs", &response)

	return &response, err
}

func (boltz *Boltz) GetFeeEstimation() (*map[string]uint64, error) {
	var response map[string]uint64
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
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) StreamSwapStatus(id string, events chan *SwapStatusResponse, stopListening <-chan bool) error {
	return streamSwapStatus(boltz.URL+"/streamswapstatus?id="+id, events, stopListening)
}

func (boltz *Boltz) GetSwapTransaction(id string) (*GetSwapTransactionResponse, error) {
	var response GetSwapTransactionResponse
	err := boltz.sendPostRequest("/getswaptransaction", GetSwapTransactionRequest{
		Id: id,
	}, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) BroadcastTransaction(transactionHex string, currency string) (*BroadcastTransactionResponse, error) {
	var response BroadcastTransactionResponse
	err := boltz.sendPostRequest("/broadcasttransaction", BroadcastTransactionRequest{
		Currency:       currency,
		TransactionHex: transactionHex,
	}, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) CreateSwap(request CreateSwapRequest) (*CreateSwapResponse, error) {
	var response CreateSwapResponse
	err := boltz.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) SwapRates(request SwapRatesRequest) (*SwapRatesResponse, error) {
	var response SwapRatesResponse
	err := boltz.sendPostRequest("/swaprates", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) SetInvoice(request SetInvoiceRequest) (*SetInvoiceResponse, error) {
	var response SetInvoiceResponse
	err := boltz.sendPostRequest("/setinvoice", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) CreateReverseSwap(request CreateReverseSwapRequest) (*CreateReverseSwapResponse, error) {
	var response CreateReverseSwapResponse
	err := boltz.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
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
	rawBody, err := io.ReadAll(body)

	if err != nil {
		return err
	}

	return json.Unmarshal(rawBody, &response)
}
