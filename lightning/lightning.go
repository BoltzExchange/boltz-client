package lightning

import (
	"context"
	"errors"
	"github.com/BoltzExchange/boltz-client/v2/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/onchain"
)

type PaymentState string
type LightningNodeType string

const (
	PaymentSucceeded PaymentState = "succeeded"
	PaymentFailed    PaymentState = "failed"
	PaymentPending   PaymentState = "pending"

	NodeTypeLnd LightningNodeType = "LND"
	NodeTypeCln LightningNodeType = "CLN"

	// The cltv expiry has to be lowered in regtest to allow for lower swap timeouts
	RegtestCltv = 20
)

var ErrInvoiceNotFound = errors.New("invoice not found")

type PaymentStatus struct {
	State         PaymentState
	FailureReason string
	Preimage      string
	FeeMsat       uint64
}

type PaymentUpdate struct {
	IsLastUpdate bool
	Update       PaymentStatus
}

type LightningInfo struct {
	Pubkey      string
	BlockHeight uint32
	Version     string
	Network     string
	Synced      bool
}

type ChannelPoint struct {
	FundingTxId string
	OutputIndex uint32
}

type LightningChannel struct {
	OutboundSat uint64
	InboundSat  uint64
	Capacity    uint64
	Id          ChanId
	PeerId      string
	Point       *ChannelPoint
}

func (channel *LightningChannel) GetId() ChanId {
	if channel == nil {
		return ChanId(0)
	}
	return channel.Id
}

type AddInvoiceResponse struct {
	PaymentRequest string
	PaymentHash    []byte
}

type PayInvoiceResponse struct {
	FeeMsat uint
}

type LightningNode interface {
	onchain.Wallet

	Connect() error
	Name() string
	//NodeType() LightningNodeType
	PaymentStatus(paymentHash []byte) (*PaymentStatus, error)

	//SendPayment(invoice string, feeLimit uint64, timeout int32) (<-chan *PaymentUpdate, error)
	//PayInvoice(invoice string, maxParts uint32, timeoutSeconds int32) (int64, error)
	PayInvoice(ctx context.Context, invoice string, feeLimit uint, timeoutSeconds uint, channelIds []ChanId) (*PayInvoiceResponse, error)
	CreateInvoice(value uint64, preimage []byte, expiry int64, memo string) (*AddInvoiceResponse, error)

	NewAddress() (string, error)

	GetInfo() (*LightningInfo, error)

	CheckInvoicePaid(paymentHash []byte) (bool, error)
	ListChannels() ([]*LightningChannel, error)
	//GetChannelInfo(chanId uint64) (*lnrpc.ChannelEdge, error)

	ConnectPeer(uri string) error

	SetupWallet(info onchain.WalletInfo)
}

func SerializeChanId(chanId ChanId) *boltzrpc.ChannelId {
	if chanId != 0 {
		return &boltzrpc.ChannelId{
			Cln: chanId.ToCln(),
			Lnd: chanId.ToLnd(),
		}
	}
	return nil
}

func SerializeChanIds(chanIds []ChanId) (result []*boltzrpc.ChannelId) {
	for _, chanId := range chanIds {
		result = append(result, SerializeChanId(chanId))
	}
	return result
}
