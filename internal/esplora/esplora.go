package esplora

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
)

type transactionStatus struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight uint32 `json:"block_height,omitempty"`
	BlockHash   string `json:"block_hash,omitempty"`
	BlockTime   uint64 `json:"block_time,omitempty"`
}

type utxo struct {
	TxId   string `json:"txid"`
	Vout   uint32 `json:"vout"`
	Status struct {
		Confirmed bool `json:"confirmed"`
	} `json:"status"`
	Value uint64 `json:"value"`
}

type Client struct {
	httpClient *http.Client
	api        string
}

// InitClient creates a new Esplora API client
func InitClient(endpoint string) *Client {
	endpointStripped := strings.TrimSuffix(endpoint, "/")

	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		api: endpointStripped,
	}
}

func (c *Client) get(path string) ([]byte, error) {
	res, err := c.httpClient.Get(c.api + path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s status %d: %s", path, res.StatusCode, string(body))
	}

	return body, nil
}

// getJson performs a GET request and decodes the JSON response into dest
func (c *Client) getJson(path string, dest any) error {
	body, err := c.get(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}

	return nil
}

func (c *Client) EstimateFee() (float64, error) {
	// esplora has really bad fee estimation
	return 0, errors.ErrUnsupported
}

func (c *Client) GetBlockHeight() (uint32, error) {
	raw, err := c.get("/blocks/tip/height")
	if err != nil {
		return 0, err
	}

	height, err := strconv.ParseUint(string(raw), 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(height), nil
}

func (c *Client) GetRawTransaction(txId string) (string, error) {
	hex, err := c.get("/tx/" + txId + "/hex")
	if err != nil {
		return "", err
	}

	return string(hex), nil
}

func (c *Client) BroadcastTransaction(txHex string) (string, error) {
	res, err := c.httpClient.Post(c.api+"/tx", "text/plain", strings.NewReader(txHex))
	if err != nil {
		return "", err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("could not broadcast tx, failed with code %d: %s", res.StatusCode, string(body))
	}

	id, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(id), nil
}

func (c *Client) IsTransactionConfirmed(txId string) (bool, error) {
	var status transactionStatus
	if err := c.getJson("/tx/"+txId+"/status", &status); err != nil {
		return false, err
	}

	return status.Confirmed, nil
}

func (c *Client) GetUnspentOutputs(address string) ([]*onchain.Output, error) {
	var utxos []utxo
	if err := c.getJson("/address/"+address+"/utxo", &utxos); err != nil {
		return nil, err
	}

	result := make([]*onchain.Output, 0, len(utxos))
	for _, u := range utxos {
		result = append(result, &onchain.Output{
			TxId:  u.TxId,
			Value: u.Value,
		})
	}

	return result, nil
}

// Disconnect closes any resources held by the client
// For HTTP client, there's no persistent connection to close
func (c *Client) Disconnect() {
	c.httpClient.CloseIdleConnections()
}
