package macaroons

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
)

type contextKey struct {
	Name string
}

var rootKeyIDContextKey = contextKey{"rootkeyid"}

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
