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
	TransactionLockupFailed
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

	"transaction.failed":       TransactionFailed,
	"transaction.mempool":      TransactionMempool,
	"transaction.claimed":      TransactionClaimed,
	"transaction.refunded":     TransactionRefunded,
	"transaction.confirmed":    TransactionConfirmed,
	"transaction.lockupFailed": TransactionLockupFailed,
}

var CompletedStatus = []string{
	InvoiceSettled.String(),
	TransactionClaimed.String(),
}

var FailedStatus = []string{
	SwapExpired.String(),
	InvoiceFailedToPay.String(),
	TransactionFailed.String(),
	TransactionLockupFailed.String(),
	TransactionRefunded.String(),
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

func (event SwapUpdateEvent) IsCompletedStatus() bool {
	eventString := event.String()

	for _, value := range CompletedStatus {
		if value == eventString {
			return true
		}
	}

	return false
}

func (event SwapUpdateEvent) IsFailedStatus() bool {
	eventString := event.String()

	for _, value := range FailedStatus {
		if value == eventString {
			return true
		}
	}

	return false
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
