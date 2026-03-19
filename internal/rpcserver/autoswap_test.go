//go:build !unit

package rpcserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"

	"github.com/BoltzExchange/boltz-client/v2/internal/autoswap"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestAutoSwap(t *testing.T) {
	cfg := loadConfig(t)
	cfg.Node = "CLN"

	_, err := connectLightning(nil, cfg.Cln)
	require.NoError(t, err)
	_, err = connectLightning(nil, cfg.LND)
	require.NoError(t, err)

	admin, autoSwap, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()
	fundedWallet(t, admin, boltzrpc.Currency_LBTC)

	reset := func(t *testing.T) {
		_, err = autoSwap.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)
		_, err = autoSwap.ResetConfig(client.ChainAutoSwap)
		require.NoError(t, err)
	}

	t.Run("Chain", func(t *testing.T) {
		executeChainRecommendation := func(t *testing.T, chainCfg *autoswaprpc.ChainConfig) {
			t.Helper()

			_, err := autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: chainCfg})
			require.NoError(t, err)

			status, err := autoSwap.GetStatus()
			require.NoError(t, err)
			require.False(t, status.Chain.Running)

			recommendations, err := autoSwap.GetRecommendations()
			require.NoError(t, err)
			require.Len(t, recommendations.Chain, 1)

			stream, _ := swapStream(t, admin, "")

			_, err = autoSwap.ExecuteRecommendations(&autoswaprpc.ExecuteRecommendationsRequest{
				Chain: recommendations.Chain,
			})
			require.NoError(t, err)

			info := stream(boltzrpc.SwapState_PENDING)
			require.NotNil(t, info.ChainSwap)
			id := info.ChainSwap.Id

			recommendations, err = autoSwap.GetRecommendations()
			require.NoError(t, err)
			require.Len(t, recommendations.Chain, 1)
			require.Nil(t, recommendations.Chain[0].Swap)

			require.Eventually(t, func() bool {
				recommendations, err = autoSwap.GetRecommendations()
				require.NoError(t, err)
				return recommendations.Chain[0].Swap == nil
			}, 10*time.Second, 250*time.Millisecond)

			response, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{Include: boltzrpc.IncludeSwaps_AUTO})
			require.NoError(t, err)
			require.True(t, slices.ContainsFunc(response.ChainSwaps, func(swap *boltzrpc.ChainSwapInfo) bool {
				return swap.Id == id && swap.IsAuto
			}))

			stream, _ = swapStream(t, admin, id)
			test.MineBlock()
			stream(boltzrpc.SwapState_PENDING)
			test.MineBlock()
			stream(boltzrpc.SwapState_SUCCESSFUL)
		}

		t.Run("KeepsReserveBalance", func(t *testing.T) {
			reset(t)

			fromWallet := fundedWallet(t, admin, boltzrpc.Currency_LBTC)
			maxBalance := fromWallet.Balance.Confirmed - 1000
			chainCfg := &autoswaprpc.ChainConfig{
				FromWallet:     fromWallet.Name,
				ToWallet:       cfg.Node,
				Budget:         1_000_000,
				MaxBalance:     maxBalance,
				ReserveBalance: maxBalance - swapAmount,
			}

			executeChainRecommendation(t, chainCfg)

			_, write, _ := createTenant(t, admin, "test")
			tenant := client.NewAutoSwapClient(write)

			_, err := tenant.GetChainConfig()
			require.Error(t, err)

			chainCfg.Enabled = true
			_, err = autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: chainCfg})
			require.NoError(t, err)

			status, err := autoSwap.GetStatus()
			require.NoError(t, err)
			require.True(t, status.Chain.Running)
		})

		t.Run("SweepsWalletToZero", func(t *testing.T) {
			reset(t)

			fromWallet := fundedWallet(t, admin, boltzrpc.Currency_LBTC)
			chainCfg := &autoswaprpc.ChainConfig{
				FromWallet:     fromWallet.Name,
				ToWallet:       cfg.Node,
				Budget:         1_000_000,
				MaxBalance:     fromWallet.Balance.Confirmed - 1000,
				ReserveBalance: 0,
			}

			executeChainRecommendation(t, chainCfg)

			require.Eventually(t, func() bool {
				wallet, err := admin.GetWalletById(fromWallet.Id)
				require.NoError(t, err)
				return wallet.Balance.GetTotal() == 0
			}, 10*time.Second, 200*time.Millisecond)
		})
	})

	t.Run("Lightning", func(t *testing.T) {
		reset(t)

		t.Run("Setup", func(t *testing.T) {
			running := func(value bool) *autoswaprpc.GetStatusResponse {
				status, err := autoSwap.GetStatus()
				require.NoError(t, err)
				require.Equal(t, value, status.Lightning.Running)
				require.NotEmpty(t, status.Lightning.Description)
				return status
			}

			running(false)

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{
				Config: &autoswaprpc.LightningConfig{
					Currency: boltzrpc.Currency_LBTC,
					Wallet:   walletName(boltzrpc.Currency_LBTC),
				},
				FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"currency", "wallet"}},
			})
			require.NoError(t, err)

			_, err = autoSwap.Enable()
			require.NoError(t, err)

			status := running(true)
			require.Empty(t, status.Lightning.Error)

			_, err = autoSwap.SetLightningConfigValue("wallet", "invalid")
			require.Error(t, err)
		})

		t.Run("CantRemoveWallet", func(t *testing.T) {
			_, err := autoSwap.SetLightningConfigValue("wallet", walletName(boltzrpc.Currency_LBTC))
			require.NoError(t, err)
			_, err = autoSwap.SetLightningConfigValue("enabled", true)
			require.NoError(t, err)
			_, err = admin.RemoveWallet(walletId(t, admin, boltzrpc.Currency_LBTC))
			require.Error(t, err)
		})

		t.Run("Start", func(t *testing.T) {
			swapCfg := autoswap.DefaultLightningConfig()
			swapCfg.AcceptZeroConf = true
			swapCfg.Budget = 1000000
			swapCfg.MaxFeePercent = 10
			swapCfg.Currency = boltzrpc.Currency_BTC
			swapCfg.InboundBalance = 1
			swapCfg.OutboundBalance = 1
			swapCfg.Wallet = strings.ToUpper(cfg.Node)

			_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
			require.NoError(t, err)

			setupRecommendation := func(t *testing.T) {
				// sleep for a second to make sure the lightning channel states are updated from previous tests
				time.Sleep(1 * time.Second)
				recommendations, err := autoSwap.GetRecommendations()
				require.NoError(t, err)
				recommendation := recommendations.Lightning[0]
				offset := uint64(100000)
				swapCfg.InboundBalance = recommendation.Channel.InboundSat + offset
				swapCfg.OutboundBalance = recommendation.Channel.OutboundSat - offset

				_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: swapCfg})
				require.NoError(t, err)
			}

			t.Run("Recommendations", func(t *testing.T) {
				setupRecommendation(t)

				recommendations, err := autoSwap.GetRecommendations()
				require.NoError(t, err)
				require.Len(t, recommendations.Lightning, 1)
				require.Equal(t, boltzrpc.SwapType_REVERSE, recommendations.Lightning[0].Swap.Type)

				stream, _ := swapStream(t, admin, "")
				_, err = autoSwap.ExecuteRecommendations(&autoswaprpc.ExecuteRecommendationsRequest{
					Lightning: recommendations.Lightning,
				})
				require.NoError(t, err)
				info := stream(boltzrpc.SwapState_PENDING)
				require.NotNil(t, info.ReverseSwap)
				require.True(t, info.ReverseSwap.IsAuto)

				stream(boltzrpc.SwapState_SUCCESSFUL)
				test.MineBlock()
			})

			t.Run("Auto", func(t *testing.T) {
				setupRecommendation(t)

				stream, _ := swapStream(t, admin, "")

				_, err := autoSwap.Enable()
				require.NoError(t, err)

				test.MineBlock()
				info := stream(boltzrpc.SwapState_PENDING)
				require.NotNil(t, info.ReverseSwap)
				require.True(t, info.ReverseSwap.IsAuto)
				id := info.ReverseSwap.Id

				swaps, err := admin.ListSwaps(&boltzrpc.ListSwapsRequest{Include: boltzrpc.IncludeSwaps_AUTO})
				require.NoError(t, err)
				// it might be the first index since we create swaps above aswell
				require.True(t, slices.ContainsFunc(swaps.ReverseSwaps, func(s *boltzrpc.ReverseSwapInfo) bool {
					return s.Id == id
				}))
				stream, _ = swapStream(t, admin, id)
				stream(boltzrpc.SwapState_SUCCESSFUL)

				status, err := autoSwap.GetStatus()
				budget := status.Lightning.Budget
				require.NoError(t, err)
				require.NotZero(t, budget.Stats.Count)
				require.Less(t, budget.Remaining, budget.Total)
				require.NotZero(t, budget.Stats.TotalFees)
				require.NotZero(t, budget.Stats.TotalAmount)
			})

		})

	})
}
