package autoswap

import (
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	onchainmock "github.com/BoltzExchange/boltz-client/mocks/github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"testing"
	"time"
)

func mockedWallet(t *testing.T, info onchain.WalletInfo) *onchainmock.MockWallet {
	if info.TenantId == 0 {
		info.TenantId = database.DefaultTenantId
	}
	wallet := onchainmock.NewMockWallet(t)
	wallet.EXPECT().Ready().Return(true)
	wallet.EXPECT().GetWalletInfo().Return(info)
	return wallet
}

func newPairInfo() *boltzrpc.PairInfo {
	return &boltzrpc.PairInfo{
		Limits: &boltzrpc.Limits{
			Minimal: 100,
			Maximal: 10000,
		},
		Fees: &boltzrpc.SwapFees{
			Percentage: 1,
			MinerFees:  10,
		},
	}
}

func TestChainSwapper(t *testing.T) {
	setup := func(t *testing.T) (*AutoSwap, *ChainSwapper, *MockRpcProvider, *onchainmock.MockWallet) {
		name := database.DefaultTenantName
		config := &SerializedChainConfig{
			MaxBalance:    500,
			FromWallet:    "test",
			ToAddress:     "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			MaxFeePercent: 10,
			Tenant:        &name,
		}

		fromWallet := mockedWallet(t, onchain.WalletInfo{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid})

		swapper, mockProvider := getSwapper(t)
		swapper.onchain.AddWallet(fromWallet)

		err := swapper.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: config}, database.DefaultTenant)
		require.NoError(t, err)

		return swapper, swapper.GetChainSwapper(database.DefaultTenantId), mockProvider, fromWallet
	}

	test.InitLogger()

	t.Run("GetRecommendation", func(t *testing.T) {
		_, chainSwapper, rpcMock, fromWallet := setup(t)
		chainConfig := chainSwapper.cfg
		pairInfo := newPairInfo()
		rpcMock.EXPECT().GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, mock.Anything).Return(pairInfo, nil)

		balance := &onchain.Balance{Total: 20000, Confirmed: 20000, Unconfirmed: 0}
		fromWallet.EXPECT().GetBalance().Return(balance, nil)

		expectedAmount := balance.Confirmed - 10000

		t.Run("Valid", func(t *testing.T) {

			tenant := &database.Tenant{Name: "test"}
			require.NoError(t, chainSwapper.database.CreateTenant(tenant))
			fakeSwaps{chainSwaps: []database.ChainSwap{
				{
					TenantId: tenant.Id,
				},
			}}.create(t, chainSwapper.database)

			recommendation, err := chainConfig.GetRecommendation()
			require.NoError(t, err)
			require.NotZero(t, recommendation.Swap.FeeEstimate)
			require.Empty(t, recommendation.Swap.DismissedReasons)
			require.Equal(t, expectedAmount, recommendation.Swap.Amount)
		})

		t.Run("Dismissed", func(t *testing.T) {
			fakeSwaps{chainSwaps: []database.ChainSwap{
				{
					TenantId: chainConfig.tenant.Id,
				},
			}}.create(t, chainSwapper.database)

			pairInfo.Fees.MinerFees = 1000000
			pairInfo.Limits.Minimal = 2 * expectedAmount
			recommendation, err := chainConfig.GetRecommendation()
			require.NoError(t, err)
			require.Contains(t, recommendation.Swap.DismissedReasons, ReasonBudgetExceeded)
			require.Contains(t, recommendation.Swap.DismissedReasons, ReasonMaxFeePercent)
			require.Contains(t, recommendation.Swap.DismissedReasons, ReasonAmountBelowMin)
			require.Contains(t, recommendation.Swap.DismissedReasons, ReasonPendingSwap)
			require.Equal(t, expectedAmount, recommendation.Swap.Amount)
		})

		t.Run("NoBalance", func(t *testing.T) {
			balance.Total = 100
			balance.Confirmed = 100
			recommendation, err := chainConfig.GetRecommendation()
			require.NoError(t, err)
			require.Nil(t, recommendation.Swap)
		})

	})

	t.Run("Execute", func(t *testing.T) {
		_, chainSwapper, rpcMock, _ := setup(t)

		var amount uint64 = 750

		rpcMock.EXPECT().CreateAutoChainSwap(mock.Anything, mock.Anything).RunAndReturn(func(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error {
			require.Equal(t, database.DefaultTenantId, tenant.Id)
			require.Equal(t, amount, request.Amount)
			require.NotNil(t, request.FromWalletId)
			require.NotZero(t, request.ToAddress)
			return nil
		}).Once()

		require.NoError(t, chainSwapper.cfg.execute(&ChainSwap{Amount: amount}))
		require.NoError(t, chainSwapper.cfg.execute(nil))
		require.NoError(t, chainSwapper.cfg.execute(&ChainSwap{Amount: amount, DismissedReasons: []string{ReasonBudgetExceeded}}))
	})

	t.Run("Start", func(t *testing.T) {
		swapper, chainSwapper, rpcMock, fromWallet := setup(t)

		pairInfo := newPairInfo()
		rpcMock.EXPECT().GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, mock.Anything).Return(pairInfo, nil).Once()
		rpcMock.EXPECT().CreateAutoChainSwap(mock.Anything, mock.Anything).Return(nil).Once()

		balance := &onchain.Balance{Total: 1000, Confirmed: 1000, Unconfirmed: 0}
		fromWallet.EXPECT().GetBalance().Return(balance, nil)

		cleaned := false
		blockUpdates := make(chan *onchain.BlockEpoch)
		rpcMock.EXPECT().GetBlockUpdates(fromWallet.GetWalletInfo().Currency).Return(blockUpdates, func() {
			cleaned = true
			close(blockUpdates)
		})

		err := swapper.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{
			Config:    &autoswaprpc.ChainConfig{Enabled: true},
			FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"enabled"}},
		}, database.DefaultTenant)
		require.NoError(t, err)

		require.True(t, chainSwapper.cfg.Enabled)
		require.True(t, chainSwapper.Running())
		require.Empty(t, chainSwapper.Error())

		blockUpdates <- &onchain.BlockEpoch{Height: 1000}

		require.True(t, swapper.WalletUsed(fromWallet.GetWalletInfo().Id))

		swapper.onchain.RemoveWallet(fromWallet.GetWalletInfo().Id)
		time.Sleep(100 * time.Millisecond)

		require.False(t, chainSwapper.Running())
		require.NotEmpty(t, chainSwapper.Error())
		require.True(t, cleaned)
	})

}
