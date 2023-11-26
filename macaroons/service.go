package macaroons

import (
	"context"
	"github.com/BoltzExchange/boltz-client/database"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon.v2"
)

var defaultRootKeyID = []byte("0")

type Service struct {
	Database *database.Database

	bakery *bakery.Bakery
}

func (service *Service) Init() {
	rootKeyStorage := RootKeyStorage{
		database: service.Database,
	}

	macaroonParams := bakery.BakeryParams{
		Location:     "boltz",
		RootKeyStore: &rootKeyStorage,
	}

	service.bakery = bakery.New(macaroonParams)
}

func (service *Service) NewMacaroon(ops ...bakery.Op) (*bakery.Macaroon, error) {
	ctx := addRootKeyIdToContext(context.Background(), defaultRootKeyID)

	return service.bakery.Oven.NewMacaroon(ctx, bakery.LatestVersion, nil, ops...)
}

func (service *Service) ValidateMacaroon(macBytes []byte, requiredPermissions []bakery.Op) error {
	mac := &macaroon.Macaroon{}
	err := mac.UnmarshalBinary(macBytes)

	if err != nil {
		return err
	}

	authChecker := service.bakery.Checker.Auth(macaroon.Slice{mac})
	_, err = authChecker.Allow(context.Background(), requiredPermissions...)

	return err
}
