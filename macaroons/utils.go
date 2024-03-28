package macaroons

import (
	"context"
	"crypto/rand"
	"errors"
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

func EntityFromContext(ctx context.Context) *int64 {
	entity, ok := ctx.Value(entityContextKey).(int64)
	if ok {
		return &entity
	}
	return nil
}

func addEntityToContext(ctx context.Context, value int64) context.Context {
	return context.WithValue(ctx, entityContextKey, value)
}
