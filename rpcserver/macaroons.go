package rpcserver

import (
	"os"

	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/macaroons"
	"github.com/BoltzExchange/boltz-client/utils"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

func (server *RpcServer) generateMacaroons(database *database.Database) (*macaroons.Service, error) {
	logger.Info("Enabling Macaroon authentication")

	service := macaroons.Service{
		Database: database,
	}

	service.Init()

	if utils.FileExists(server.AdminMacaroonPath) && utils.FileExists(server.ReadonlyMacaroonPath) {
		adminMac, err := os.ReadFile(server.AdminMacaroonPath)
		if err != nil {
			return nil, err
		}

		readMac, err := os.ReadFile(server.ReadonlyMacaroonPath)
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

	err := writeMacaroon(service, nil, macaroons.AdminPermissions(), server.AdminMacaroonPath)

	if err != nil {
		return nil, err
	}

	err = writeMacaroon(service, nil, macaroons.ReadPermissions, server.ReadonlyMacaroonPath)

	if err != nil {
		return nil, err
	}

	return &service, nil
}

func writeMacaroon(service macaroons.Service, entityId *database.Id, permissions []bakery.Op, path string) error {
	macaroon, err := service.NewMacaroon(entityId, permissions...)

	if err != nil {
		return err
	}

	macaroonBytes, err := macaroon.M().MarshalBinary()

	if err != nil {
		return err
	}

	return os.WriteFile(path, macaroonBytes, 0600)
}
