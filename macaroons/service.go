package macaroons

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/database"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon-bakery.v2/bakery/checkers"
	"gopkg.in/macaroon.v2"
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

	checker := checkers.New(nil)
	// checker.Register(entityContextKey.Name, checkers.StdNamespace, func(ctx context.Context, cond, arg string) error {
	// 	if cond == checkers.CondDeclared {
	// 		split := strings.Split(arg, " ")
	// 		if split[0] == entityContextKey.Name {
	// 			fmt.Println("Adding entity to context", split[1])
	// 			ctx = addEntityToContext(ctx, split[1])
	// 		}
	// 	}
	// 	return nil
	// })

	macaroonParams := bakery.BakeryParams{
		Location:     "boltz",
		RootKeyStore: &rootKeyStorage,
		Checker:      bakery.NewChecker(bakery.CheckerParams{Checker: checker}),
	}

	service.bakery = bakery.New(macaroonParams)
}

func (service *Service) NewMacaroon(entity *int64, ops ...bakery.Op) (*bakery.Macaroon, error) {
	ctx := addRootKeyIdToContext(context.Background(), defaultRootKeyID)

	var caveats []checkers.Caveat
	if entity != nil {
		caveats = append(caveats, checkers.DeclaredCaveat(string(entityContextKey), fmt.Sprint(*entity)))
	}

	return service.bakery.Oven.NewMacaroon(ctx, bakery.LatestVersion, caveats, ops...)
}

func (service *Service) ValidateMacaroon(ctx context.Context, macBytes []byte, requiredPermissions []bakery.Op) (context.Context, error) {
	mac := &macaroon.Macaroon{}
	err := mac.UnmarshalBinary(macBytes)

	if err != nil {
		return nil, err
	}

	authChecker := service.bakery.Checker.Auth(macaroon.Slice{mac})
	info, err := authChecker.Allow(context.Background(), requiredPermissions...)
	if err != nil {
		return nil, err
	}

	for _, caveat := range info.Conditions() {
		cond, arg, err := checkers.ParseCaveat(caveat)
		if err != nil {
			return nil, err
		}
		if cond == checkers.CondDeclared {
			split := strings.Split(arg, " ")
			if split[0] == string(entityContextKey) {
				if split[1] != "" {
					entity, err := strconv.ParseInt(split[1], 10, 64)
					if err != nil {
						return nil, err
					}
					ctx = addEntityToContext(ctx, entity)
				}
			}
		}
	}

	return ctx, err
}
