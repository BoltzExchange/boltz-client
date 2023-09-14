package lightning

type PaymentState string
type LightningNodeType string

const (
	PaymentSucceeded PaymentState = "succeeded"
	PaymentFailed    PaymentState = "failed"
	PaymentPending   PaymentState = "pending"

	NodeTypeLnd LightningNodeType = "LND"
	NodeTypeCln LightningNodeType = "CLN"
)

type PaymentStatus struct {
	State         PaymentState
	FailureReason *string

	Hash     string
	Preimage string

	FeeMsat uint64
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
	FundingTxid string
	OutputIndex uint32
}

type LightningChannel struct {
	LocalMsat  uint
	RemoteMsat uint
	Capacity   uint
	Id         string
	PeerId     string
	Point      ChannelPoint
}

type AddInvoiceResponse struct {
	PaymentRequest string
	PaymentHash    []byte
}

type PayInvoiceResponse struct {
	FeeMsat uint
}

type LightningNode interface {
	Connect() error
	//Name() string
	//NodeType() LightningNodeType
	//PaymentStatus(preimageHash string) (*PaymentStatus, error)

	//SendPayment(invoice string, feeLimit uint64, timeout int32) (<-chan *PaymentUpdate, error)
	//PayInvoice(invoice string, maxParts uint32, timeoutSeconds int32) (int64, error)
	PayInvoice(invoice string, feeLimit uint, timeoutSeconds uint) (*PayInvoiceResponse, error)
	CreateInvoice(value int64, preimage []byte, expiry int64, memo string) (*AddInvoiceResponse, error)

	NewAddress() (string, error)

	GetInfo() (*LightningInfo, error)
	ListChannels() ([]LightningChannel, error)
	//GetChannelInfo(chanId uint64) (*lnrpc.ChannelEdge, error)
}
