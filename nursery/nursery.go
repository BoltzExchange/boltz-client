package nursery

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
)

type Nursery struct {
	symbol      string
	boltzPubKey string

	chainParams *chaincfg.Params

	lnd      *lnd.LND
	boltz    *boltz.Boltz
	database *database.Database
}

const retryInterval = 15

// Map between Swap ids and a channel that tells its SSE event listeners to stop
var eventListeners = make(map[string]chan bool)
var eventListenersLock sync.RWMutex

func (nursery *Nursery) Init(
	symbol string,
	boltzPubKey string,
	chainParams *chaincfg.Params,
	lnd *lnd.LND,
	boltz *boltz.Boltz,
	database *database.Database,
) error {
	nursery.symbol = symbol
	nursery.boltzPubKey = boltzPubKey

	nursery.chainParams = chainParams

	nursery.lnd = lnd
	nursery.boltz = boltz
	nursery.database = database

	logger.Info("Starting nursery")

	// TODO: use channel acceptor to prevent invalid channel openings from happening

	blockNotifier := make(chan *chainrpc.BlockEpoch)
	go nursery.registerBlockListener(blockNotifier)

	err := nursery.recoverSwaps(blockNotifier)

	if err != nil {
		return err
	}

	err = nursery.recoverReverseSwaps()

	return err
}

func (nursery *Nursery) registerBlockListener(blockNotifier chan *chainrpc.BlockEpoch) {
	logger.Info("Connecting to LND block epoch stream")
	err := nursery.lnd.RegisterBlockListener(blockNotifier)

	if err != nil {
		logger.Error("Lost connection to LND block epoch stream: " + err.Error())
		logger.Info("Retrying LND connection in " + strconv.Itoa(retryInterval) + " seconds")

		time.Sleep(retryInterval * time.Second)

		nursery.registerBlockListener(blockNotifier)
	}
}

func (nursery *Nursery) findLockupVout(addressToFind string, outputs []*wire.TxOut) (uint32, error) {
	for vout, output := range outputs {
		_, outputAddresses, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, nursery.chainParams)

		// Just ignore outputs we can't decode
		if err != nil {
			continue
		}

		for _, outputAddress := range outputAddresses {
			if outputAddress.EncodeAddress() == addressToFind {
				return uint32(vout), nil
			}
		}
	}

	return 0, errors.New("could not find lockup vout")
}

// TODO: test behaviour on testnet / mainnet
func (nursery *Nursery) getFeeEstimation() (int64, error) {
	feeResponse, err := nursery.lnd.EstimateFee(2)

	if err != nil {
		return 0, err
	}

	// Divide by 4 to get the fee per kilo vbyte and by 1000 to get the fee per vbyte
	return maxInt64(int64(math.Round(float64(feeResponse.SatPerKw)/4000)), 2), nil
}

func (nursery *Nursery) broadcastTransaction(transaction *wire.MsgTx) error {
	transactionHex, err := boltz.SerializeTransaction(transaction)

	if err != nil {
		return errors.New("could not serialize transaction: " + err.Error())
	}

	_, err = nursery.boltz.BroadcastTransaction(transactionHex)

	if err != nil {
		return errors.New("could not broadcast transaction: " + err.Error())
	}

	logger.Info("Broadcast transaction with Boltz API")

	return nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}

	return b
}
