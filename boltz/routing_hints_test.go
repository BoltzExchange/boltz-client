package boltz

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFindMagicRoutingHint(t *testing.T) {
	tests := []struct {
		name    string
		invoice string
		find    bool
	}{
		{
			"WithMagicHint",
			"lnbcrt1m1pnrqts6pp5f545jvan9s4qr92h8vm8a99hc9c6p4rlkk5tj55umwyww9jpqjjsdpz2djkuepqw3hjqnpdgf2yxgrpv3j8yetnwvcqzpxxqrgegrzjqdrdrehshza87d0kx8fzrvy9m3vy2lfdmayr36qfemafgl4ztqlcjzzxeyqq28qqqqqqqqqqqqqqq9gq2ysp5gjmrl98w88dyj0dsc9yyezqx9s7jamuwkaf8dgv9mvva54qsvxls9qyyssqz58huhfekkedzxa05405sh99edfmvu4g9a68jljy9appnsk9mkdq8ck5gnzn3rtfzwpn466rqc8cccplegy7chrszn75ud6w5wtdx8qqt3au9t",
			true,
		},
		{
			"WithoutHint",
			"lnbcrt1m1pnrdvytpp5pd852whdy0v7zq80r57x4vuke42606k59menkv54lq8w2gkuplnqdqqcqzzsxqyz5vqsp5v93ulsu4q9r59jgz699tfq7q7xasrdhveamplf0qd3z23atqlcjq9qyyssqmgwrkwu92jpqdf56a7qlxxqts93x7qnw0qc5nmsv0f6fp2uqfktqsfrzd5vcwgsmm3rrjyf6uums66kput09c5wwfudt4ngqky24swcq0ratfe",
			false,
		},
		{
			"WithNormalHint",
			"lnbcrt1pnr3275pp5p223gss6yr93s3shv56fvdk8n54399j7cnje0cv54p0vncvtjyeqdq8w3jhxaqrzjqgqvancpl6sk2ylx2r0mtek0qng60yqlp566ctj3xasck8q2akrj5qqqqqqqqqqq0vqqqqqqqqqqqqqqqqnp4qgqvancpl6sk2ylx2r0mtek0qng60yqlp566ctj3xasck8q2akrj57n3wemqzhn4f08h7xekft4ty0llyvd33z2vrernpqqv83csxky3p9s3ynzxtpq558cgvw5v8zljhr3kqnr3u9kjns3mgj6r30l5t56qq7ulqxr",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice, err := zpay32.Decode(tt.invoice, &chaincfg.RegressionNetParams)
			require.NoError(t, err)
			hint := FindMagicRoutingHint(invoice)
			if tt.find {
				require.NotNil(t, hint)
			} else {
				require.Nil(t, hint)
			}
		})
	}
}
