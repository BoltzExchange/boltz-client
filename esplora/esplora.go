package esplora

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
)

type Client struct {
	api       string
	pollDelay time.Duration
}

func InitClient(endpoint string, pollDelay time.Duration) *Client {
	endpointStripped := strings.TrimSuffix(endpoint, "/")
	return &Client{
		api:       endpointStripped,
		pollDelay: pollDelay,
	}
}

func (c *Client) EstimateFee() (float64, error) {
	res, err := http.Get(c.api + "/api/fee-estimates")
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed with status: %d", res.StatusCode)
	}

	var fees map[string]float64
	err = json.NewDecoder(res.Body).Decode(&fees)
	if err != nil {
		return 0, err
	}

	return fees["1"], nil
}

func (c *Client) GetRawTransaction(txId string) (string, error) {
	res, err := http.Get(c.api + "/api/tx/" + txId + "/hex")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not get tx %s, failed with status: %d", txId, res.StatusCode)
	}

	hex, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(hex), nil
}

func (c *Client) IsTransactionConfirmed(txId string) (bool, error) {
	res, err := http.Get(c.api + "/api/tx/" + txId)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("could not get transaction %s, failed with status: %d", txId, res.StatusCode)
	}

	var transaction struct {
		Status struct {
			Confirmed bool `json:"confirmed"`
		} `json:"status"`
	}

	if err := json.NewDecoder(res.Body).Decode(&transaction); err != nil {
		return false, err
	}

	return transaction.Status.Confirmed, nil
}

func (c *Client) BroadcastTransaction(txHex string) (string, error) {
	res, err := http.Post(c.api+"/api/tx", "text/plain", strings.NewReader(txHex))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not post tx, failed with code %d", res.StatusCode)
	}

	id, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(id), nil
}

func (c *Client) GetUnspentOutputs(address string) ([]*onchain.Output, error) {
	res, err := http.Get(c.api + "/api/address/" + address + "/utxo")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get UTXOs for address %s, failed with status: %d", address, res.StatusCode)
	}

	var utxos []struct {
		TxId  string `json:"txid"`
		Vout  uint32 `json:"vout"`
		Value uint64 `json:"value"`
	}

	if err := json.NewDecoder(res.Body).Decode(&utxos); err != nil {
		return nil, err
	}

	var outputs []*onchain.Output
	for _, utxo := range utxos {
		outputs = append(outputs, &onchain.Output{
			TxId:  utxo.TxId,
			Value: utxo.Value,
		})
	}

	return outputs, nil
}

func (c *Client) GetBlockHeight() (uint32, error) {
	res, err := http.Get(c.api + "/api/blocks/tip/height")
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("could not get block height, failed with status: %d", res.StatusCode)
	}

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	height, err := strconv.ParseUint(string(raw), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(height), nil
}

func (c *Client) RegisterBlockListener(ctx context.Context, channel chan<- *onchain.BlockEpoch) error {
	var lastHeight uint32

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			currentHeight, err := c.GetBlockHeight()
			if err != nil {
				logger.Error("Error fetching block height: " + err.Error())
				time.Sleep(c.pollDelay)
				continue
			}

			if currentHeight > lastHeight {
				logger.Info(fmt.Sprintf("New block detected: %d", currentHeight))
				lastHeight = currentHeight
				channel <- &onchain.BlockEpoch{
					Height: currentHeight,
				}
			}

			time.Sleep(c.pollDelay)
		}
	}
}

func (c *Client) Disconnect() {}
