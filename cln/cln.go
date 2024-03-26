package cln

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"

	"github.com/BoltzExchange/boltz-client/cln/protos"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/onchain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Cln struct {
	Host    string `long:"cln.host" description:"gRPC host of the CLN daemon"`
	Port    int    `long:"cln.port" description:"gRPC port of the CLN daemon"`
	DataDir string `long:"cln.datadir" description:"Path to the data directory of CLN"`

	RootCert   string `long:"cln.rootcert" description:"Path to the root cert of the CLN gRPC"`
	PrivateKey string `long:"cln.privatekey" description:"Path to the client key of the CLN gRPC"`
	CertChain  string `long:"cln.certchain" description:"Path to the client cert of the CLN gRPC"`

	Client protos.NodeClient

	// id which is used to satisfy wallet interface
	regtest    bool
	walletInfo onchain.WalletInfo
}

const (
	serviceName = lightning.NodeTypeCln

	paymentFailure = "payment failed"
)

var (
	ErrPaymentNotInitiated = errors.New("payment not initialized")

	paymentStatusFromGrpc = map[protos.ListpaysPays_ListpaysPaysStatus]lightning.PaymentState{
		protos.ListpaysPays_PENDING:  lightning.PaymentPending,
		protos.ListpaysPays_COMPLETE: lightning.PaymentSucceeded,
		protos.ListpaysPays_FAILED:   lightning.PaymentFailed,
	}
)

func (c *Cln) Ready() bool {
	return c.Client != nil
}

func (c *Cln) Readonly() bool {
	return false
}

func (c *Cln) Currency() boltz.Currency {
	return boltz.CurrencyBtc
}

func (c *Cln) Connect() error {
	caFile, err := os.ReadFile(c.RootCert)
	if err != nil {
		return fmt.Errorf("could not read %s root certificate %s: %s", serviceName, c.RootCert, err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caFile) {
		return fmt.Errorf("could not parse %s root certificate", serviceName)
	}

	cert, err := tls.LoadX509KeyPair(c.CertChain, c.PrivateKey)
	if err != nil {
		return fmt.Errorf("could not read %s client certificate: %s", serviceName, err)
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "cln",
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
	})

	con, err := grpc.Dial(c.Host+":"+strconv.Itoa(c.Port), grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("could not create %s gRPC client: %s", serviceName, err)
	}

	c.Client = protos.NewNodeClient(con)
	return nil
}

func (c *Cln) Name() string {
	return "CLN"
}

func (c *Cln) GetWalletInfo() onchain.WalletInfo {
	return c.walletInfo
}

func (c *Cln) SetId(id int64) {
	c.walletInfo = onchain.WalletInfo{
		Id:       id,
		Readonly: false,
		Name:     c.Name(),
		Currency: boltz.CurrencyBtc,
	}
}

func (c *Cln) NodeType() lightning.LightningNodeType {
	return serviceName
}

func (c *Cln) GetInfo() (*lightning.LightningInfo, error) {
	info, err := c.Client.Getinfo(context.Background(), &protos.GetinfoRequest{})
	if err != nil {
		return nil, err
	}
	c.regtest = info.Network == "regtest"
	return &lightning.LightningInfo{
		Pubkey:      hex.EncodeToString(info.Id),
		BlockHeight: info.Blockheight,
		Version:     info.Version,
		Network:     info.Network,
		Synced:      (info.WarningBitcoindSync == nil || *info.WarningBitcoindSync == "null") && (info.WarningLightningdSync == nil || *info.WarningLightningdSync == "null"),
	}, nil
}

func (c *Cln) GetBlockHeight() (uint32, error) {
	info, err := c.GetInfo()
	if err != nil {
		return 0, err
	}
	return info.BlockHeight, nil
}

