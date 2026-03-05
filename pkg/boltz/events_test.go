package boltz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEvent(t *testing.T) {
	assert.Equal(t, SwapCreated, ParseEvent("swap.created"))
	assert.Equal(t, TransactionMempool, ParseEvent("transaction.mempool"))
	assert.Equal(t, SwapUnknown, ParseEvent("not.a.real.event"))
}
