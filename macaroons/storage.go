package macaroons

import (
	"context"
	"github.com/BoltzExchange/boltz-lnd/database"
)

// TODO: encryption

var rootKeyLen = 32

type RootKeyStorage struct {
	database *database.Database
}

func (storage *RootKeyStorage) Get(_ context.Context, id []byte) ([]byte, error) {
	macaroon, err := storage.database.QueryMacaroon(id)

	if err != nil {
		return nil, err
	}

	return macaroon.RootKey, nil
}

func (storage *RootKeyStorage) RootKey(ctx context.Context) ([]byte, []byte, error) {
	id, err := rootKeyIDFromContext(ctx)

	if err != nil {
		return nil, nil, err
	}

	// Check if there is a macaroon for that ID already
	macaroon, err := storage.database.QueryMacaroon(id)

	// Create a new macaroon and save it to the database if not
	if err != nil {
		newRootKey, err := generateNewRootKey()

		if err != nil {
			return nil, nil, err
		}

		macaroon = &database.Macaroon{
			Id:      id,
			RootKey: newRootKey,
		}

		err = storage.database.CreateMacaroon(*macaroon)

		if err != nil {
			return nil, nil, err
		}
	}

	return macaroon.RootKey, macaroon.Id, nil
}
