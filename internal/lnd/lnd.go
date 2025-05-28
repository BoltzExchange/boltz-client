package lnd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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
		lnrpc.Payment_INITIATED: lightning.PaymentPending,
		lnrpc.Payment_IN_FLIGHT: lightning.PaymentPending,
		lnrpc.Payment_SUCCEEDED: lightning.PaymentSucceeded,
		lnrpc.Payment_FAILED:    lightning.PaymentFailed,
		// Ignore the UNKNOWN status in the linter but keep using it for compatibility with older LND versions
		//nolint:all
		lnrpc.Payment_UNKNOWN: lightning.PaymentFailed,
	}
)

type LND struct {
	Host        string `long:"lnd.host" description:"gRPC host of the LND node"`
	Port        int    `long:"lnd.port" description:"gRPC port of the LND node"`
	Macaroon    string `long:"lnd.macaroon" description:"Path to a macaroon file of the LND node or the macaroon encoded in hex"`
	Certificate string `long:"lnd.certificate" description:"Path to a certificate file of the LND node"`
	DataDir     string `long:"lnd.datadir" description:"Path to the data directory of the LND node"`
	Insecure    bool   `long:"lnd.insecure" description:"Connect to lnd without TLS"`

	ctx           context.Context
	metadata      metadata.MD
	regtest       bool
	client        lnrpc.LightningClient
	router        routerrpc.RouterClient
	invoices      invoicesrpc.InvoicesClient
	walletKit     walletrpc.WalletKitClient
	chainNotifier chainrpc.ChainNotifierClient

	walletInfo onchain.WalletInfo
}

func (lnd *LND) withParentCtx(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(ctx, lnd.metadata)
}

func (lnd *LND) GetWalletInfo() onchain.WalletInfo {
	return lnd.walletInfo
}

func (lnd *LND) Name() string {
	return "LND"
}

func (lnd *LND) SetupWallet(info onchain.WalletInfo) {
	lnd.walletInfo = info
}

func (lnd *LND) Ready() bool {
	return lnd.client != nil
}

func (lnd *LND) Disconnect() error {
	return nil
}

func (lnd *LND) Connect() error {
	creds := insecure.NewCredentials()
	if !lnd.Insecure {
		cert, err := filepath.EvalSymlinks(lnd.Certificate)
		if err != nil {
			return errors.New(fmt.Sprint("could not eval symlinks: ", err))
		}
		creds, err = credentials.NewClientTLSFromFile(cert, "")

		if err != nil {
			return errors.New(fmt.Sprint("could not read LND certificate: ", err))
		}
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
		macaroon, err := hex.DecodeString(lnd.Macaroon)
		if err != nil {
			macaroon, err = os.ReadFile(lnd.Macaroon)

			if err != nil {
				return errors.New(fmt.Sprint("could not read LND macaroon: ", err))
			}
		}

		lnd.metadata = metadata.Pairs("macaroon", hex.EncodeToString(macaroon))
		lnd.ctx = lnd.withParentCtx(context.Background())
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
	network := info.Chains[0].Network
	lnd.regtest = network == "regtest"
	return &lightning.LightningInfo{
		Pubkey:      info.IdentityPubkey,
		BlockHeight: info.BlockHeight,
		Version:     info.Version,
		Network:     network,
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
			OutboundSat: uint64(channel.LocalBalance),
			InboundSat:  uint64(channel.RemoteBalance),
			Capacity:    uint64(channel.Capacity),
			Id:          lightning.ChanId(channel.ChanId),
			PeerId:      channel.RemotePubkey,
			Point:       point,
		})
	}

	return results, nil
}

func (lnd *LND) GetTransactions(limit, offset uint64) ([]*onchain.WalletTransaction, error) {
	return nil, lightning.ErrUnsupported
}

func (lnd *LND) BumpTransactionFee(txId string, feeRate float64) (string, error) {
	return "", lightning.ErrUnsupported
}

func (lnd *LND) CreateInvoice(value uint64, preimage []byte, expiry int64, memo string) (*lightning.AddInvoiceResponse, error) {
	request := &lnrpc.Invoice{
		Memo:      memo,
		Value:     int64(value),
		Expiry:    expiry,
		RPreimage: preimage,
	}
	if lnd.regtest {
		request.CltvExpiry = lightning.RegtestCltv
	}
	invoice, err := lnd.client.AddInvoice(lnd.ctx, request)
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
		if strings.Contains(err.Error(), "there are no existing invoices") {
			return false, lightning.ErrInvoiceNotFound
		}
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

func (lnd *LND) PayInvoice(ctx context.Context, invoice string, feeLimit uint, timeoutSeconds uint, chanIds []lightning.ChanId) (*lightning.PayInvoiceResponse, error) {
	var outgoungIds []uint64
	for _, chanId := range chanIds {
		if chanId != 0 {
			outgoungIds = append(outgoungIds, uint64(chanId))
		}
	}

	client, err := lnd.router.SendPaymentV2(lnd.withParentCtx(ctx), &routerrpc.SendPaymentRequest{
		PaymentRequest:    invoice,
		TimeoutSeconds:    int32(timeoutSeconds),
		FeeLimitSat:       int64(feeLimit),
		NoInflightUpdates: true,
		OutgoingChanIds:   outgoungIds,
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
		NoInflightUpdates: false,
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

func (lnd *LND) SendToAddress(args onchain.WalletSendArgs) (string, error) {
	response, err := lnd.client.SendCoins(lnd.ctx, &lnrpc.SendCoinsRequest{
		Addr:        args.Address,
		Amount:      int64(args.Amount),
		SatPerVbyte: uint64(args.SatPerVbyte),
		SendAll:     args.SendAll,
	})

	if err != nil {
		return "", err
	}

	return response.Txid, nil
}

func (lnd *LND) GetSendFee(args onchain.WalletSendArgs) (send uint64, fee uint64, err error) {
	return 0, 0, lightning.ErrUnsupported
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
