package boltz

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
)

type Boltz struct {
	symbol string

	URL string `long:"boltz.url" description:"URL endpoint of the Boltz API"`
}

// Types for Boltz API
type GetVersionResponse struct {
	Version string `json:"version"`
}

type symbolMinerFees struct {
	Normal  int `json:"normal"`
	Reverse struct {
		Lockup int64 `json:"lockup"`
		Claim  int64 `json:"claim"`
	} `json:"reverse"`
}

type GetPairsResponse struct {
	Warnings []string `json:"warnings"`
	Pairs    map[string]struct {
		Rate   float32 `json:"rate"`
		Limits struct {
			Maximal int64 `json:"maximal"`
			Minimal int64 `json:"minimal"`
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
	Status  string `json:"status"`
	Channel struct {
		FundingTransactionId   string `json:"fundingTransactionId"`
		FundingTransactionVout int    `json:"fundingTransactionVout"`
	} `json:"channel"`
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
	Type            string `json:"type"`
	PairId          string `json:"pairId"`
	OrderSide       string `json:"orderSide"`
	RefundPublicKey string `json:"refundPublicKey"`
	Invoice         string `json:"invoice"`
	PreimageHash    string `json:"preimageHash"`
}

type CreateSwapResponse struct {
	Id                 string `json:"id"`
	Bip21              string `json:"bip21"`
	Address            string `json:"address"`
	RedeemScript       string `json:"redeemScript"`
	AcceptZeroConf     bool   `json:"acceptZeroConf"`
	ExpectedAmount     int    `json:"expectedAmount"`
	TimeoutBlockHeight int    `json:"timeoutBlockHeight"`

	Error string `json:"error"`
}

type SwapRatesRequest struct {
	Id string `json:"id"`
}

type SwapRatesResponse struct {
	OnchainAmount int `json:"onchainAmount"`
	SubmarineSwap struct {
		InvoiceAmount int `json:"invoiceAmount"`
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
	Type           string `json:"type"`
	PairId         string `json:"pairId"`
	OrderSide      string `json:"orderSide"`
	InvoiceAmount  int    `json:"invoiceAmount"`
	PreimageHash   string `json:"preimageHash"`
	ClaimPublicKey string `json:"claimPublicKey"`
}

type Channel struct {
	Auto             bool   `json:"auto"`
	Private          bool   `json:"private"`
	InboundLiquidity uint32 `json:"inboundLiquidity"`
}

type CreateChannelCreationRequest struct {
	Type            string `json:"type"`
	PairId          string `json:"pairId"`
	OrderSide       string `json:"orderSide"`
	RefundPublicKey string `json:"refundPublicKey"`
	Invoice         string `json:"invoice"`
	PreimageHash    string `json:"preimageHash"`

	Channel Channel `json:"channel"`
}

type CreateReverseSwapResponse struct {
	Id                 string `json:"id"`
	Invoice            string `json:"invoice"`
	OnchainAmount      int    `json:"onchainAmount"`
	RedeemScript       string `json:"redeemScript"`
	LockupAddress      string `json:"lockupAddress"`
	TimeoutBlockHeight int    `json:"TimeoutBlockHeight"`

	Error string `json:"error"`
}

func (boltz *Boltz) Init(symbol string) {
	boltz.symbol = symbol
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

func (boltz *Boltz) StreamSwapStatus(id string, events chan *SwapStatusResponse, stopListening chan bool) error {
	return streamSwapStatus(boltz.URL+"/streamswapstatus?id="+id, events, stopListening)
}

func (boltz *Boltz) GetSwapTransaction(id string) (*GetSwapTransactionResponse, error) {
	var response GetSwapTransactionResponse
	err := boltz.sendPostRequest("/getswaptransaction", GetSwapTransactionRequest{
		Id: id,
	}, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) BroadcastTransaction(transactionHex string) (*BroadcastTransactionResponse, error) {
	var response BroadcastTransactionResponse
	err := boltz.sendPostRequest("/broadcasttransaction", BroadcastTransactionRequest{
		Currency:       boltz.symbol,
		TransactionHex: transactionHex,
	}, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) CreateSwap(request CreateSwapRequest) (*CreateSwapResponse, error) {
	var response CreateSwapResponse
	err := boltz.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) SwapRates(request SwapRatesRequest) (*SwapRatesResponse, error) {
	var response SwapRatesResponse
	err := boltz.sendPostRequest("/swaprates", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) SetInvoice(request SetInvoiceRequest) (*SetInvoiceResponse, error) {
	var response SetInvoiceResponse
	err := boltz.sendPostRequest("/setinvoice", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (boltz *Boltz) CreateChannelCreation(request CreateChannelCreationRequest) (response *CreateSwapResponse, err error) {
	err = boltz.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return response, err
}

func (boltz *Boltz) CreateReverseSwap(request CreateReverseSwapRequest) (*CreateReverseSwapResponse, error) {
	var response CreateReverseSwapResponse
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
