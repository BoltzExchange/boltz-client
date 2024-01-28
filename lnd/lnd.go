package lnd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type LightningClient interface {
	GetInfo() (*lnrpc.GetInfoResponse, error)
	GetNodeInfo(pubkey string) (*lnrpc.NodeInfo, error)
	ListChannels() (*lnrpc.ListChannelsResponse, error)
	ClosedChannels() (*lnrpc.ClosedChannelsResponse, error)
	GetChannelInfo(chanId uint64) (*lnrpc.ChannelEdge, error)
	ListInactiveChannels() (*lnrpc.ListChannelsResponse, error)
	ForceCloseChannel(channelPoint string) (lnrpc.Lightning_CloseChannelClient, error)
}

var (
	paymentStatusFromGrpc = map[lnrpc.Payment_PaymentStatus]lightning.PaymentState{
		lnrpc.Payment_IN_FLIGHT: lightning.PaymentPending,
		lnrpc.Payment_SUCCEEDED: lightning.PaymentSucceeded,
		lnrpc.Payment_FAILED:    lightning.PaymentFailed,
		lnrpc.Payment_UNKNOWN:   lightning.PaymentFailed,
	}
)

type LND struct {
	Host        string `long:"lnd.host" description:"gRPC host of the LND node"`
	Port        int    `long:"lnd.port" description:"gRPC port of the LND node"`
	Macaroon    string `long:"lnd.macaroon" description:"Path to a macaroon file of the LND node"`
	Certificate string `long:"lnd.certificate" description:"Path to a certificate file of the LND node"`

	ChainParams *chaincfg.Params

	ctx context.Context

	client        lnrpc.LightningClient
	router        routerrpc.RouterClient
	invoices      invoicesrpc.InvoicesClient
	walletKit     walletrpc.WalletKitClient
	chainNotifier chainrpc.ChainNotifierClient
}

func (lnd *LND) Name() string {
	return "LND"
}

func (lnd *LND) Ready() bool {
	return lnd.client != nil
}

func (lnd *LND) Readonly() bool {
	return false
}

func (lnd *LND) Currency() boltz.Currency {
	return boltz.CurrencyBtc
}

func (lnd *LND) Connect() error {
	cert, err := filepath.EvalSymlinks(lnd.Certificate)
	if err != nil {
		return errors.New(fmt.Sprint("could not eval symlinks: ", err))
	}
	creds, err := credentials.NewClientTLSFromFile(cert, "")

	if err != nil {
		return errors.New(fmt.Sprint("could not read LND certificate: ", err))
	}

	con, err := grpc.Dial(lnd.Host+":"+strconv.Itoa(lnd.Port), grpc.WithTransportCredentials(creds))

	if err != nil {
		return errors.New(fmt.Sprint("could not create gRPC client: ", err))
	}

	lnd.client = lnrpc.NewLightningClient(con)
	lnd.router = routerrpc.NewRouterClient(con)
	lnd.invoices = invoicesrpc.NewInvoicesClient(con)
	lnd.walletKit = walletrpc.NewWalletKitClient(con)
	lnd.chainNotifier = chainrpc.NewChainNotifierClient(con)

	if lnd.ctx == nil {
		macaroonFile, err := os.ReadFile(lnd.Macaroon)

		if err != nil {
			return errors.New(fmt.Sprint("could not read LND macaroon: ", err))
		}

		macaroon := metadata.Pairs("macaroon", hex.EncodeToString(macaroonFile))
		lnd.ctx = metadata.NewOutgoingContext(context.Background(), macaroon)
	}

	return nil
}

func (lnd *LND) getInfo() (*lnrpc.GetInfoResponse, error) {
	return lnd.client.GetInfo(lnd.ctx, &lnrpc.GetInfoRequest{})
}

func (lnd *LND) GetInfo() (*lightning.LightningInfo, error) {
	info, err := lnd.getInfo()
	if err != nil {
		return nil, err
	}
	return &lightning.LightningInfo{
		Pubkey:      info.IdentityPubkey,
		BlockHeight: info.BlockHeight,
		Version:     info.Version,
		Network:     info.Chains[0].Network,
		Synced:      info.SyncedToChain,
	}, nil
}

func (lnd *LND) GetBlockHeight() (uint32, error) {
	info, err := lnd.getInfo()
	if err != nil {
		return 0, err
	}
	return info.BlockHeight, nil
}

func (lnd *LND) ConnectPeer(uri string) error {
	uriParts := strings.Split(uri, "@")

	if len(uriParts) != 2 {
		return errors.New("could not parse URI")
	}
	_, err := lnd.client.ConnectPeer(lnd.ctx, &lnrpc.ConnectPeerRequest{
		Perm: true,
		Addr: &lnrpc.LightningAddress{
			Host:   uriParts[0],
			Pubkey: uriParts[1],
		},
	})
	return err
}

