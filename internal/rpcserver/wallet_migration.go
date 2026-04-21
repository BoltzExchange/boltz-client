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

func migrateLegacyBitcoinWallet(credentials *onchain.WalletCredentials, network *boltz.Network) (bool, string, error) {
	if credentials.CoreDescriptor != "" {
		credentials.Subaccount = nil
		credentials.Legacy = false
		return true, "", nil
	}
	if credentials.Mnemonic == "" {
		return false, manualWalletMigrationWarning(credentials, "missing mnemonic"), nil
	}
	if credentials.Subaccount == nil {
		return false, manualWalletMigrationWarning(credentials, "missing legacy subaccount metadata"), nil
	}
	if *credentials.Subaccount != legacyDefaultSinglesigSubaccount {
		return false, manualWalletMigrationWarning(credentials, fmt.Sprintf("unsupported legacy subaccount %d", *credentials.Subaccount)), nil
	}

	descriptor, err := bitcoin_wallet.DeriveDefaultDescriptor(network, credentials.Mnemonic)
	if err != nil {
		return false, manualWalletMigrationWarning(credentials, err.Error()), nil
	}
	credentials.CoreDescriptor = descriptor
	credentials.Subaccount = nil
	credentials.Legacy = false
	return true, "", nil
}

func migrateLegacyLiquidWallet(credentials *onchain.WalletCredentials, network *boltz.Network) (bool, string, error) {
	if credentials.CoreDescriptor != "" {
		credentials.Subaccount = nil
		credentials.Legacy = false
		return true, "", nil
	}
	if credentials.Mnemonic == "" {
		return false, manualWalletMigrationWarning(credentials, "missing mnemonic"), nil
	}
	if credentials.Subaccount == nil {
		return false, manualWalletMigrationWarning(credentials, "missing legacy subaccount metadata"), nil
	}
	if *credentials.Subaccount != legacyDefaultSinglesigSubaccount {
		return false, manualWalletMigrationWarning(credentials, fmt.Sprintf("unsupported legacy subaccount %d", *credentials.Subaccount)), nil
	}

	descriptor, err := liquid_wallet.DeriveDefaultDescriptor(network, credentials.Mnemonic)
	if err != nil {
		return false, manualWalletMigrationWarning(credentials, err.Error()), nil
	}
	credentials.CoreDescriptor = descriptor
	credentials.Subaccount = nil
	credentials.Legacy = false
	return true, "", nil
}

func (server *routedBoltzServer) migrateWalletCredentials(credentials []*onchain.WalletCredentials) (bool, []string, error) {
	var warnings []string
	changed := false
	for _, creds := range credentials {
		if !creds.Legacy {
			continue
		}

		var (
			migrated bool
			warning  string
			err      error
		)
		switch creds.Currency {
		case boltz.CurrencyBtc:
			migrated, warning, err = migrateLegacyBitcoinWallet(creds, server.network)
		case boltz.CurrencyLiquid:
			migrated, warning, err = migrateLegacyLiquidWallet(creds, server.network)
		default:
			warning = manualWalletMigrationWarning(creds, "unsupported currency")
		}
		if err != nil {
			return false, nil, err
		}
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
	return changed, warnings, nil
}

func (server *routedBoltzServer) setWalletMigrationWarnings(warnings []string) {
	server.walletMigrationWarningsLock.Lock()
	defer server.walletMigrationWarningsLock.Unlock()
	server.walletMigrationWarnings = append(server.walletMigrationWarnings[:0], warnings...)
}

func (server *routedBoltzServer) getWalletMigrationWarnings() []string {
	server.walletMigrationWarningsLock.RLock()
	defer server.walletMigrationWarningsLock.RUnlock()
	return append([]string(nil), server.walletMigrationWarnings...)
}
