package boltz

type SwapUpdateEvent int

const (
	SwapCreated SwapUpdateEvent = iota
	SwapExpired

	InvoiceSet
	InvoicePaid
	InvoiceSettled
	InvoiceFailedToPay

	TransactionFailed
	TransactionMempool
	TransactionClaimed
	TransactionRefunded
	TransactionConfirmed

	// Custom events

	// Client refunded transaction
	SwapRefunded
)

var swapUpdateEventStrings = map[string]SwapUpdateEvent{
	"swap.created": SwapCreated,
	"swap.expired": SwapExpired,

	"invoice.set":         InvoiceSet,
	"invoice.paid":        InvoicePaid,
	"invoice.settled":     InvoiceSettled,
	"invoice.failedToPay": InvoiceFailedToPay,

	"transaction.failed":    TransactionFailed,
	"transaction.mempool":   TransactionMempool,
	"transaction.claimed":   TransactionClaimed,
	"transaction.refunded":  TransactionRefunded,
	"transaction.confirmed": TransactionConfirmed,

	"swap.refunded": SwapRefunded,
}

var CompletedStatus = []string{
	SwapRefunded.String(),
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