func (lnd *LND) PendingChannels() (*lnrpc.PendingChannelsResponse, error) {
	return lnd.client.PendingChannels(lnd.ctx, &lnrpc.PendingChannelsRequest{})
}

func parseChannelPoint(channelPoint string) (*lightning.ChannelPoint, error) {
	split := strings.Split(channelPoint, ":")
	vout, err := strconv.Atoi(split[1])

	if err != nil {
		return nil, err
	}

	return &lightning.ChannelPoint{
		FundingTxId: split[0],
		OutputIndex: uint32(vout),
	}, nil
}

func (lnd *LND) ListChannels() ([]*lightning.LightningChannel, error) {
	channels, err := lnd.client.ListChannels(lnd.ctx, &lnrpc.ListChannelsRequest{})

	if err != nil {
		return nil, err
	}

	var results []*lightning.LightningChannel

	for _, channel := range channels.Channels {
		point, err := parseChannelPoint(channel.ChannelPoint)

		if err != nil {
			logger.Warn("Could not parse channel point: " + err.Error())
		}
		results = append(results, &lightning.LightningChannel{
			LocalSat:  uint64(channel.LocalBalance),
			RemoteSat: uint64(channel.RemoteBalance),
			Capacity:  uint64(channel.Capacity),
			Id:        lightning.ChanId(channel.ChanId),
			PeerId:    channel.RemotePubkey,
			Point:     *point,
		})
	}

	return results, nil
}

func (lnd *LND) CreateInvoice(value int64, preimage []byte, expiry int64, memo string) (*lightning.AddInvoiceResponse, error) {
	invoice, err := lnd.client.AddInvoice(lnd.ctx, &lnrpc.Invoice{
		Memo:      memo,
		Value:     value,
		Expiry:    expiry,
		RPreimage: preimage,
	})
	if err != nil {
		return nil, err
	}
	return &lightning.AddInvoiceResponse{
		PaymentRequest: invoice.PaymentRequest,
		PaymentHash:    invoice.RHash,
	}, nil
}

func (lnd *LND) CheckInvoicePaid(paymentHash []byte) (bool, error) {
	invoice, err := lnd.client.LookupInvoice(lnd.ctx, &lnrpc.PaymentHash{
		RHash: paymentHash,
	})
	if err != nil {
		return false, err
	}
	return invoice.State == lnrpc.Invoice_SETTLED, nil
}

func (lnd *LND) CheckPaymentFee(paymentHash []byte) (uint64, error) {
	client, err := lnd.router.TrackPaymentV2(lnd.ctx, &routerrpc.TrackPaymentRequest{
		PaymentHash:       paymentHash,
		NoInflightUpdates: true,
	})
	if err != nil {
		return 0, err
	}

	payment, err := client.Recv()

	if err != nil {
		return 0, err
	}
	if payment.Status == lnrpc.Payment_SUCCEEDED {
		return uint64(payment.FeeMsat), nil
	}
	return 0, nil
}

func (lnd *LND) AddHoldInvoice(preimageHash []byte, value int64, expiry int64, memo string) (*invoicesrpc.AddHoldInvoiceResp, error) {
	return lnd.invoices.AddHoldInvoice(lnd.ctx, &invoicesrpc.AddHoldInvoiceRequest{
		Memo:   memo,
		Value:  value,
		Expiry: expiry,
		Hash:   preimageHash,
	})
}

func (lnd *LND) SettleInvoice(preimage []byte) (*invoicesrpc.SettleInvoiceResp, error) {
	return lnd.invoices.SettleInvoice(lnd.ctx, &invoicesrpc.SettleInvoiceMsg{
		Preimage: preimage,
	})
}

func (lnd *LND) CancelInvoice(preimageHash []byte) (*invoicesrpc.CancelInvoiceResp, error) {
	return lnd.invoices.CancelInvoice(lnd.ctx, &invoicesrpc.CancelInvoiceMsg{
		PaymentHash: preimageHash,
	})
}

func (lnd *LND) LookupInvoice(preimageHash []byte) (*lnrpc.Invoice, error) {
	return lnd.client.LookupInvoice(lnd.ctx, &lnrpc.PaymentHash{
		RHash: preimageHash,
	})
}

func (lnd *LND) GetChannelInfo(chanId uint64) (*lnrpc.ChannelEdge, error) {
	return lnd.client.GetChanInfo(lnd.ctx, &lnrpc.ChanInfoRequest{
		ChanId: chanId,
	})
}

