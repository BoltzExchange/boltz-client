package rpcserver

import (
	"fmt"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	bitcoin_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

const legacyDefaultSinglesigSubaccount = 1

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

func (server *routedBoltzServer) migrateWalletCredentials(credentials []*onchain.WalletCredentials) (bool, []string) {
	var warnings []string
	changed := false
	for _, creds := range credentials {
		if !creds.Legacy {
			continue
		}

		migrated, warning := migrateLegacyWallet(creds, server.network)
		if warning != "" {
			warnings = append(warnings, warning)
			logger.Warn(warning)
			continue
		}
		if migrated {
			changed = true
			logger.Infof("Migrated wallet %q (%s) away from deprecated legacy credentials", creds.Name, creds.Currency)
		}
	}
	return changed, warnings
}
