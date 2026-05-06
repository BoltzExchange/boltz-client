package rpcserver

import (
	"context"
	"fmt"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/macaroons"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	bitcoin_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

const legacyDefaultSinglesigSubaccount = 1

type walletMigrationWarning struct {
	walletId database.Id
	tenantId database.Id
	message  string
}

func manualWalletMigrationWarning(credentials *onchain.WalletCredentials, reason string) string {
	return fmt.Sprintf(
		"Could not automatically migrate wallet %q (%s): %s. Please manually re-import it.",
		credentials.Name,
		credentials.Currency,
		reason,
	)
}

func migrateLegacyWallet(credentials *onchain.WalletCredentials, network *boltz.Network) (bool, string) {
	var deriveDescriptor func(*boltz.Network, string) (string, error)
	switch credentials.Currency {
	case boltz.CurrencyBtc:
		deriveDescriptor = bitcoin_wallet.DeriveDefaultDescriptor
	case boltz.CurrencyLiquid:
		deriveDescriptor = liquid_wallet.DeriveDefaultDescriptor
	default:
		return false, manualWalletMigrationWarning(credentials, "unsupported currency")
	}
	if credentials.CoreDescriptor == "" {
		if credentials.Mnemonic == "" {
			return false, manualWalletMigrationWarning(credentials, "missing mnemonic")
		}
		if credentials.Subaccount == nil {
			return false, manualWalletMigrationWarning(credentials, "missing legacy subaccount metadata")
		}
		if *credentials.Subaccount != legacyDefaultSinglesigSubaccount {
			return false, manualWalletMigrationWarning(credentials, fmt.Sprintf("unsupported legacy subaccount %d", *credentials.Subaccount))
		}
		descriptor, err := deriveDescriptor(network, credentials.Mnemonic)
		if err != nil {
			return false, manualWalletMigrationWarning(credentials, err.Error())
		}
		credentials.CoreDescriptor = descriptor
	}
	credentials.Subaccount = nil
	credentials.Legacy = false
	return true, ""
}

func (server *routedBoltzServer) migrateWalletCredentials(credentials []*onchain.WalletCredentials) (bool, []walletMigrationWarning) {
	var warnings []walletMigrationWarning
	changed := false
	for _, creds := range credentials {
		if !creds.Legacy {
			continue
		}

		migrated, warning := migrateLegacyWallet(creds, server.network)
		if warning != "" {
			warnings = append(warnings, walletMigrationWarning{
				walletId: creds.Id,
				tenantId: creds.TenantId,
				message:  warning,
			})
			logger.Warn(warning)
			continue
		}
		if migrated {
			changed = true
			logger.Infof("Migrated legacy GDK wallet %q (%s) to descriptor credentials", creds.Name, creds.Currency)
		}
	}
	return changed, warnings
}

func (server *routedBoltzServer) setWalletMigrationWarnings(warnings []walletMigrationWarning) {
	server.walletMigrationWarningsLock.Lock()
	defer server.walletMigrationWarningsLock.Unlock()
	server.walletMigrationWarnings = warnings
}

func (server *routedBoltzServer) removeWalletMigrationWarning(walletId database.Id) {
	server.walletMigrationWarningsLock.Lock()
	defer server.walletMigrationWarningsLock.Unlock()

	filtered := server.walletMigrationWarnings[:0]
	for _, warning := range server.walletMigrationWarnings {
		if warning.walletId != walletId {
			filtered = append(filtered, warning)
		}
	}
	server.walletMigrationWarnings = filtered
}

func (server *routedBoltzServer) getWalletMigrationWarnings(ctx context.Context) []string {
	tenantId := macaroons.TenantIdFromContext(ctx)
	includeAll := tenantId == nil || *tenantId == database.DefaultTenantId

	server.walletMigrationWarningsLock.RLock()
	defer server.walletMigrationWarningsLock.RUnlock()

	warnings := make([]string, 0, len(server.walletMigrationWarnings))
	for _, warning := range server.walletMigrationWarnings {
		if !includeAll && warning.tenantId != *tenantId {
			continue
		}
		warnings = append(warnings, warning.message)
	}
	return warnings
}
