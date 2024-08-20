package electrum

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/checksum0/go-electrum/electrum"
)

type Client struct {
	client *electrum.Client
	ctx    context.Context

	blockHeight uint32
}

func NewClient(options onchain.ElectrumOptions) (*Client, error) {
	// Establishing a new SSL connection to an ElectrumX server
	c := &Client{ctx: context.Background()}
	var err error
	if options.SSL {
		c.client, err = electrum.NewClientSSL(c.ctx, options.Url, &tls.Config{})
	} else {
		c.client, err = electrum.NewClientTCP(c.ctx, options.Url)
	}
	if err != nil {
		return nil, err
	}

	// Making sure connection is not closed with timed "client.ping" call
	go func() {
		for !c.client.IsShutdown() {
			ctx, cancel := c.timeoutContext()
			if err := c.client.Ping(ctx); err != nil {
				logger.Errorf("failed to ping electrum server: %s", err)
			}
			cancel()
			time.Sleep(60 * time.Second)
		}
	}()

	ctx, cancel := c.timeoutContext()
	defer cancel()
	// Making sure we declare to the server what protocol we want to use
	if _, _, err := c.client.ServerVersion(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) timeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.ctx, 3*time.Second)
}

func (c *Client) RegisterBlockListener(ctx context.Context, channel chan<- *onchain.BlockEpoch) error {
	results, err := c.client.SubscribeHeaders(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case result := <-results:
			c.blockHeight = uint32(result.Height)
			channel <- &onchain.BlockEpoch{Height: c.blockHeight}
		}
	}
}
func (c *Client) GetBlockHeight() (uint32, error) {
	return c.blockHeight, nil
}

func (c *Client) EstimateFee(confTarget int32) (float64, error) {
	ctx, cancel := c.timeoutContext()
	defer cancel()
	fee, err := c.client.GetFee(ctx, uint32(confTarget))
	return float64(fee), err
}

func (c *Client) GetRawTransaction(txId string) (string, error) {
	ctx, cancel := c.timeoutContext()
	defer cancel()
	return c.client.GetRawTransaction(ctx, txId)
}

func (c *Client) BroadcastTransaction(txHex string) (string, error) {
	ctx, cancel := c.timeoutContext()
	defer cancel()
	return c.client.BroadcastTransaction(ctx, txHex)
}

func (c *Client) Disconnect() {
	c.client.Shutdown()
}

func (c *Client) IsTransactionConfirmed(txId string) (bool, error) {
	ctx, cancel := c.timeoutContext()
	defer cancel()
	transaction, err := c.client.GetTransaction(ctx, txId)
	if err != nil {
		return false, err
	}
	return transaction.Confirmations > 0, nil
}
