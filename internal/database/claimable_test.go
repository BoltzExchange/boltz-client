package database_test

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestQueryClaimableSwapsExcludesPermanentErrors(t *testing.T) {
	db := database.Database{Path: "file:claimable_test?mode=memory&cache=shared"}
	require.NoError(t, db.Connect())

	reverse := func(id, swapErr string) database.ReverseSwap {
		return database.ReverseSwap{
			Id:                  id,
			Pair:                boltz.Pair{From: boltz.CurrencyLiquid, To: boltz.CurrencyBtc},
			Status:              boltz.TransactionMempool,
			LockupTransactionId: "lockup",
			Error:               swapErr,
		}
	}
	chain := func(id, swapErr string) database.ChainSwap {
		return database.ChainSwap{
			Id:     id,
			State:  boltzrpc.SwapState_PENDING,
			Status: boltz.TransactionMempool,
			Error:  swapErr,
			ToData: &database.ChainSwapData{LockupTransactionId: "lockup"},
		}
	}

	test.FakeSwaps{
		ReverseSwaps: []database.ReverseSwap{
			reverse("rev-no-error", ""),
			reverse("rev-transient", "broadcast: some transient error"),
			reverse("rev-permanent", "broadcast: bad-txns-inputs-missingorspent"),
			// upper-case to assert case-insensitive matching (was strings.ToLower before)
			reverse("rev-permanent-upper", "BROADCAST: TXN-MEMPOOL-CONFLICT"),
		},
		ChainSwaps: []database.ChainSwap{
			chain("chain-no-error", ""),
			chain("chain-permanent", "broadcast: bad-txns-inputs-missing-or-spent"),
		},
	}.Create(t, &db)

	reverseSwaps, err := db.QueryClaimableReverseSwaps(nil, boltz.CurrencyBtc)
	require.NoError(t, err)
	var reverseIds []string
	for _, swap := range reverseSwaps {
		reverseIds = append(reverseIds, swap.Id)
	}
	require.ElementsMatch(t, []string{"rev-no-error", "rev-transient"}, reverseIds)

	chainSwaps, err := db.QueryClaimableChainSwaps(nil, boltz.CurrencyBtc)
	require.NoError(t, err)
	var chainIds []string
	for _, swap := range chainSwaps {
		chainIds = append(chainIds, swap.Id)
	}
	require.ElementsMatch(t, []string{"chain-no-error"}, chainIds)
}
