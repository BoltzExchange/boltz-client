package rpcserver

import (
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/macaroons"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"io/ioutil"
)

func (server *RpcServer) generateMacaroons(database *database.Database) (*macaroons.Service, error) {
	logger.Info("Enabling Macaroon authentication")

	service := macaroons.Service{
		Database: database,
	}

	service.Init()

	if utils.FileExists(server.AdminMacaroonPath) && utils.FileExists(server.ReadonlyMacaroonPath) {
		// TODO: check if the macaroons on the disk are still up to date
		return &service, nil
	}

	logger.Warning("Could not find Macaroons")
	logger.Info("Generating new Macaroons")

	err := writeMacaroon(service, macaroons.AdminPermissions(), server.AdminMacaroonPath)

	if err != nil {
		return nil, err
	}

	err = writeMacaroon(service, macaroons.ReadPermissions, server.ReadonlyMacaroonPath)

	if err != nil {
		return nil, err
	}

	return &service, nil
}

func writeMacaroon(service macaroons.Service, permissions []bakery.Op, path string) error {
	macaroon, err := service.NewMacaroon(permissions...)

	if err != nil {
		return err
	}

	macaroonBytes, err := macaroon.M().MarshalBinary()

	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, macaroonBytes, 0600)
}
