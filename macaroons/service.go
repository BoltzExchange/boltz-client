package macaroons

import (
	"context"
	"fmt"
	"github.com/BoltzExchange/boltz-client/database"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon-bakery.v2/bakery/checkers"
	"gopkg.in/macaroon.v2"
	"strconv"
)

var defaultRootKeyID = []byte("abcdef")

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

func (service *Service) NewMacaroon(entity *database.Id, ops ...bakery.Op) (*bakery.Macaroon, error) {
	ctx := addRootKeyIdToContext(context.Background(), defaultRootKeyID)

	var caveats []checkers.Caveat
	if entity != nil {
		caveats = append(caveats, checkers.DeclaredCaveat(string(entityContextKey), fmt.Sprint(*entity)))
	}

	return service.bakery.Oven.NewMacaroon(ctx, bakery.LatestVersion, caveats, ops...)
}

func (service *Service) ValidateMacaroon(macBytes []byte, requiredPermissions []bakery.Op) (*bakery.AuthInfo, error) {
	mac := &macaroon.Macaroon{}
	err := mac.UnmarshalBinary(macBytes)

	if err != nil {
		return nil, err
	}

	authChecker := service.bakery.Checker.Auth(macaroon.Slice{mac})
	return authChecker.Allow(context.Background(), requiredPermissions...)
}

func (service *Service) addEntityToContext(ctx context.Context, raw string) (context.Context, error) {
	id := database.DefaultEntityId
	if raw != "" {
		var err error
		id, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	entity, err := service.Database.GetEntity(id)
	if err != nil {
		return nil, fmt.Errorf("invalid entity %d: %w", id, err)
	}

	return context.WithValue(ctx, entityContextKey, entity), nil
}
