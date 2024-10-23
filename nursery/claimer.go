package nursery

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"time"
)

type Claimer struct {
	ExpiryTolerance time.Duration    `long:"expiry-tolerance" description:"Time before a swap expires that it should be claimed"`
	Symbols         []boltz.Currency `long:"deferred-symbols" description:"Symbols for which swaps should be deferred" default:"liquid"`
	Interval        time.Duration    `long:"claim-interval" description:"Interval at which the claimer should check for deferred swaps"`

	onchain *onchain.Onchain
}

func (nursery *Nursery) startClaimer() {
	// sweep everything at startup, because previously deferred swaps might have expired
	nursery.SweepAll([]boltz.Currency{boltz.CurrencyBtc, boltz.CurrencyLiquid})

	if nursery.claimer.Interval == 0 {
		logger.Infof("Deferred Claimer disabled")
		return
	}
	logger.Infof("Starting deferred claimer for symbols: %s", nursery.claimer.Symbols)

	nursery.waitGroup.Add(1)
	go func() {
		defer nursery.waitGroup.Done()
		ticker := time.NewTicker(nursery.claimer.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				nursery.SweepAll(nursery.claimer.Symbols)
			case <-nursery.ctx.Done():
				return
			}
		}
	}()
}

func (nursery *Nursery) checkSweep(tenantId database.Id, currency boltz.Currency, timeoutBlockHeight uint32) error {
	if nursery.claimer.shouldSweep(currency, timeoutBlockHeight) {
		_, err := nursery.Sweep(&tenantId, currency)
		return err
	}
	return nil
}

func (nursery *Claimer) shouldSweep(currency boltz.Currency, timeoutBlockHeight uint32) bool {
	if nursery.Interval == 0 {
		return true
	}
	blockHeight, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		logger.Warnf("Could not get block height for %s, forcing sweep: %s", currency, err)
		return true
	}
	blocks := timeoutBlockHeight - blockHeight
	timeout := time.Duration(boltz.BlocksToHours(blocks, currency) * float64(time.Hour))
	return timeout > nursery.ExpiryTolerance
}

func (nursery *Nursery) SweepAll(symbols []boltz.Currency) {
	for _, currency := range symbols {
		if _, err := nursery.Sweep(nil, currency); err != nil {
			logger.Errorf("could not sweep %s: %s", currency, err)
		}
	}
}

func (nursery *Nursery) queryAllClaimableOutputs(tenantId *database.Id, currency boltz.Currency) ([]*Output, error) {
	currentHeight, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		return nil, fmt.Errorf("could not get block height: %w", err)
	}
	var outputs []*Output
	reverseSwaps, claimableChain, err := nursery.database.QueryAllClaimableSwaps(tenantId, currency, currentHeight)
	if err != nil {
		return nil, fmt.Errorf("could not query claimable swaps: %w", err)
	}
	for _, swap := range reverseSwaps {
		outputs = append(outputs, nursery.getReverseSwapClaimOutput(swap))
	}
	for _, swap := range claimableChain {
		outputs = append(outputs, nursery.getChainSwapClaimOutput(swap))
	}
	submarineSwaps, refundableChain, err := nursery.database.QueryAllRefundableSwaps(tenantId, currency, currentHeight)
	if err != nil {
		return nil, fmt.Errorf("could not query refundable swaps: %w", err)
	}
	for _, swap := range submarineSwaps {
		outputs = append(outputs, nursery.getRefundOutput(swap))
	}
	for _, swap := range refundableChain {
		outputs = append(outputs, nursery.getChainSwapRefundOutput(swap))
	}
	for _, output := range outputs {
		output.Cooperative = output.TimeoutBlockHeight > currentHeight
		logger.Debugf(
			"Output for swap %s cooperative: %t (%d > %d)",
			output.SwapId, output.Cooperative, output.TimeoutBlockHeight, currentHeight,
		)
	}
	return outputs, nil
}

func (nursery *Nursery) Sweep(tenantId *database.Id, currency boltz.Currency) (string, error) {
	outputs, err := nursery.queryAllClaimableOutputs(tenantId, currency)
	if err != nil {
		return "", fmt.Errorf("could not query claimable outputs: %w", err)
	}

	if len(outputs) > 0 {
		logger.Infof("Sweeping %d outputs for currency %s", len(outputs), currency)
		return nursery.createTransaction(currency, outputs)
	}
	return "", nil
}

/*
func (nursery *Nursery) SweepableBalance(walletId *database.Id) (uint64, error) {
	outputs, err := nursery.queryAllClaimableOutputs(boltz.CurrencyBtc)
	if err != nil {
		return 0, fmt.Errorf("could not query claimable outputs: %w", err)
	}
	var balance uint64
	for _, output := range outputs {
		if walletId == nil || (output.walletId != nil && *output.walletId == *walletId) {
			//balance += output.voutInfo.valu
		}
	}
	return balance
}
*/
