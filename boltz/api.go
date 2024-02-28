package boltz

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Boltz struct {
	URL string `long:"boltz.url" description:"URL endpoint of the Boltz API"`

	DisablePartialSignatures bool
}

type SwapType string

const (
	NormalSwap  SwapType = "submarine"
	ReverseSwap SwapType = "reverseSubmarine"
)

func ParseSwapType(swapType string) (SwapType, error) {
	switch strings.ToLower(swapType) {
	case string(NormalSwap), "normal":
		return NormalSwap, nil
	case string(ReverseSwap), "reverse":
		return ReverseSwap, nil
	case "":
		return "", nil
	default:
		return "", errors.New("invalid swap type")
	}
}

type HexString []byte

func (s *HexString) UnmarshalText(data []byte) (err error) {
	*s, err = hex.DecodeString(string(data))
	return err
}

func (s HexString) MarshalText() ([]byte, error) {
	result := make([]byte, hex.EncodedLen(len(s)))
	hex.Encode(result, s)
	return result, nil
}

type Error error

// Types for Boltz API
type GetVersionResponse struct {
	Version string `json:"version"`
}

type SubmarinePair struct {
	Hash   string  `json:"hash"`
	Rate   float64 `json:"rate"`
	Limits struct {
		Minimal               uint64 `json:"minimal"`
		Maximal               uint64 `json:"maximal"`
		MaximalZeroConfAmount uint64 `json:"maximalZeroConfAmount"`
	} `json:"limits"`
	Fees struct {
		Percentage float64 `json:"percentage"`
		MinerFees  uint64  `json:"minerFees"`
	} `json:"fees"`
}

type SubmarinePairs map[Currency]map[Currency]SubmarinePair

type ReversePair struct {
	Hash   string  `json:"hash"`
	Rate   float64 `json:"rate"`
	Limits struct {
		Minimal uint64 `json:"minimal"`
		Maximal uint64 `json:"maximal"`
	} `json:"limits"`
	Fees struct {
		Percentage float64 `json:"percentage"`
		MinerFees  struct {
			Lockup uint64 `json:"lockup"`
			Claim  uint64 `json:"claim"`
		} `json:"minerFees"`
	} `json:"fees"`
}

type ReversePairs map[Currency]map[Currency]ReversePair

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

type NodeInfo struct {
	PublicKey string   `json:"publicKey"`
	Uris      []string `json:"uris"`
}

type Nodes = map[string]map[string]NodeInfo

