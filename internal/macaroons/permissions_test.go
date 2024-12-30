package macaroons

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"testing"
)

func TestAdminPermissions(t *testing.T) {
	admin := make([]bakery.Op, len(ReadPermissions)+len(WritePermissions))
	copy(admin[:len(WritePermissions)], ReadPermissions)
	copy(admin[len(ReadPermissions):], WritePermissions)

	assert.Equal(t, admin, AdminPermissions(), "admin permissions are not copied correctly")
}
