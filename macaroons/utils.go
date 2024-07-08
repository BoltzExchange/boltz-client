package macaroons

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/BoltzExchange/boltz-client/database"
	"io"
)

type contextKey string

var rootKeyIDContextKey contextKey = "rootkeyid"

const entityContextKey contextKey = "entity"

func generateNewRootKey() ([]byte, error) {
	rootKey := make([]byte, rootKeyLen)
	if _, err := io.ReadFull(rand.Reader, rootKey); err != nil {
		return nil, err
	}

	return rootKey, nil
}

func rootKeyIDFromContext(ctx context.Context) ([]byte, error) {
	id, ok := ctx.Value(rootKeyIDContextKey).([]byte)

	if !ok {
		return nil, errors.New("could not read root key ID from context")
	}

	if len(id) == 0 {
		return nil, errors.New("root key ID is missing from the context")
	}

	return id, nil
}

func addRootKeyIdToContext(ctx context.Context, value interface{}) context.Context {
	return context.WithValue(ctx, rootKeyIDContextKey, value)
}

func EntityIdFromContext(ctx context.Context) *database.Id {
	if entity := ctx.Value(entityContextKey); entity != nil {
		return &EntityFromContext(ctx).Id
	}
	return nil
}

func EntityFromContext(ctx context.Context) *database.Entity {
	if value := ctx.Value(entityContextKey); value != nil {
		return value.(*database.Entity)
	}
	return nil
}

func AddEntityToContext(ctx context.Context, entity *database.Entity) context.Context {
	if entity == nil {
		return ctx
	}
	return context.WithValue(ctx, entityContextKey, entity)
}
