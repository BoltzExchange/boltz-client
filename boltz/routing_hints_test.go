package boltz

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFindMagicRoutingHint(t *testing.T) {
	rawInvoice := "lnbcrt1m1pnrqts6pp5f545jvan9s4qr92h8vm8a99hc9c6p4rlkk5tj55umwyww9jpqjjsdpz2djkuepqw3hjqnpdgf2yxgrpv3j8yetnwvcqzpxxqrgegrzjqdrdrehshza87d0kx8fzrvy9m3vy2lfdmayr36qfemafgl4ztqlcjzzxeyqq28qqqqqqqqqqqqqqq9gq2ysp5gjmrl98w88dyj0dsc9yyezqx9s7jamuwkaf8dgv9mvva54qsvxls9qyyssqz58huhfekkedzxa05405sh99edfmvu4g9a68jljy9appnsk9mkdq8ck5gnzn3rtfzwpn466rqc8cccplegy7chrszn75ud6w5wtdx8qqt3au9t"
	invoice, err := zpay32.Decode(rawInvoice, &chaincfg.RegressionNetParams)
	require.NoError(t, err)
	hint := FindMagicRoutingHint(invoice)
	require.NotNil(t, hint)
}
