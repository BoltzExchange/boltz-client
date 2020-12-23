package lnd

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetFeeLimit(t *testing.T) {
	bigInvoice := "lnbcrt242314120n1p07xy5wpp5th2xv0vdmcx9ure5gs5zcs3vj2y37vg6a35dnl4te79nyq08drdsdqqcqzpgsp5zpwtknhqrdh5rz6lnzst52zt0wj88rjjhx49gxycx7m6z4qgv9ms9qy9qsq520tkslgzqhgsetygx8mc8se928l9favv4jdsmajmeds8ckzaxfrky55sazwx8gpfhx33ys9hg9mpj2vrx8wpe3jmsh8pvwayx2kpkcqm69z2z"
	bigInvoiceAmt := 24231412

	smallInvoice := "lnbcrt10n1p07xy0spp585tu2049ghzs6se80zryvskkrtp94cec87qf90xp068unsy0j0tsdqqcqzpgsp5k4dx8025w6wtkpz4tm2py675n5e0ajlhgchw6edgs8lpf9m435ks9qy9qsquycyql7ucqmdgzk75uctw87jq6cpszexadp9clekk7cna27vjz7nx4pwy86nvw28eppkwlk8kavcy2rx02kl23g6yemfqff80den62cphujfge"

	lnd := LND{
		ChainParams: &chaincfg.RegressionNetParams,
	}

	// Should use the payment fee ratio for big invoices
	bigPaymentFeeLimit, err := lnd.getFeeLimit(bigInvoice)

	assert.Nil(t, err)
	assert.Equal(t, int64(float64(bigInvoiceAmt)*maxPaymentFeeRatio), bigPaymentFeeLimit)

	// Should use minimal payment fee for small invoices
	smallPaymentFeeLimit, err := lnd.getFeeLimit(smallInvoice)

	assert.Nil(t, err)
	assert.Equal(t, int64(minPaymentFee), smallPaymentFeeLimit)

	// Should return fee limit 0 for invalid invoices
	zeroFeeLimit, err := lnd.getFeeLimit("")

	assert.NotNil(t, err)
	assert.Equal(t, int64(0), zeroFeeLimit)
}
