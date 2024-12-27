package nursery

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"slices"
	"time"
)

type Claimer struct {
	ExpiryTolerance time.Duration    `long:"expiry-tolerance" description:"Time before a swap expires that it should be claimed"`
	Symbols         []boltz.Currency `long:"deferred-symbols" description:"Symbols for which swaps should be deferred" default:"liquid"`
	Interval        time.Duration    `long:"claim-interval" description:"Interval at which the claimer should check for deferred swaps"`
	MaxCount        int              `long:"max-count" description:"Maximum number of outputs to claim in a single transaction" default:"200"`
	MaxBalance      uint64           `long:"max-balance" description:"Maximum number of outputs to claim in a single transaction" default:"200"`

	onchain *onchain.Onchain
	outputs map[boltz.Currency][]*CheckedOutput
}

func (claimer *Claimer) Init(onchain *onchain.Onchain) {
	claimer.onchain = onchain
	claimer.outputs = make(map[boltz.Currency][]*CheckedOutput)
}

func (nursery *Nursery) startClaimer() {
	// sweep everything at startup, because previously deferred swaps might have expired
	nursery.SweepAll(ReasonForced, []boltz.Currency{boltz.CurrencyBtc, boltz.CurrencyLiquid})
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
				nursery.SweepAll(ReasonInterval, nursery.claimer.Symbols)
			case <-nursery.ctx.Done():
				return
			}
		}
	}()
}

type CheckedOutput struct {
	*Output
	outputResult *onchain.OutputResult
}

func (claimer *Claimer) checkOutput(output *Output) *CheckedOutput {
	if output.Address == "" {
		verb := "claimed"
		if output.IsRefund() {
			verb = "refunded"
		}
		output.setError(fmt.Errorf("swap %s can not be %s, no address set", output.SwapId, verb))
		return nil
	}
	result, err := claimer.onchain.FindOutput(output.findArgs)
	if err != nil {
		output.setError(err)
		return nil
	}
	output.LockupTransaction = result.Transaction
	output.Vout = result.Vout
	return &CheckedOutput{Output: output, outputResult: result}
}

func (nursery *Nursery) checkSweep(output *Output) bool {
	if checked := nursery.claimer.checkOutput(output); checked != nil {
		if reason := nursery.claimer.shouldSweep(checked); reason != ReasonNone {
			result := nursery.sweep(reason, output.findArgs.Currency)
			return result.SwapResult(output.SwapId).Err != nil
		}
		return false
	}
	return true
}

type SweepReason string

const (
	ReasonNone     SweepReason = "none"
	ReasonExpiry   SweepReason = "expiry"
	ReasonCount    SweepReason = "count"
	ReasonAmount   SweepReason = "amount"
	ReasonInterval SweepReason = "interval"
	ReasonForced   SweepReason = "forced"
)

func (claimer *Claimer) shouldSweep(output *CheckedOutput) SweepReason {
	currency := output.findArgs.Currency
	outputs := claimer.outputs[currency]
	if !slices.ContainsFunc(outputs, func(existing *CheckedOutput) bool {
		return existing.SwapId == output.SwapId
	}) {
		claimer.outputs[currency] = append(outputs, output)
	}
	if claimer.Interval == 0 {
		return ReasonForced
	}
	blockHeight, err := claimer.onchain.GetBlockHeight(currency)
	if err != nil {
		logger.Warnf("Could not get block height for %s, forcing sweep: %s", currency, err)
		return ReasonForced
	}
	blocks := output.TimeoutBlockHeight - blockHeight
	timeout := time.Duration(boltz.BlocksToHours(blocks, currency) * float64(time.Hour))
	if timeout <= claimer.ExpiryTolerance {
		return ReasonExpiry
	}
	if len(claimer.outputs[currency]) > claimer.MaxCount {
		return ReasonCount
	}
	if claimer.SweepableBalance(currency, nil) > claimer.MaxBalance {
		return ReasonAmount
	}
	return ReasonNone
}

func (nursery *Nursery) SweepAll(reason SweepReason, symbols []boltz.Currency) {
	for _, currency := range symbols {
		if result := nursery.sweep(reason, currency); result.Err != nil {
			logger.Errorf("could not sweep %s: %s", currency, result.Err)
		}
	}
}

func (nursery *Nursery) getAllOutputs(tenantId *database.Id, currency boltz.Currency) ([]*CheckedOutput, error) {
	currentHeight, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		return nil, fmt.Errorf("could not get block height: %w", err)
	}
	var outputs []*Output
	reverseSwaps, claimableChain, err := nursery.database.QueryAllClaimableSwaps(tenantId, currency)
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
	var result []*CheckedOutput
	for _, output := range outputs {
		if checked := nursery.claimer.checkOutput(output); checked != nil {
			result = append(result, checked)
		}
	}
	return result, nil
}

func (nursery *Nursery) Sweep(currency boltz.Currency) (string, error) {
	outputs, err := nursery.getAllOutputs(nil, currency)
	if err != nil {
		return "", fmt.Errorf("could not query claimable outputs: %w", err)
	}
	nursery.claimer.outputs[currency] = outputs
	result := nursery.sweep(ReasonForced, currency)
	return result.TransactionId, result.Err
}

func (nursery *Nursery) sweep(reason SweepReason, currency boltz.Currency) boltz.ConstructResult {
	outputs := nursery.claimer.outputs[currency]
	currentHeight, err := nursery.onchain.GetBlockHeight(currency)
	if err != nil {
		logger.Warnf("Could not get block height for %s sweep, assuming all outputs to be non-coop: %s", currency, err)
	}
	for _, output := range outputs {
		output.Cooperative = currentHeight != 0 && output.TimeoutBlockHeight > currentHeight
		logger.Debugf(
			"Output for swap %s cooperative: %t (%d > %d)",
			output.SwapId, output.Cooperative, output.TimeoutBlockHeight, currentHeight,
		)
	}
	if len(outputs) > 0 {
		logger.Infof("Sweeping %d outputs for currency %s (reason: %s)", len(outputs), currency, reason)
		result := nursery.createTransaction(currency, outputs)
		if result.Err == nil {
			nursery.claimer.outputs[currency] = nil
		}
		return result
	}
	return boltz.ConstructResult{}
}

func (claimer *Claimer) SweepableBalance(currency boltz.Currency, walletId *database.Id) uint64 {
	var balance uint64
	for _, output := range claimer.outputs[currency] {
		if walletId == nil || (output.walletId != nil && *output.walletId == *walletId) {
			balance += output.outputResult.Value
		}
	}
	return balance
}