func (c *Cln) ListChannels() ([]*lightning.LightningChannel, error) {
	funds, err := c.Client.ListFunds(context.Background(), &protos.ListfundsRequest{})

	if err != nil {
		return nil, err
	}

	var results []*lightning.LightningChannel

	for _, channel := range funds.Channels {
		chanId, err := lightning.NewChanIdFromString(*channel.ShortChannelId)
		if err != nil {
			logger.Warnf("Could not parse cln channel id %s: %v", *channel.ShortChannelId, err)
			continue
		}

		results = append(results, &lightning.LightningChannel{
			LocalSat:  channel.OurAmountMsat.Msat / 1000,
			RemoteSat: (channel.AmountMsat.Msat - channel.OurAmountMsat.Msat) / 1000,
			Capacity:  channel.AmountMsat.Msat / 1000,
			Id:        chanId,
			PeerId:    hex.EncodeToString(channel.PeerId),
			Point: lightning.ChannelPoint{
				FundingTxId: hex.EncodeToString(channel.FundingTxid),
				OutputIndex: channel.FundingOutput,
			},
		})

	}
	return results, nil
}

func (c *Cln) SanityCheck() (string, error) {
	info, err := c.Client.Getinfo(context.Background(), &protos.GetinfoRequest{})
	if err != nil {
		return "", err
	}

	return info.Version, nil
}

func (c *Cln) CreateInvoice(value int64, preimage []byte, expiry int64, memo string) (*lightning.AddInvoiceResponse, error) {
	request := &protos.InvoiceRequest{
		// wtf is this
		AmountMsat: &protos.AmountOrAny{
			Value: &protos.AmountOrAny_Amount{
				Amount: &protos.Amount{
					Msat: uint64(value) * 1000,
				},
			},
		},
		Preimage:    preimage,
		Description: memo,
		Label:       fmt.Sprint(time.Now().UTC().UnixMilli()),
	}
	if c.regtest {
		cltv := uint32(lightning.RegtestCltv)
		request.Cltv = &cltv
	}
	if expiry != 0 {
		expiryDate := uint64(time.Now().Unix()) + uint64(expiry)
		request.Expiry = &expiryDate
	}
	invoice, err := c.Client.Invoice(context.Background(), request)

	if err != nil {
		return nil, err
	}

	return &lightning.AddInvoiceResponse{
		PaymentRequest: invoice.Bolt11,
		PaymentHash:    invoice.PaymentHash,
	}, nil
}

func (c *Cln) PayInvoice(invoice string, feeLimit uint, timeoutSeconds uint, chanIds []lightning.ChanId) (*lightning.PayInvoiceResponse, error) {
	retry := uint32(timeoutSeconds)

	var exclude []string

	if len(chanIds) > 0 {
		channels, err := c.ListChannels()
		if err != nil {
			return nil, err
		}
		for _, channel := range channels {
			if !slices.Contains(chanIds, channel.Id) {
				exclude = append(exclude, channel.Id.ToCln()+"/0")
				exclude = append(exclude, channel.Id.ToCln()+"/1")
			}
		}
	}

	res, err := c.Client.Pay(context.Background(), &protos.PayRequest{
		Bolt11:   invoice,
		RetryFor: &retry,
		Maxfee: &protos.Amount{
			Msat: uint64(feeLimit),
		},
		Exclude: exclude,
	})

	if err != nil {
		return nil, err
	}

	return &lightning.PayInvoiceResponse{
		FeeMsat: uint(res.AmountSentMsat.Msat - res.AmountMsat.Msat),
	}, nil
}

func (c *Cln) ConnectPeer(uri string) error {
	_, err := c.Client.ConnectPeer(context.Background(), &protos.ConnectRequest{
		Id: uri,
	})
	return err
}

func (c *Cln) NewAddress() (string, error) {
	res, err := c.Client.NewAddr(context.Background(), &protos.NewaddrRequest{
		//Addresstype: &protos.NewaddrRequest_BECH32,
	})
	if err != nil {
		return "", err
	}
	return *res.Bech32, nil
}

func (c *Cln) CheckInvoicePaid(paymentHash []byte) (bool, error) {
	res, err := c.Client.ListInvoices(context.Background(), &protos.ListinvoicesRequest{
		PaymentHash: paymentHash,
	})
	if err != nil {
		return false, err
	}

	if len(res.Invoices) == 0 {
		return false, nil
	}

	for _, invoice := range res.Invoices {
		if invoice.Status == *protos.ListinvoicesInvoices_PAID.Enum() {
			return true, nil
		}
	}
	return false, nil
}

