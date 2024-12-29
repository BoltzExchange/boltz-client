package macaroons

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateNewRootKey(t *testing.T) {
	newRootKey, err := generateNewRootKey()
	assert.Nil(t, err)
	assert.Len(t, newRootKey, rootKeyLen)

	anotherRootKey, err := generateNewRootKey()
	assert.Nil(t, err)

	// Very basic sanity check. But better than nothing
	assert.NotEqual(t, newRootKey, anotherRootKey, "new root keys are not random")
}

func TestRootKeyIDFromContext(t *testing.T) {
	// Should return error when no root key id is set
	rootKeyId, err := rootKeyIDFromContext(context.Background())
	assert.Nil(t, rootKeyId)
	assert.Equal(t, errors.New("could not read root key ID from context"), err)

	// Should return error when set root key id empty
	ctx := context.WithValue(context.Background(), rootKeyIDContextKey, []byte{})
	rootKeyId, err = rootKeyIDFromContext(ctx)
	assert.Nil(t, rootKeyId)
	assert.Equal(t, errors.New("root key ID is missing from the context"), err)

	// Should successfully get the root key id from the context if set correctly
	expectedRootKeyId := []byte{1}
	ctx = context.WithValue(context.Background(), rootKeyIDContextKey, expectedRootKeyId)
	rootKeyId, err = rootKeyIDFromContext(ctx)
	assert.Equal(t, expectedRootKeyId, rootKeyId)
	assert.Nil(t, err)
}

func TestAddRootKeyIdToContext(t *testing.T) {
	expectedValue := []byte{0, 1, 2, 3}
	ctx := addRootKeyIdToContext(context.Background(), expectedValue)

	assert.Equal(t, ctx.Value(rootKeyIDContextKey), expectedValue)
}
