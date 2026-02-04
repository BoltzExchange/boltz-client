//go:build !unit

package rpcserver

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/BoltzExchange/boltz-client/v2/internal/macaroons"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestMacaroons(t *testing.T) {
	admin, adminAuto, stop := setup(t, setupOptions{})
	defer stop()
	conn := admin.Connection
	global := admin
	global.SetTenant(macaroons.TenantAll)

	tenantName := "test"

	tenantInfo, write, read := createTenant(t, admin, tenantName)

	list, err := admin.ListTenants()
	require.NoError(t, err)
	require.Len(t, list.Tenants, 2)

	tenant := client.NewBoltzClient(write)
	tenantAuto := client.NewAutoSwapClient(write)
	readTenant := client.NewBoltzClient(read)

	t.Run("Reserved", func(t *testing.T) {
		_, err := admin.CreateTenant(macaroons.TenantAll)
		requireCode(t, err, codes.InvalidArgument)
	})

	t.Run("SetTenant", func(t *testing.T) {
		admin := client.NewBoltzClient(conn)

		admin.SetTenant(tenantName)
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)

		admin.SetTenant(info.Tenant.Id)
		info, err = admin.GetInfo()
		require.NoError(t, err)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)

		admin.SetTenant("invalid")
		_, err = admin.GetInfo()
		require.Error(t, err)
	})

	t.Run("Bake", func(t *testing.T) {
		response, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.FullPermissions,
		})
		require.NoError(t, err)

		anotherAdmin := client.NewBoltzClient(conn)

		anotherAdmin.SetMacaroon(response.Macaroon)

		response, err = admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.ReadPermissions,
		})
		require.NoError(t, err)

		anotherAdmin.SetMacaroon(response.Macaroon)

		// write actions are not allowed now
		_, err = anotherAdmin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			Permissions: client.ReadPermissions,
		})
		require.Error(t, err)

		err = anotherAdmin.Stop()
		require.Error(t, err)
	})

	t.Run("Admin", func(t *testing.T) {
		_, err = tenant.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
			TenantId:    &tenantInfo.Id,
			Permissions: client.ReadPermissions,
		})
		require.Error(t, err)

		_, err = tenant.GetTenant(tenantInfo.Name)
		require.Error(t, err)

		_, err = tenant.ListTenants()
		require.Error(t, err)

		_, err = admin.GetTenant(tenantInfo.Name)
		require.NoError(t, err)
	})

	t.Run("Infos", func(t *testing.T) {
		info, err := admin.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.NotNil(t, info.Tenant)

		info, err = global.GetInfo()
		require.NoError(t, err)
		require.NotEmpty(t, info.NodePubkey)
		require.Nil(t, info.Tenant)

		info, err = readTenant.GetInfo()
		require.NoError(t, err)
		require.Empty(t, info.NodePubkey)
		require.Equal(t, tenantInfo.Id, info.Tenant.Id)
	})

	t.Run("AutoSwap", func(t *testing.T) {
		_, err := adminAuto.ResetConfig(client.LnAutoSwap)
		require.NoError(t, err)
		cfg, err := adminAuto.GetLightningConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		_, err = tenantAuto.GetLightningConfig()
		require.Error(t, err)
	})

	t.Run("Wallet", func(t *testing.T) {
		testWallet := fundedWallet(t, admin, boltzrpc.Currency_LBTC)
		hasWallets := func(t *testing.T, client client.Boltz, amount int) {
			wallets, err := client.GetWallets(nil, true)
			require.NoError(t, err)
			require.Len(t, wallets.Wallets, amount)
		}
		hasWallets(t, tenant, 0)
		hasWallets(t, admin, 2)

		_, err = tenant.GetWallet(testWallet.Name)
		requireCode(t, err, codes.NotFound)

		walletParams := &boltzrpc.WalletParams{Currency: boltzrpc.Currency_LBTC, Name: "test"}
		_, err = readTenant.CreateWallet(walletParams)
		requireCode(t, err, codes.PermissionDenied)

		_, err = tenant.CreateWallet(walletParams)
		requireCode(t, err, codes.OK)

		hasWallets(t, tenant, 1)
		hasWallets(t, admin, 2)
		hasWallets(t, global, 3)
	})

	t.Run("RemoveTenant", func(t *testing.T) {
		t.Run("NonExistent", func(t *testing.T) {
			err := admin.RemoveTenant("nonexistent")
			requireCode(t, err, codes.NotFound)
			require.ErrorContains(t, err, "does not exist")
		})

		t.Run("Default", func(t *testing.T) {
			err := admin.RemoveTenant("admin")
			requireCode(t, err, codes.InvalidArgument)
			require.ErrorContains(t, err, "default tenant")
		})

		t.Run("WithWallets", func(t *testing.T) {
			err := admin.RemoveTenant(tenantName)
			requireCode(t, err, codes.FailedPrecondition)
			require.ErrorContains(t, err, "associated wallets")
		})

		t.Run("WithoutWallets", func(t *testing.T) {
			emptyTenantName := "empty-tenant"
			emptyTenant, err := admin.CreateTenant(emptyTenantName)
			require.NoError(t, err)
			require.NotZero(t, emptyTenant.Id)

			_, err = admin.GetTenant(emptyTenantName)
			require.NoError(t, err)

			err = admin.RemoveTenant(emptyTenantName)
			require.NoError(t, err)

			_, err = admin.GetTenant(emptyTenantName)
			requireCode(t, err, codes.NotFound)
		})

		t.Run("Permission", func(t *testing.T) {
			removableTenant := "removable-tenant"
			_, err := admin.CreateTenant(removableTenant)
			require.NoError(t, err)

			err = tenant.RemoveTenant(removableTenant)
			requireCode(t, err, codes.PermissionDenied)

			err = admin.RemoveTenant(removableTenant)
			require.NoError(t, err)
		})
	})

	t.Run("Swaps", func(t *testing.T) {
		hasSwaps := func(t *testing.T, client client.Boltz, length int) {
			swaps, err := client.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Len(t, swaps.Swaps, length)
			require.Len(t, swaps.ReverseSwaps, length)
			//require.Len(t, swaps.ChainSwaps, length)
		}

		t.Run("Admin", func(t *testing.T) {
			hasSwaps(t, admin, 0)
			_, err = admin.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.NoError(t, err)
			externalPay := false
			_, err = admin.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
			require.NoError(t, err)
			hasSwaps(t, admin, 1)
			hasSwaps(t, tenant, 0)
		})

		t.Run("Tenant", func(t *testing.T) {
			hasSwaps(t, tenant, 0)
			_, err = tenant.CreateSwap(&boltzrpc.CreateSwapRequest{})
			require.ErrorContains(t, err, "invoice is required in standalone mode")
			externalPay := false
			_, err = tenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay})
			require.ErrorContains(t, err, "can not create reverse swap without external pay in standalone mode")
			hasSwaps(t, tenant, 0)
			externalPay = true
			_, err = tenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000, ExternalPay: &externalPay, Address: test.BtcCli("getnewaddress")})
			require.NoError(t, err)
			swaps, err := tenant.ListSwaps(&boltzrpc.ListSwapsRequest{})
			require.NoError(t, err)
			require.Len(t, swaps.ReverseSwaps, 1)
		})

		t.Run("Read", func(t *testing.T) {
			_, err = readTenant.CreateSwap(&boltzrpc.CreateSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{Amount: 100000})
			requireCode(t, err, codes.PermissionDenied)
			_, err = readTenant.CreateChainSwap(&boltzrpc.CreateChainSwapRequest{})
			requireCode(t, err, codes.PermissionDenied)
		})
	})
}

func TestPasswordAuth(t *testing.T) {
	cfg := loadConfig(t)
	cfg.Standalone = true
	cfg.RPC.Password = "testpassword"

	client, _, stop := setup(t, setupOptions{cfg: cfg})
	defer stop()

	t.Run("Success", func(t *testing.T) {
		info, err := client.GetInfo()
		require.NoError(t, err)
		require.Equal(t, "regtest", info.Network)
	})

	t.Run("WrongPassword", func(t *testing.T) {
		wrongClient := client
		wrongClient.SetPassword("wrongpassword")
		_, err := wrongClient.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unauthenticated)
	})

	t.Run("NoPassword", func(t *testing.T) {
		noPasswordClient := client
		noPasswordClient.SetPassword("")
		_, err := noPasswordClient.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unauthenticated)
	})
}
