package macaroons

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"io"
)

type contextKey string

var rootKeyIDContextKey contextKey = "rootkeyid"

const tenantContextKey contextKey = "tenant"

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

func TenantIdFromContext(ctx context.Context) *database.Id {
	if tenant := ctx.Value(tenantContextKey); tenant != nil {
		return &TenantFromContext(ctx).Id
	}
	return nil
}

func TenantFromContext(ctx context.Context) *database.Tenant {
	if value := ctx.Value(tenantContextKey); value != nil {
		return value.(*database.Tenant)
	}
	return nil
}

func AddTenantToContext(ctx context.Context, tenant *database.Tenant) context.Context {
	if tenant == nil {
		return ctx
	}
	return context.WithValue(ctx, tenantContextKey, tenant)
}
