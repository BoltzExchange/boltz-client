package nursery

import (
	"database/sql"
	"fmt"

	dbpkg "github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

func (nursery *Nursery) RegisterFundingAddress(fa *dbpkg.FundingAddress) error {
	return nursery.boltzWs.SubscribeFunding([]string{fa.Id})
}

func (nursery *Nursery) startFundingListener() {
	logger.Infof("Starting funding address update listener")

	nursery.waitGroup.Add(1)

	go func() {
		defer nursery.waitGroup.Done()
		for update := range nursery.boltzWs.FundingUpdates {
			logger.Debugf("Funding address %s status update: %s", update.Id, update.Status)
			if err := nursery.processFundingUpdate(update); err != nil {
				logger.Errorf("Could not process funding update: %v", err)
			}
		}
	}()
}

func (nursery *Nursery) processFundingUpdate(update boltz.FundingUpdate) error {
	logger.Debugf("Processing funding update: %s", update.Id)
	event := boltz.ParseFundingEvent(update.Status)
	fa, err := nursery.database.QueryFundingAddress(update.Id)
	if err != nil {
		return fmt.Errorf("could not query funding address %s: %v", update.Id, err)
	}

	// Update status
	if err := nursery.database.UpdateFundingAddressStatus(fa, update.Status); err != nil {
		return fmt.Errorf("could not update funding address status: %v", err)
	}

	if update.Transaction != nil && update.Transaction.Id != "" {
		lockupTransaction, err := boltz.NewTxFromHex(fa.Currency, update.Transaction.Hex, fa.BlindingKey)
		if err != nil {
			return fmt.Errorf("could not deserialize lockup transaction: %v", err)
		}
		_, amount, err := lockupTransaction.FindVout(nursery.network, fa.Address)
		if err != nil {
			return fmt.Errorf("could not find funding output in lockup transaction: %v", err)
		}
		if err := nursery.database.SetFundingAddressLockupTransaction(fa, update.Transaction.Id, amount); err != nil {
			return fmt.Errorf("could not update funding address lockup transaction: %v", err)
		}
	}

	if update.SwapId != "" {
		if err := nursery.database.SetFundingAddressSwapId(fa, update.SwapId); err != nil {
			return fmt.Errorf("could not update funding address swap id: %v", err)
		}
	}

	nursery.FundingAddressUpdates.Send(fa)

	if event.IsFinalStatus() {
		nursery.boltzWs.UnsubscribeFunding(update.Id)
	}

	return nil
}

func (nursery *Nursery) getSwapLockupAddress(swapId string) (string, error) {
	swap, err := nursery.database.QuerySwap(swapId)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("could not query swap %s: %v", swapId, err)
	}
	if swap != nil {
		return swap.Address, nil
	}

	chainSwap, err := nursery.database.QueryChainSwap(swapId)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("could not query reverse swap %s: %v", swapId, err)
	}
	if chainSwap != nil {
		return chainSwap.FromData.LockupAddress, nil
	}

	return "", fmt.Errorf("swap %s not found", swapId)
}

func (nursery *Nursery) SignFundingAddress(fundingAddress *dbpkg.FundingAddress) error {
	// Get signing details from Boltz API
	signingDetails, err := nursery.boltz.GetFundingAddressSigningDetails(fundingAddress.Id, fundingAddress.SwapId)
	if err != nil {
		return fmt.Errorf("failed to get signing details from boltz: %v", err)
	}

	fundingTree, err := fundingAddress.GetFundingTree()
	if err != nil {
		return fmt.Errorf("failed to create funding tree: %v", err)
	}
	// Create signing session
	session, err := boltz.NewFundingSigningSession(fundingTree)
	if err != nil {
		return fmt.Errorf("failed to create signing session: %v", err)
	}

	lockupAddress, err := nursery.getSwapLockupAddress(fundingAddress.SwapId)
	if err != nil {
		return fmt.Errorf("failed to get swap lockup address: %v", err)
	}

	lockupTransaction, err := nursery.onchain.GetTransaction(fundingAddress.Currency, fundingAddress.LockupTransactionId, fundingAddress.BlindingKey, false)
	if err != nil {
		return fmt.Errorf("failed to get lockup transaction: %v", err)
	}

	partial, err := session.PresignTransaction(nursery.network, lockupTransaction, lockupAddress, signingDetails)
	if err != nil {
		return fmt.Errorf("failed to presign transaction: %v", err)
	}

	// Send our partial signature to Boltz
	if err := nursery.boltz.SendFundingAddressSignature(fundingAddress.Id, partial); err != nil {
		return fmt.Errorf("failed to send signature to boltz: %v", err)
	}

	// Update funding address with the swap ID
	if err := nursery.database.SetFundingAddressSwapId(fundingAddress, fundingAddress.SwapId); err != nil {
		logger.Warnf("Failed to update funding address %s with swap ID: %v", fundingAddress.Id, err)
	}

	return nil
}

func (nursery *Nursery) recoverFundingAddresses() error {
	addresses, err := nursery.database.QueryPendingFundingAddresses()
	if err != nil {
		return err
	}

	if len(addresses) == 0 {
		return nil
	}

	var fundingIds []string
	for _, fa := range addresses {
		fundingIds = append(fundingIds, fa.Id)
	}

	return nursery.boltzWs.SubscribeFunding(fundingIds)
}
