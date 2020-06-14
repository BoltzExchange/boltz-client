package boltz_lnd

import (
	litecoinCfg "github.com/ltcsuite/ltcd/chaincfg"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestApplyLitecoinParams(t *testing.T) {
	params := ApplyLitecoinParams(litecoinCfg.RegressionNetParams)

	expectedRef := reflect.ValueOf(litecoinCfg.RegressionNetParams)
	paramsRef := reflect.ValueOf(*params)

	comparedValues := 0

	for i := 0; i < paramsRef.NumField(); i++ {
		entry := paramsRef.Field(i)

		switch entry.Kind() {
		case reflect.String:
			comparedValues += 1
			assert.Equal(t, expectedRef.Field(i).String(), entry.String())

		case reflect.Uint8, reflect.Uint16, reflect.Uint32:
			if entry.Uint() != 0 {
				comparedValues += 1
				assert.Equal(t, expectedRef.Field(i).Uint(), entry.Uint())
			}

		case reflect.Int32, reflect.Int64:
			if entry.Int() != 0 {
				comparedValues += 1
				assert.Equal(t, expectedRef.Field(i).Int(), entry.Int())
			}

		case reflect.Array:
			if paramsRef.Type().Field(i).Name != "Deployments" {
				comparedValues += 1
				assert.Equal(t, expectedRef.Field(i).Interface(), entry.Interface())
			}
		}
	}

	assert.Equal(t, 10, comparedValues)
}