func (c *Cln) PaymentStatus(paymentHash []byte) (*lightning.PaymentStatus, error) {
	res, err := c.Client.ListPays(context.Background(), &protos.ListpaysRequest{
		PaymentHash: paymentHash,
	})
	if err != nil {
		return nil, err
	}

	if len(res.Pays) == 0 {
		return nil, ErrPaymentNotInitiated
	}

	status := res.Pays[len(res.Pays)-1]

	// ListPays doesn't give a proper reason
	var failureReason string
	if status.Status == protos.ListpaysPays_FAILED {
		failureReason = paymentFailure
	}

	return &lightning.PaymentStatus{
		State:         paymentStatusFromGrpc[status.Status],
		FailureReason: failureReason,
		Preimage:      encodeOptionalBytes(status.Preimage),
		FeeMsat:       parseFeeMsat(status.AmountMsat, status.AmountSentMsat),
	}, nil
}

func (c *Cln) RegisterBlockListener(channel chan<- *onchain.BlockEpoch, stop <-chan bool) error {
	info, err := c.GetInfo()
	if err != nil {
		return err
	}
	blockheight := info.BlockHeight
	var interval time.Duration
	if info.Network != "regtest" {
		interval = 1 * time.Minute
	} else {
		interval = 1 * time.Second
	}
	ticker := time.NewTicker(interval)

	for range ticker.C {
		select {
		case <-ticker.C:
			// FIXME: wait for new block with rpc: see https://github.com/ElementsProject/lightning/issues/6735
			info, err := c.GetInfo()
			if err != nil {
				return err
			}

			if info.BlockHeight > blockheight {
				blockheight = info.BlockHeight
				channel <- &onchain.BlockEpoch{
					Height: blockheight,
				}
			}
		case <-stop:
			return nil
		}
	}

	return err
}

func (c *Cln) GetBalance() (*onchain.Balance, error) {
	response, err := c.Client.ListFunds(context.Background(), &protos.ListfundsRequest{})
	if err != nil {
		return nil, err
	}
	balance := &onchain.Balance{}
	for _, output := range response.Outputs {
		amount := output.AmountMsat.Msat / 1000
		if output.Status == *protos.ListfundsOutputs_CONFIRMED.Enum() {
			balance.Confirmed += amount
		} else if output.Status == *protos.ListfundsOutputs_UNCONFIRMED.Enum() {
			balance.Unconfirmed += amount
		}
		balance.Total += amount
	}
	return balance, err
}

func (c *Cln) SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error) {
	response, err := c.Client.Withdraw(context.Background(), &protos.WithdrawRequest{
		Destination: address,
		Satoshi: &protos.AmountOrAll{
			Value: &protos.AmountOrAll_Amount{
				Amount: &protos.Amount{
					Msat: amount * 1000,
				},
			},
		},
		Feerate: &protos.Feerate{
			Style: &protos.Feerate_Perkb{
				// TODO: check this is correct
				Perkb: uint32(satPerVbyte * 1000),
			},
		},
	})

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(response.GetTxid()), nil

}

func (c *Cln) EstimateFee(confTarget int32) (float64, error) {
	response, err := c.Client.Feerates(context.Background(), &protos.FeeratesRequest{})
	if err != nil {
		return 0, err
	}

	for _, estimate := range response.Perkb.Estimates {
		if *estimate.Blockcount >= uint32(confTarget) {
			return float64(*estimate.Feerate) / 1000, nil
		}
	}
	return 0, errors.New("could not find fee estimate")
}

func encodeOptionalBytes(data []byte) string {
	if data == nil {
		return ""
	}

	return hex.EncodeToString(data)
}

func parseFeeMsat(amountMsat *protos.Amount, amountSentMsat *protos.Amount) uint64 {
	if amountMsat == nil || amountSentMsat == nil {
		return 0
	}

	return amountSentMsat.Msat - amountMsat.Msat
}
