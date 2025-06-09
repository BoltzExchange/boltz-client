package lightning

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
)

func TestGetFeeLimit(t *testing.T) {
	routingFeePpm := uint64(1000) // 0.1%

	bigInvoice := "lnbcrt242314120n1p07xy5wpp5th2xv0vdmcx9ure5gs5zcs3vj2y37vg6a35dnl4te79nyq08drdsdqqcqzpgsp5zpwtknhqrdh5rz6lnzst52zt0wj88rjjhx49gxycx7m6z4qgv9ms9qy9qsq520tkslgzqhgsetygx8mc8se928l9favv4jdsmajmeds8ckzaxfrky55sazwx8gpfhx33ys9hg9mpj2vrx8wpe3jmsh8pvwayx2kpkcqm69z2z"
	bigInvoiceAmt := 24231412

	smallInvoice := "lnbcrt10n1p07xy0spp585tu2049ghzs6se80zryvskkrtp94cec87qf90xp068unsy0j0tsdqqcqzpgsp5k4dx8025w6wtkpz4tm2py675n5e0ajlhgchw6edgs8lpf9m435ks9qy9qsquycyql7ucqmdgzk75uctw87jq6cpszexadp9clekk7cna27vjz7nx4pwy86nvw28eppkwlk8kavcy2rx02kl23g6yemfqff80den62cphujfge"

	cfg := &chaincfg.RegressionNetParams

	// Should use the payment fee ratio for big invoices
	bigPaymentFeeLimit, err := CalculateFeeLimit(bigInvoice, cfg, routingFeePpm)

	assert.Nil(t, err)
	assert.Equal(t, uint(float64(bigInvoiceAmt)*float64(routingFeePpm)/1000000), bigPaymentFeeLimit)

	// Should use minimal payment fee for small invoices
	smallPaymentFeeLimit, err := CalculateFeeLimit(smallInvoice, cfg, routingFeePpm)

	assert.Nil(t, err)
	assert.Equal(t, uint(minPaymentFee), smallPaymentFeeLimit)

	// Should return fee limit 0 for invalid invoices
	zeroFeeLimit, err := CalculateFeeLimit("", cfg, routingFeePpm)

	assert.NotNil(t, err)
	assert.Equal(t, uint(0), zeroFeeLimit)
}
