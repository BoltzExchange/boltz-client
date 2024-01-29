package boltz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAddress(t *testing.T) {
	err := ValidateAddress(MainNet, "3F8UixJcrfxCaGpRryyRuKotBFXRFeW7ej", CurrencyBtc)
	assert.NoError(t, err)

	err = ValidateAddress(MainNet, "02f1a8c87607f415c8f22c00593002775941dea48869ce23096af27b0cfdcc0b69", CurrencyBtc)
	assert.Error(t, err)
}
