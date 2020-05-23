package boltz

type SwapUpdateEvent int

// TODO: add status when refund is e<xecuted locally or new database column?
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
}

var CompletedStatus = []string{
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
