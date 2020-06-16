package boltz

type SwapUpdateEvent int

const (
	SwapCreated SwapUpdateEvent = iota
	SwapExpired

	InvoiceSet
	InvoicePaid
	InvoicePending
	InvoiceSettled
	InvoiceFailedToPay

	ChannelCreated

	TransactionFailed
	TransactionMempool
	TransactionClaimed
	TransactionRefunded
	TransactionConfirmed

	// Custom events

	// Client refunded transaction
	SwapRefunded

	// Client noticed the Swap expired but didn't find any output to refund
	SwapAbandoned
)

var swapUpdateEventStrings = map[string]SwapUpdateEvent{
	"swap.created": SwapCreated,
	"swap.expired": SwapExpired,

	"invoice.set":         InvoiceSet,
	"invoice.paid":        InvoicePaid,
	"invoice.pending":     InvoicePending,
	"invoice.settled":     InvoiceSettled,
	"invoice.failedToPay": InvoiceFailedToPay,

	"channel.created": ChannelCreated,

	"transaction.failed":    TransactionFailed,
	"transaction.mempool":   TransactionMempool,
	"transaction.claimed":   TransactionClaimed,
	"transaction.refunded":  TransactionRefunded,
	"transaction.confirmed": TransactionConfirmed,

	"swap.refunded":  SwapRefunded,
	"swap.abandoned": SwapAbandoned,
}

var CompletedStatus = []string{
	SwapRefunded.String(),
	SwapAbandoned.String(),
	InvoiceSettled.String(),
	TransactionClaimed.String(),
}

func (event SwapUpdateEvent) String() string {
	for key, value := range swapUpdateEventStrings {
		if event == value {
			return key
		}
	}

	return ""
}

func ParseEvent(event string) SwapUpdateEvent {
	return swapUpdateEventStrings[event]
}

type ChannelState int

const (
	ChannelNone ChannelState = iota
	ChannelAccepted
	ChannelSettled
)

var channelStateStrings = map[string]ChannelState{
	"none":     ChannelNone,
	"accepted": ChannelAccepted,
	"settled":  ChannelSettled,
}

func (event ChannelState) String() string {
	for key, value := range channelStateStrings {
		if event == value {
			return key
		}
	}

	return ""
}

func ParseChannelState(event string) ChannelState {
	return channelStateStrings[event]
}
