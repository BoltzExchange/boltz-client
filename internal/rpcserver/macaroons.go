package rpcserver

import (
	"os"

	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/macaroons"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

func (server *RpcServer) generateMacaroons(database *database.Database) (*macaroons.Service, error) {
	logger.Info("Enabling Macaroon authentication")

	service := macaroons.Service{
		Database: database,
	}

	service.Init()

	cfg := server.cfg.RPC

	if utils.FileExists(cfg.AdminMacaroonPath) && utils.FileExists(cfg.ReadonlyMacaroonPath) {
		adminMac, err := os.ReadFile(cfg.AdminMacaroonPath)
		if err != nil {
			return nil, err
		}

		readMac, err := os.ReadFile(cfg.ReadonlyMacaroonPath)
		if err != nil {
			return nil, err
		}

		valid := true
		if _, err := service.ValidateMacaroon(adminMac, macaroons.AdminPermissions()); err != nil {
			valid = false
		}

		if _, err := service.ValidateMacaroon(readMac, macaroons.ReadPermissions); err != nil {
			valid = false
		}

		if valid {
			return &service, nil
		}
		logger.Warn("Macaroons are outdated")
	} else {
		logger.Warn("Could not find Macaroons")
	}
	logger.Info("Generating new Macaroons")

	err := writeMacaroon(service, nil, macaroons.AdminPermissions(), cfg.AdminMacaroonPath)

	if err != nil {
		return nil, err
	}

	err = writeMacaroon(service, nil, macaroons.ReadPermissions, cfg.ReadonlyMacaroonPath)

	if err != nil {
		return nil, err
	}

	return &service, nil
}

func writeMacaroon(service macaroons.Service, tenantId *database.Id, permissions []bakery.Op, path string) error {
	macaroon, err := service.NewMacaroon(tenantId, permissions...)

	if err != nil {
		return err
	}

	macaroonBytes, err := macaroon.M().MarshalBinary()

	if err != nil {
		return err
	}

	return os.WriteFile(path, macaroonBytes, 0600)
}