type SwapStatusResponse struct {
	Status           string `json:"status"`
	ZeroConfRejected bool   `json:"zeroConfRejected"`
	Transaction      struct {
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

type GetTransactionRequest struct {
	Currency      string `json:"currency"`
	TransactionId string `json:"transactionId"`

	Error string `json:"error"`
}

type GetTransactionResponse struct {
	Hex string `json:"hex"`

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
	From            Currency  `json:"from"`
	To              Currency  `json:"to"`
	PairHash        string    `json:"pairHash,omitempty"`
	RefundPublicKey HexString `json:"refundPublicKey"`
	Invoice         string    `json:"invoice,omitempty"`
	ReferralId      string    `json:"referralId"`
	PreimageHash    HexString `json:"preimageHash,omitempty"`

	Error string `json:"error"`
}

type CreateSwapResponse struct {
	Id                 string          `json:"id"`
	Bip21              string          `json:"bip21"`
	Address            string          `json:"address"`
	SwapTree           *SerializedTree `json:"swapTree"`
	ClaimPublicKey     HexString       `json:"claimPublicKey"`
	TimeoutBlockHeight uint32          `json:"timeoutBlockHeight"`
	AcceptZeroConf     bool            `json:"acceptZeroConf"`
	ExpectedAmount     uint64          `json:"expectedAmount"`
	BlindingKey        HexString       `json:"blindingKey"`

	Error string `json:"error"`
}

type RefundSwapRequest struct {
	Id          string    `json:"id"`
	PubNonce    HexString `json:"pubNonce"`
	Transaction string    `json:"transaction"`
	Index       int       `json:"index"`
}

type SwapClaimDetails struct {
	PubNonce        HexString `json:"pubNonce"`
	TransactionHash HexString `json:"transactionHash"`
	Preimage        HexString `json:"preimage"`
	PublicKey       HexString `json:"publicKey"`

	Error string `json:"error"`
}

type GetInvoiceAmountResponse struct {
	InvoiceAmount uint64 `json:"invoiceAmount"`
	Error         string `json:"error"`
}

type SetInvoiceRequest struct {
	Invoice string `json:"invoice"`
}

type SetInvoiceResponse struct {
	Error string `json:"error"`
}

type CreateReverseSwapRequest struct {
	From           Currency  `json:"from"`
	To             Currency  `json:"to"`
	PreimageHash   HexString `json:"preimageHash"`
	ClaimPublicKey HexString `json:"claimPublicKey"`
	InvoiceAmount  uint64    `json:"invoiceAmount,omitempty"`
	OnchainAmount  uint64    `json:"onchainAmount,omitempty"`
	PairHash       string    `json:"pairHash,omitempty"`
	ReferralId     string    `json:"referralId"`

	Error string `json:"error"`
}

type CreateReverseSwapResponse struct {
	Id                 string          `json:"id"`
	Invoice            string          `json:"invoice"`
	SwapTree           *SerializedTree `json:"swapTree"`
	RefundPublicKey    HexString       `json:"refundPublicKey"`
	LockupAddress      string          `json:"lockupAddress"`
	TimeoutBlockHeight uint32          `json:"timeoutBlockHeight"`
	OnchainAmount      uint64          `json:"onchainAmount"`
	BlindingKey        HexString       `json:"blindingKey"`

	Error string `json:"error"`
}
type ClaimReverseSwapRequest struct {
	Id          string    `json:"id"`
	Preimage    HexString `json:"preimage"`
	PubNonce    HexString `json:"pubNonce"`
	Transaction string    `json:"transaction"`
	Index       int       `json:"index"`
}

type PartialSignature struct {
	PubNonce         HexString `json:"pubNonce"`
	PartialSignature HexString `json:"partialSignature"`

	Error string `json:"error"`
}

type ErrorMessage struct {
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

func (boltz *Boltz) GetSubmarinePairs() (response SubmarinePairs, err error) {
	err = boltz.sendGetRequest("/v2/swap/submarine", &response)

	return response, err
}

func (boltz *Boltz) GetReversePairs() (response ReversePairs, err error) {
	err = boltz.sendGetRequest("/v2/swap/reverse", &response)

	return response, err
}

func (boltz *Boltz) GetNodes() (Nodes, error) {
	var response Nodes
	err := boltz.sendGetRequest("/v2/nodes", &response)

	return response, err
}

func (boltz *Boltz) SwapStatus(id string) (*SwapStatusResponse, error) {
	var response SwapStatusResponse
	err := boltz.sendGetRequest("/v2/swap/"+id, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
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

func (boltz *Boltz) GetTransaction(transactionId string, currency Currency) (string, error) {
	var response GetTransactionResponse
	path := fmt.Sprintf("/v2/chain/%s/transaction/%s", currency, transactionId)
	err := boltz.sendGetRequest(path, &response)

	if response.Error != "" {
		return "", Error(errors.New(response.Error))
	}

	return response.Hex, err
}

func (boltz *Boltz) BroadcastTransaction(transaction Transaction) (*BroadcastTransactionResponse, error) {
	var response BroadcastTransactionResponse
	currency := CurrencyBtc
	if _, ok := transaction.(*LiquidTransaction); ok {
		currency = CurrencyLiquid
	}
	transactionHex, err := transaction.Serialize()
	if err != nil {
		return nil, fmt.Errorf("could not serialize transaction: %v", err)
	}
	err = boltz.sendPostRequest("/broadcasttransaction", BroadcastTransactionRequest{
		Currency:       string(currency),
		TransactionHex: transactionHex,
	}, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) CreateSwap(request CreateSwapRequest) (*CreateSwapResponse, error) {
	var response CreateSwapResponse
	err := boltz.sendPostRequest("/v2/swap/submarine", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) RefundSwap(request RefundSwapRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, errors.New("partial signatures are disabled")
	}
	var response PartialSignature
	err := boltz.sendPostRequest("/v2/swap/submarine/refund", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) GetInvoiceAmount(swapId string) (*GetInvoiceAmountResponse, error) {
	var response GetInvoiceAmountResponse
	err := boltz.sendGetRequest(fmt.Sprintf("/v2/swap/submarine/%s/invoice/amount", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) GetSwapClaimDetails(swapId string) (*SwapClaimDetails, error) {
	if boltz.DisablePartialSignatures {
		return nil, errors.New("partial signatures are disabled")
	}
	var response SwapClaimDetails
	err := boltz.sendGetRequest(fmt.Sprintf("/v2/swap/submarine/%s/claim", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) SendSwapClaimSignature(swapId string, signature *PartialSignature) error {
	var response ErrorMessage
	err := boltz.sendPostRequest(fmt.Sprintf("/v2/swap/submarine/%s/claim", swapId), signature, &response)

	if response.Error != "" {
		return Error(errors.New(response.Error))
	}

	return err
}

func (boltz *Boltz) SetInvoice(swapId string, invoice string) (*SetInvoiceResponse, error) {
	var response SetInvoiceResponse
	err := boltz.sendPostRequest(fmt.Sprintf("/v2/swap/submarine/%s/invoice", swapId), SetInvoiceRequest{Invoice: invoice}, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) CreateReverseSwap(request CreateReverseSwapRequest) (*CreateReverseSwapResponse, error) {
	var response CreateReverseSwapResponse
	err := boltz.sendPostRequest("/v2/swap/reverse", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Boltz) ClaimReverseSwap(request ClaimReverseSwapRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, errors.New("partial signatures are disabled")
	}
	var response PartialSignature
	err := boltz.sendPostRequest("/v2/swap/reverse/claim", request, &response)

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

	if err := unmarshalJson(res.Body, &response); err != nil {
		return fmt.Errorf("could not parse boltz response with status %d: %v", res.StatusCode, err)
	}
	return nil
}

func unmarshalJson(body io.ReadCloser, response interface{}) error {
	rawBody, err := io.ReadAll(body)

	if err != nil {
		return err
	}

	return json.Unmarshal(rawBody, &response)
}
