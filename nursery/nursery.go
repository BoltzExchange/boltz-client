package nursery

import (
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
)

type Nursery struct {
	symbol      string
	chainParams *chaincfg.Params

	lnd      *lnd.LND
	boltz    *boltz.Boltz
	database *database.Database
}

func (nursery *Nursery) Init(symbol string, chainParams *chaincfg.Params, lnd *lnd.LND, boltz *boltz.Boltz, database *database.Database) error {
	nursery.symbol = symbol
	nursery.chainParams = chainParams

	nursery.lnd = lnd
	nursery.boltz = boltz
	nursery.database = database

	logger.Info("Starting Swap nursery")

	blockNotifier := make(chan *chainrpc.BlockEpoch)
	err := nursery.lnd.RegisterBlockListener(blockNotifier)

	if err != nil {
		return err
	}

	err = nursery.recoverSwaps(blockNotifier)

	return err
}
