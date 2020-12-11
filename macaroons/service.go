package macaroons

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/BoltzExchange/boltz-lnd/database"
	"google.golang.org/grpc/metadata"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon.v2"
	"strconv"
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

func (service *Service) ValidateMacaroon(ctx context.Context, requiredPermissions []bakery.Op) error {
	md, foundMetadata := metadata.FromIncomingContext(ctx)

	if !foundMetadata {
		return errors.New("could not get metadata from context")
	}

	if len(md["macaroon"]) != 1 {
		return errors.New("expected 1 macaroon, got " + strconv.Itoa(len(md["macaroon"])))
	}

	macBytes, err := hex.DecodeString(md["macaroon"][0])

	if err != nil {
		return err
	}

	mac := &macaroon.Macaroon{}
	err = mac.UnmarshalBinary(macBytes)

	if err != nil {
		return err
	}

	authChecker := service.bakery.Checker.Auth(macaroon.Slice{mac})
	_, err = authChecker.Allow(ctx, requiredPermissions...)

	return err
}
