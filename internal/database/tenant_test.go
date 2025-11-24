package database_test

import (
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/stretchr/testify/require"
)

func TestTenantOperations(t *testing.T) {
	db := database.Database{Path: ":memory:"}
	err := db.Connect()
	require.NoError(t, err)

	t.Run("CreateTenant", func(t *testing.T) {
		tenant := &database.Tenant{Name: "test-tenant"}
		err := db.CreateTenant(tenant)
		require.NoError(t, err)
		require.NotZero(t, tenant.Id)

		retrieved, err := db.GetTenant(tenant.Id)
		require.NoError(t, err)
		require.Equal(t, tenant.Name, retrieved.Name)
		require.Equal(t, tenant.Id, retrieved.Id)
	})

	t.Run("GetTenantByName", func(t *testing.T) {
		tenant := &database.Tenant{Name: "named-tenant"}
		err := db.CreateTenant(tenant)
		require.NoError(t, err)

		retrieved, err := db.GetTenantByName("named-tenant")
		require.NoError(t, err)
		require.Equal(t, tenant.Id, retrieved.Id)
		require.Equal(t, "named-tenant", retrieved.Name)

		_, err = db.GetTenantByName("nonexistent")
		require.Error(t, err)
	})

	t.Run("QueryTenants", func(t *testing.T) {
		tenants, err := db.QueryTenants()
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(tenants), 3)
	})

	t.Run("HasTenantWallets", func(t *testing.T) {
		tenantNoWallets := &database.Tenant{Name: "tenant-no-wallets"}
		err := db.CreateTenant(tenantNoWallets)
		require.NoError(t, err)

		hasWallets, err := db.HasTenantWallets(tenantNoWallets.Id)
		require.NoError(t, err)
		require.False(t, hasWallets)

		tenantWithWallets := &database.Tenant{Name: "tenant-with-wallets"}
		err = db.CreateTenant(tenantWithWallets)
		require.NoError(t, err)

		wallet := &database.Wallet{
			WalletCredentials: &onchain.WalletCredentials{
				WalletInfo: onchain.WalletInfo{
					Name:     "test-wallet",
					Currency: boltz.CurrencyBtc,
					TenantId: tenantWithWallets.Id,
				},
			},
		}
		err = db.CreateWallet(wallet)
		require.NoError(t, err)

		hasWallets, err = db.HasTenantWallets(tenantWithWallets.Id)
		require.NoError(t, err)
		require.True(t, hasWallets)
	})

	t.Run("DeleteTenant", func(t *testing.T) {
		tenant := &database.Tenant{Name: "tenant-to-delete"}
		err := db.CreateTenant(tenant)
		require.NoError(t, err)

		err = db.DeleteTenant(int64(tenant.Id))
		require.NoError(t, err)

		_, err = db.GetTenant(tenant.Id)
		require.Error(t, err)

		err = db.DeleteTenant(99999)
		require.Error(t, err)
	})

	t.Run("DefaultTenant", func(t *testing.T) {
		tenant, err := db.GetTenant(database.DefaultTenantId)
		require.NoError(t, err)
		require.Equal(t, database.DefaultTenantName, tenant.Name)
		require.Equal(t, database.DefaultTenantId, tenant.Id)
	})
}
