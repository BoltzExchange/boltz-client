package lightning

import (
	"github.com/BoltzExchange/boltz-client/onchain"
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
	LocalSat  uint64
	RemoteSat uint64
	Capacity  uint64
	Id        ChanId
	PeerId    string
	Point     ChannelPoint
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
	onchain.BlockListener
	onchain.Wallet

	Connect() error
	Name() string
	//NodeType() LightningNodeType
	PaymentStatus(paymentHash []byte) (*PaymentStatus, error)

	//SendPayment(invoice string, feeLimit uint64, timeout int32) (<-chan *PaymentUpdate, error)
	//PayInvoice(invoice string, maxParts uint32, timeoutSeconds int32) (int64, error)
	PayInvoice(invoice string, feeLimit uint, timeoutSeconds uint, channelIds []ChanId) (*PayInvoiceResponse, error)
	CreateInvoice(value int64, preimage []byte, expiry int64, memo string) (*AddInvoiceResponse, error)

	NewAddress() (string, error)

	GetInfo() (*LightningInfo, error)

	CheckInvoicePaid(paymentHash []byte) (bool, error)
	ListChannels() ([]*LightningChannel, error)
	//GetChannelInfo(chanId uint64) (*lnrpc.ChannelEdge, error)

	ConnectPeer(uri string) error

	SetupWallet(int64)
}