func (lnd *LND) PayInvoice(invoice string, feeLimit uint, timeoutSeconds uint, chanIds []lightning.ChanId) (*lightning.PayInvoiceResponse, error) {
	var outgoungIds []uint64
	for _, chanId := range chanIds {
		outgoungIds = append(outgoungIds, uint64(chanId))
	}

	fmt.Println(outgoungIds)

	client, err := lnd.router.SendPaymentV2(lnd.ctx, &routerrpc.SendPaymentRequest{
		PaymentRequest:    invoice,
		TimeoutSeconds:    int32(timeoutSeconds),
		FeeLimitSat:       int64(feeLimit),
		NoInflightUpdates: true,
		// TODO: this is not working right now
		OutgoingChanIds: outgoungIds,
	})
	if err != nil {
		return nil, err
	}

	event, err := client.Recv()
	if err != nil {
		return nil, err
	}

	switch event.Status {
	case lnrpc.Payment_SUCCEEDED:
		return &lightning.PayInvoiceResponse{
			FeeMsat: uint(event.FeeMsat),
		}, nil
	case lnrpc.Payment_FAILED:
		return nil, errors.New(event.FailureReason.String())
	default:
		return nil, errors.New("unknown payment status")
	}
}

func (lnd *LND) PaymentStatus(paymentHash []byte) (*lightning.PaymentStatus, error) {
	client, err := lnd.router.TrackPaymentV2(lnd.ctx, &routerrpc.TrackPaymentRequest{
		PaymentHash:       paymentHash,
		NoInflightUpdates: true,
	})
	if err != nil {
		return nil, err
	}

	event, err := client.Recv()
	if err != nil {
		return nil, err
	}

	return &lightning.PaymentStatus{
		FeeMsat:       uint64(event.FeeMsat),
		FailureReason: event.FailureReason.String(),
		Preimage:      event.PaymentPreimage,
		State:         paymentStatusFromGrpc[event.Status],
	}, nil
}

func (lnd *LND) NewAddress() (string, error) {
	response, err := lnd.client.NewAddress(lnd.ctx, &lnrpc.NewAddressRequest{
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})

	if err != nil {
		return "", err
	}

	return response.Address, err
}

func (lnd *LND) EstimateFee(confTarget int32) (float64, error) {
	response, err := lnd.walletKit.EstimateFee(lnd.ctx, &walletrpc.EstimateFeeRequest{
		ConfTarget: confTarget,
	})
	if err != nil {
		return 0, err
	}
	return math.Max(math.Round(float64(response.SatPerKw)/1000), 2), nil
}

func (lnd *LND) RegisterBlockListener(channel chan<- *onchain.BlockEpoch, stop <-chan bool) error {
	client, err := lnd.chainNotifier.RegisterBlockEpochNtfn(lnd.ctx, &chainrpc.BlockEpoch{})

	if err != nil {
		return err
	}

	logger.Info("Connected to LND block epoch stream")

	errChannel := make(chan error)
	recv := true

	go func() {
		for {
			block, err := client.Recv()
			if !recv {
				return
			}
			if err != nil {
				errChannel <- err
				return
			}

			channel <- &onchain.BlockEpoch{
				Height: block.Height,
			}
		}
	}()

	for {
		select {
		case err := <-errChannel:
			return err
		case <-stop:
			recv = false
			return nil
		}
	}
}
func (lnd *LND) GetBalance() (*onchain.Balance, error) {
	response, err := lnd.client.WalletBalance(lnd.ctx, &lnrpc.WalletBalanceRequest{})
	if err != nil {
		return nil, err
	}
	return &onchain.Balance{
		Total:       uint64(response.TotalBalance),
		Confirmed:   uint64(response.ConfirmedBalance),
		Unconfirmed: uint64(response.UnconfirmedBalance),
	}, nil

}

func (lnd *LND) SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error) {
	response, err := lnd.client.SendCoins(lnd.ctx, &lnrpc.SendCoinsRequest{
		Addr:        address,
		Amount:      int64(amount),
		SatPerVbyte: uint64(satPerVbyte),
	})

	if err != nil {
		return "", err
	}

	return response.Txid, nil
}

func (lnd *LND) SubscribeSingleInvoice(preimageHash []byte, channel chan *lnrpc.Invoice, errChannel chan error) {
	client, err := lnd.invoices.SubscribeSingleInvoice(lnd.ctx, &invoicesrpc.SubscribeSingleInvoiceRequest{
		RHash: preimageHash,
	})

	if err != nil {
		errChannel <- err
		return
	}

	logger.Info("Connected to LND invoice event stream: " + hex.EncodeToString(preimageHash))

	go func() {
		for {
			invoice, err := client.Recv()

			if err != nil {
				errChannel <- err
				return
			}

			channel <- invoice
		}
	}()
}
