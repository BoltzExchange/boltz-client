package lightning

import (
	"testing"
)

func TestParseOffer(t *testing.T) {
	tests := []struct {
		name    string
		offer   string
		want    *Offer
		wantErr bool
	}{
		{
			"Invalid",
			"lnoalsödkfjasödf",
			nil,
			true,
		},
		{
			"Valid",
			"lno1pqpzwyq2p32x2um5ypmx2cm5dae8x93pqthvwfzadd7jejes8q9lhc4rvjxd022zv5l44g6qah82ru5rdpnpj",
			&Offer{
				MinAmount: 10,
			},
			false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeOffer(tt.offer)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeOffer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DecodeOffer() got = %v, want %v", got, tt.want)
			}
		})
	}
}
