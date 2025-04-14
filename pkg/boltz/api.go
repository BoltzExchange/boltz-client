package boltz

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/exp/constraints"
)

type Api struct {
	URL      string
	Client   http.Client
	Referral string

	DisablePartialSignatures bool
}

type SwapType string

const (
	NormalSwap  SwapType = "submarine"
	ReverseSwap SwapType = "reverse"
	ChainSwap   SwapType = "chain"
)

var ErrPartialSignaturesDisabled = errors.New("partial signatures are disabled")

func ParseSwapType(swapType string) (SwapType, error) {
	switch strings.ToLower(swapType) {
	case string(NormalSwap), "normal":
		return NormalSwap, nil
	case string(ReverseSwap):
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

type ResponseError struct {
	Error string `json:"error"`
}

func (response ResponseError) ApiError(err error) error {
	if response.Error != "" {
		return Error(errors.New(response.Error))
	}
	return err
}

func StripQuotes(text []byte) string {
	return string(bytes.Trim(text, "\""))
}

type Percentage float64

func (p Percentage) String() string {
	return fmt.Sprintf("%.2f%%", float64(p))
}

func (p Percentage) Ratio() float64 {
	return float64(p / 100)
}

func (p Percentage) Calculate(value uint64) uint64 {
	return uint64(math.Ceil(float64(value) * p.Ratio()))
}

func (p *Percentage) UnmarshalJSON(text []byte) error {
	str := StripQuotes(text)

	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	*p = Percentage(parsed)
	return nil
}

func CalculatePercentage[T constraints.Integer](p Percentage, value T) T {
	return T(math.Ceil(float64(value) * p.Ratio()))
}

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
		MaximalZeroConfAmount uint64 `json:"maximalZeroConf"`
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

type ChainPair struct {
	Hash   string  `json:"hash"`
	Rate   float64 `json:"rate"`
	Limits struct {
		Minimal               uint64 `json:"minimal"`
		Maximal               uint64 `json:"maximal"`
		MaximalZeroConfAmount uint64 `json:"maximalZeroConf"`
	} `json:"limits"`
	Fees struct {
		Percentage float64 `json:"percentage"`
		MinerFees  struct {
			Server uint64 `json:"server"`
			User   struct {
				Claim  uint64 `json:"claim"`
				Lockup uint64 `json:"lockup"`
			} `json:"user"`
		} `json:"minerFees"`
	} `json:"fees"`
}

type ChainPairs map[Currency]map[Currency]ChainPair

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

type GetSwapTransactionResponse struct {
	Id                 string `json:"id"`
	Hex                string `json:"hex"`
	TimeoutBlockHeight uint32 `json:"timeoutBlockHeight"`
	TimeoutEta         uint64 `json:"timeoutEta"`

	Error string `json:"error"`
}

type ChainSwapTransaction struct {
	Transaction struct {
		Id  string `json:"id"`
		Hex string `json:"hex"`
	} `json:"transaction"`
}

type GetChainSwapTransactionsResponse struct {
	UserLock   *ChainSwapTransaction `json:"userLock"`
	ServerLock *ChainSwapTransaction `json:"serverLock"`

	Error string `json:"error"`
}

type GetTransactionRequest struct {
	Currency      string `json:"currency"`
	TransactionId string `json:"transactionId"`

	Error string `json:"error"`
}

type GetTransactionResponse struct {
	Hex           string `json:"hex"`
	Confirmations uint64 `json:"confirmations"`

	Error string `json:"error"`
}

type BroadcastTransactionRequest struct {
	Hex string `json:"hex"`
}

type BroadcastTransactionResponse struct {
	Id string `json:"id"`

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

type RefundRequest struct {
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

type ChainSwapSigningDetails struct {
	PubNonce        HexString `json:"pubNonce"`
	TransactionHash HexString `json:"transactionHash"`
	PublicKey       HexString `json:"publicKey"`

	Error string `json:"error"`
}

type ChainSwapSigningRequest struct {
	Preimage  HexString         `json:"preimage"`
	Signature *PartialSignature `json:"signature"`
	ToSign    *ClaimRequest     `json:"toSign"`

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
	From             Currency  `json:"from"`
	To               Currency  `json:"to"`
	PreimageHash     HexString `json:"preimageHash"`
	ClaimPublicKey   HexString `json:"claimPublicKey"`
	InvoiceAmount    uint64    `json:"invoiceAmount,omitempty"`
	OnchainAmount    uint64    `json:"onchainAmount,omitempty"`
	PairHash         string    `json:"pairHash,omitempty"`
	ReferralId       string    `json:"referralId"`
	Address          string    `json:"address,omitempty"`
	AddressSignature HexString `json:"addressSignature,omitempty"`
	Description      string    `json:"description,omitempty"`
	DescriptionHash  HexString `json:"descriptionHash,omitempty"`
	InvoiceExpiry    uint64    `json:"invoiceExpiry,omitempty"`

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
type ClaimRequest struct {
	Preimage    HexString `json:"preimage"`
	PubNonce    HexString `json:"pubNonce"`
	Transaction string    `json:"transaction"`
	Index       int       `json:"index"`
}

type ChainRequest struct {
	From             Currency  `json:"from"`
	To               Currency  `json:"to"`
	PreimageHash     HexString `json:"preimageHash"`
	ClaimPublicKey   HexString `json:"claimPublicKey,omitempty"`
	RefundPublicKey  HexString `json:"refundPublicKey,omitempty"`
	UserLockAmount   uint64    `json:"userLockAmount,omitempty"`
	ServerLockAmount uint64    `json:"serverLockAmount,omitempty"`
	PairHash         string    `json:"pairHash,omitempty"`
	ReferralId       string    `json:"referralId,omitempty"`
}

type ChainResponse struct {
	Id            string         `json:"id"`
	ClaimDetails  *ChainSwapData `json:"claimDetails,omitempty"`
	LockupDetails *ChainSwapData `json:"lockupDetails,omitempty"`

	Error string `json:"error"`
}

type ChainSwapData struct {
	SwapTree           *SerializedTree `json:"swapTree,omitempty"`
	LockupAddress      string          `json:"lockupAddress"`
	ServerPublicKey    HexString       `json:"serverPublicKey,omitempty"`
	TimeoutBlockHeight uint32          `json:"timeoutBlockHeight"`
	Amount             uint64          `json:"amount"`
	BlindingKey        HexString       `json:"blindingKey,omitempty"`
	Bip21              string          `json:"bip21,omitempty"`
}

type PartialSignature struct {
	PubNonce         HexString `json:"pubNonce"`
	PartialSignature HexString `json:"partialSignature"`

	Error string `json:"error"`
}

type ErrorMessage struct {
	Error string `json:"error"`
}

func (boltz *Api) GetVersion() (*GetVersionResponse, error) {
	var response GetVersionResponse
	err := boltz.sendGetRequestV2("/version", &response)

	return &response, err
}

// Deprecated: use GetSubmarinePairs, GetChainPairs or GetReversePairs instead
func (boltz *Api) GetPairs() (*GetPairsResponse, error) {
	var response GetPairsResponse
	err := boltz.sendGetRequest("/getpairs", &response)

	return &response, err
}

func (boltz *Api) GetFeeEstimation(currency Currency) (float64, error) {
	var response struct {
		Fee float64
	}
	err := boltz.sendGetRequestV2(fmt.Sprintf("/chain/%s/fee", currency), &response)

	return response.Fee, err
}

func (boltz *Api) GetSubmarinePairs() (response SubmarinePairs, err error) {
	err = boltz.sendGetRequestV2("/swap/submarine", &response)

	return response, err
}

func (boltz *Api) GetReversePairs() (response ReversePairs, err error) {
	err = boltz.sendGetRequestV2("/swap/reverse", &response)

	return response, err
}

func (boltz *Api) GetChainPairs() (response ChainPairs, err error) {
	err = boltz.sendGetRequestV2("/swap/chain", &response)

	return response, err
}

func (boltz *Api) GetNodes() (Nodes, error) {
	var response Nodes
	err := boltz.sendGetRequestV2("/nodes", &response)

	return response, err
}

func (boltz *Api) SwapStatus(id string) (*SwapStatusResponse, error) {
	var response SwapStatusResponse
	err := boltz.sendGetRequestV2("/swap/"+id, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetSwapTransaction(id string) (*GetSwapTransactionResponse, error) {
	var response GetSwapTransactionResponse
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/submarine/%s/transaction", id), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetChainSwapTransactions(id string) (*GetChainSwapTransactionsResponse, error) {
	var response GetChainSwapTransactionsResponse
	path := fmt.Sprintf("/swap/chain/%s/transactions", id)
	err := boltz.sendGetRequestV2(path, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetTransactionDetails(transactionId string, currency Currency) (*GetTransactionResponse, error) {
	var response GetTransactionResponse
	path := fmt.Sprintf("/chain/%s/transaction/%s", currency, transactionId)
	err := boltz.sendGetRequestV2(path, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetTransaction(transactionId string, currency Currency) (string, error) {
	response, err := boltz.GetTransactionDetails(transactionId, currency)
	if err != nil {
		return "", err
	}

	return response.Hex, err
}

func (boltz *Api) BroadcastTransaction(currency Currency, txHex string) (string, error) {
	var response BroadcastTransactionResponse
	err := boltz.sendPostRequest(fmt.Sprintf("/chain/%s/transaction", currency), BroadcastTransactionRequest{
		Hex: txHex,
	}, &response)

	if response.Error != "" {
		return "", Error(errors.New(response.Error))
	}

	return response.Id, err
}

func (boltz *Api) CreateSwap(request CreateSwapRequest) (*CreateSwapResponse, error) {
	var response CreateSwapResponse
	err := boltz.sendPostRequest("/swap/submarine", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) RefundSwap(swapId string, request *RefundRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response PartialSignature
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/submarine/%s/refund", swapId), request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetInvoiceAmount(swapId string) (*GetInvoiceAmountResponse, error) {
	var response GetInvoiceAmountResponse
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/submarine/%s/invoice/amount", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetSwapClaimDetails(swapId string) (*SwapClaimDetails, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response SwapClaimDetails
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/submarine/%s/claim", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) SendSwapClaimSignature(swapId string, signature *PartialSignature) error {
	var response ErrorMessage
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/submarine/%s/claim", swapId), signature, &response)

	if response.Error != "" {
		return Error(errors.New(response.Error))
	}

	return err
}

func (boltz *Api) SetInvoice(swapId string, invoice string) (*SetInvoiceResponse, error) {
	var response SetInvoiceResponse
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/submarine/%s/invoice", swapId), SetInvoiceRequest{Invoice: invoice}, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) CreateReverseSwap(request CreateReverseSwapRequest) (*CreateReverseSwapResponse, error) {
	var response CreateReverseSwapResponse
	err := boltz.sendPostRequest("/swap/reverse", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) ClaimReverseSwap(swapId string, request *ClaimRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response PartialSignature
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/reverse/%s/claim", swapId), request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) CreateChainSwap(request ChainRequest) (*ChainResponse, error) {
	var response ChainResponse
	err := boltz.sendPostRequest("/swap/chain", request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) GetChainSwapClaimDetails(swapId string) (*ChainSwapSigningDetails, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response ChainSwapSigningDetails
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/chain/%s/claim", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) ExchangeChainSwapClaimSignature(swapId string, request *ChainSwapSigningRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response PartialSignature
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/chain/%s/claim", swapId), request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) RefundChainSwap(swapId string, request *RefundRequest) (*PartialSignature, error) {
	if boltz.DisablePartialSignatures {
		return nil, ErrPartialSignaturesDisabled
	}
	var response PartialSignature
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/chain/%s/refund", swapId), request, &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

type ReverseBip21 struct {
	Bip21     string    `json:"bip21"`
	Signature HexString `json:"signature"`

	Error string `json:"error"`
}

func (boltz *Api) GetReverseSwapBip21(invoice string) (*ReverseBip21, error) {
	var response ReverseBip21
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/reverse/%s/bip21", invoice), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

type Quote struct {
	ResponseError
	Amount uint64 `json:"amount"`
}

func (boltz *Api) GetChainSwapQuote(swapId string) (*Quote, error) {
	var response Quote
	err := boltz.sendGetRequestV2(fmt.Sprintf("/swap/chain/%s/quote", swapId), &response)

	if response.Error != "" {
		return nil, Error(errors.New(response.Error))
	}

	return &response, err
}

func (boltz *Api) AcceptChainSwapQuote(swapId string, quote *Quote) error {
	var response ResponseError
	err := boltz.sendPostRequest(fmt.Sprintf("/swap/chain/%s/quote", swapId), quote, &response)

	return response.ApiError(err)
}

func (boltz *Api) FetchBolt12Invoice(offer string, amountSat uint64) (string, error) {
	var response struct {
		Invoice string `json:"invoice"`
	}
	request := struct {
		Offer  string `json:"offer"`
		Amount uint64 `json:"amount"`
	}{
		Offer:  offer,
		Amount: amountSat,
	}
	err := boltz.sendPostRequest("/lightning/BTC/bolt12/fetch", &request, &response)

	return response.Invoice, err
}

func (boltz *Api) sendGetRequest(endpoint string, response interface{}) error {
	request, err := http.NewRequest("GET", boltz.URL+endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Referral", boltz.Referral)

	res, err := boltz.Client.Do(request)

	if err != nil {
		return err
	}

	return unmarshalJson(res.Body, &response)
}

func (boltz *Api) sendGetRequestV2(endpoint string, response interface{}) error {
	return boltz.sendGetRequest("/v2"+endpoint, response)
}

func (boltz *Api) sendPostRequest(endpoint string, requestBody interface{}, response interface{}) error {
	rawBody, err := json.Marshal(requestBody)

	if err != nil {
		return err
	}

	res, err := boltz.Client.Post(boltz.URL+"/v2"+endpoint, "application/json", bytes.NewBuffer(rawBody))

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
