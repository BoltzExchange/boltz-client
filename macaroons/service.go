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

func (service *Service) NewMacaroon(tenant *database.Id, ops ...bakery.Op) (*bakery.Macaroon, error) {
	ctx := addRootKeyIdToContext(context.Background(), defaultRootKeyID)

	var caveats []checkers.Caveat
	if tenant != nil {
		caveats = append(caveats, checkers.DeclaredCaveat(string(tenantContextKey), fmt.Sprint(*tenant)))
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

func (service *Service) validateTenant(ctx context.Context, raw string) (context.Context, error) {
	if raw != "" {
		if raw == "all" {
			return ctx, nil
		}
		var tenant *database.Tenant
		id, err := strconv.ParseUint(raw, 10, 64)
		if err == nil {
			tenant, err = service.Database.GetTenant(id)
		} else {
			tenant, err = service.Database.GetTenantByName(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid tenant %s: %w", raw, err)
		}
		return AddTenantToContext(ctx, tenant), nil
	}
	return AddTenantToContext(ctx, &database.DefaultTenant), nil
}
