package autoswap

import (
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	onchainmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/test"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type mockedWallet struct {
	info    onchain.WalletInfo
	balance *onchain.Balance
}

func (m mockedWallet) Create(t *testing.T) *onchainmock.MockWallet {
	if m.info.TenantId == 0 {
		m.info.TenantId = database.DefaultTenantId
	}
	wallet := onchainmock.NewMockWallet(t)
	wallet.EXPECT().Ready().Return(true).Maybe()
	wallet.EXPECT().GetWalletInfo().Return(m.info)
	wallet.EXPECT().Sync().Return(nil).Maybe()
	if m.balance != nil {
		wallet.EXPECT().GetBalance().Return(m.balance, nil)
	}
	return wallet
}

func newPairInfo() *boltzrpc.PairInfo {
	return &boltzrpc.PairInfo{
		Limits: &boltzrpc.Limits{
			Minimal: 1000,
			Maximal: 1000000,
		},
		Fees: &boltzrpc.SwapFees{
			Percentage: 1,
			MinerFees:  10,
		},
	}
}

func TestChainSwapper(t *testing.T) {
	walletInfo := onchain.WalletInfo{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid}
	baseConfig := func() *SerializedChainConfig {
		return &SerializedChainConfig{
			MaxBalance:    100000,
			FromWallet:    "test",
			ToAddress:     "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			MaxFeePercent: 10,
		}
	}

	setup := func(t *testing.T) (*AutoSwap, *ChainSwapper, *MockRpcProvider, *onchainmock.MockWallet) {
		fromWallet := mockedWallet{info: onchain.WalletInfo{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid}}.Create(t)

		swapper, mockProvider := getSwapper(t)
		swapper.onchain.AddWallet(fromWallet)

		err := swapper.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: baseConfig()}, database.DefaultTenant)
		require.NoError(t, err)

		return swapper, swapper.GetChainSwapper(database.DefaultTenantId), mockProvider, fromWallet
	}

	test.InitLogger()

	defaultBalance := onchain.Balance{Total: 200000, Confirmed: 200000, Unconfirmed: 0}

	t.Run("GetRecommendation", func(t *testing.T) {

		testTenantName := "test"

		tests := []struct {
			name                     string
			config                   *SerializedChainConfig
			balance                  onchain.Balance
			expectedAmount           uint64
			expectedDismissedReasons []string
			tenantId                 database.Id
			setup                    func(t *testing.T, shared shared)
		}{
			{
				name: "Valid",
				config: &SerializedChainConfig{
					ReserveBalance: MinReserve,
				},
				balance:                  defaultBalance,
				expectedAmount:           defaultBalance.Confirmed - MinReserve,
				expectedDismissedReasons: nil,
			},
			{
				name:                     "NoBalance",
				config:                   &SerializedChainConfig{},
				balance:                  onchain.Balance{Total: 100, Confirmed: 100, Unconfirmed: 0},
				expectedAmount:           0,
				expectedDismissedReasons: nil,
			},
			{
				name:           "Dismissed/Checks",
				config:         &SerializedChainConfig{},
				balance:        defaultBalance,
				expectedAmount: defaultBalance.Confirmed,
				expectedDismissedReasons: []string{
					ReasonAmountBelowMin,
					ReasonMaxFeePercent,
					ReasonBudgetExceeded,
				},
				setup: func(t *testing.T, shared shared) {
					mockRpc(shared).EXPECT().GetAutoSwapPairInfo(mock.Anything, mock.Anything).Return(&boltzrpc.PairInfo{
						Limits: &boltzrpc.Limits{
							Minimal: 100000000,
							Maximal: 1000000000,
						},
						Fees: &boltzrpc.SwapFees{
							MinerFees:  1000000,
							Percentage: 1,
						},
					}, nil)
				},
			},
			{
				name: "PendingSwap/SameTenant",
				config: &SerializedChainConfig{
					ReserveBalance: 0,
				},
				balance:        defaultBalance,
				expectedAmount: defaultBalance.Confirmed,
				setup: func(t *testing.T, shared shared) {
					fakeSwaps := test.FakeSwaps{ChainSwaps: []database.ChainSwap{
						{
							State: boltzrpc.SwapState_PENDING,
						},
					}}
					fakeSwaps.Create(t, shared.database)
				},
				expectedDismissedReasons: []string{ReasonPendingSwap},
			},
			{
				name: "PendingSwap/OtherTenant",
				config: &SerializedChainConfig{
					ReserveBalance: 0,
				},
				balance:        defaultBalance,
				expectedAmount: defaultBalance.Confirmed,
				setup: func(t *testing.T, shared shared) {
					tenant := &database.Tenant{Name: testTenantName}
					require.NoError(t, shared.database.CreateTenant(tenant))
					fakeSwaps := test.FakeSwaps{ChainSwaps: []database.ChainSwap{
						{
							TenantId: tenant.Id,
							State:    boltzrpc.SwapState_PENDING,
						},
					}}
					fakeSwaps.Create(t, shared.database)
				},
				expectedDismissedReasons: nil,
			},
			{
				name: "Sweep/SendFee",
				config: &SerializedChainConfig{
					ReserveBalance: 0,
				},
				balance:        defaultBalance,
				expectedAmount: 5000,
				setup: func(t *testing.T, shared shared) {
					mockRpc(shared).EXPECT().WalletSendFee(mock.Anything).RunAndReturn(func(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
						require.True(t, request.GetSendAll())
						return &boltzrpc.WalletSendFee{Amount: 5000}, nil
					})
				},
			},
			{
				name: "Sweep/NoSendFee",
				config: &SerializedChainConfig{
					ReserveBalance: 0,
				},
				balance:        defaultBalance,
				expectedAmount: defaultBalance.Confirmed - MinReserve,
				setup: func(t *testing.T, shared shared) {
					mockRpc(shared).EXPECT().WalletSendFee(mock.Anything).RunAndReturn(func(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
						require.True(t, request.GetSendAll())
						return nil, status.Errorf(codes.InvalidArgument, "error")
					})
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				shared := getShared(t)
				shared.onchain.AddWallet(mockedWallet{info: walletInfo, balance: &tc.balance}.Create(t))
				if tc.setup != nil {
					tc.setup(t, shared)
				}

				mockRpc(shared).EXPECT().GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, mock.Anything).Return(newPairInfo(), nil)
				mockRpc(shared).EXPECT().WalletSendFee(mock.Anything).RunAndReturn(func(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
					return &boltzrpc.WalletSendFee{Amount: request.Amount}, nil
				}).Maybe()

				chainConfig := NewChainConfig(merge(baseConfig(), tc.config), shared)
				require.NoError(t, chainConfig.Init())

				result, err := chainConfig.GetRecommendation()
				require.NoError(t, err)
				require.NotZero(t, result.MaxBalance)
				swap := result.Swap
				if tc.expectedAmount == 0 {
					require.Nil(t, swap)
				} else {
					require.NotZero(t, swap.FeeEstimate)
					require.Equal(t, tc.expectedDismissedReasons, swap.DismissedReasons)
					require.Equal(t, tc.expectedAmount, swap.Amount)
				}
			})
		}
	})

	t.Run("Execute", func(t *testing.T) {
		_, chainSwapper, rpcMock, _ := setup(t)

		var amount uint64 = 750

		rpcMock.EXPECT().CreateAutoChainSwap(mock.Anything, mock.Anything).RunAndReturn(func(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error {
			require.Equal(t, database.DefaultTenantId, tenant.Id)
			require.Equal(t, amount, request.GetAmount())
			require.NotNil(t, request.FromWalletId)
			require.NotZero(t, request.ToAddress)
			return nil
		}).Times(3)

		swap := &autoswaprpc.ChainSwap{Amount: amount}
		require.NoError(t, chainSwapper.cfg.execute(swap, nil, false))
		require.NoError(t, chainSwapper.cfg.execute(nil, nil, false))
		swap.DismissedReasons = []string{ReasonBudgetExceeded}
		require.NoError(t, chainSwapper.cfg.execute(swap, nil, false))
		require.NoError(t, chainSwapper.cfg.execute(swap, nil, true))

		accepted := &autoswaprpc.ChainSwap{
			DismissedReasons: swap.DismissedReasons,
			Amount:           amount + 50,
			FeeEstimate:      swap.FeeEstimate + 1,
		}
		require.NoError(t, chainSwapper.cfg.execute(swap, accepted, true))
		accepted.DismissedReasons = []string{}
		require.Error(t, chainSwapper.cfg.execute(swap, accepted, true))
	})

	t.Run("Start", func(t *testing.T) {
		swapper, chainSwapper, rpcMock, fromWallet := setup(t)

		pairInfo := newPairInfo()
		rpcMock.EXPECT().GetAutoSwapPairInfo(boltzrpc.SwapType_CHAIN, mock.Anything).Return(pairInfo, nil).Once()
		rpcMock.EXPECT().CreateAutoChainSwap(mock.Anything, mock.Anything).Return(nil).Once()
		rpcMock.EXPECT().WalletSendFee(mock.Anything).RunAndReturn(func(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
			return &boltzrpc.WalletSendFee{Amount: request.Amount}, nil
		}).Maybe()

		balance := defaultBalance
		fromWallet.EXPECT().GetBalance().Return(&balance, nil)

		cleaned := false
		blockUpdates := make(chan *onchain.BlockEpoch)
		rpcMock.EXPECT().GetBlockUpdates(fromWallet.GetWalletInfo().Currency).Return(blockUpdates, func() {
			if !cleaned {
				cleaned = true
				close(blockUpdates)
			}
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
		time.Sleep(100 * time.Millisecond)

		require.True(t, swapper.WalletUsed(fromWallet.GetWalletInfo().Id))

		swapper.onchain.RemoveWallet(fromWallet.GetWalletInfo().Id)
		time.Sleep(100 * time.Millisecond)

		require.False(t, chainSwapper.Running())
		require.NotEmpty(t, chainSwapper.Error())
		require.True(t, cleaned)
	})

}
