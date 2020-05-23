package nursery

import (
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/logger"
)

type Nursery struct {
	chainParams *chaincfg.Params

	lnd      *lnd.LND
	boltz    *boltz.Boltz
	database *database.Database
}

func (nursery *Nursery) Init(chainParams *chaincfg.Params, lnd *lnd.LND, boltz *boltz.Boltz, database *database.Database) error {
	nursery.chainParams = chainParams
	nursery.lnd = lnd
	nursery.boltz = boltz
	nursery.database = database

	logger.Info("Starting Swap nursery")

	return nursery.recoverSwaps()
}
