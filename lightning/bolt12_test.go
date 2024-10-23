package lightning

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckInvoiceIsForOffer(t *testing.T) {
	type args struct {
		invoice string
		offer   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{

			"Valid",
			args{
				invoice: "lni1qqgxy08evcguecrs38c7z0s5ptj7xq3qqc3xu3s3rg94nj40zfsy866mhu5vxne6tcej5878k2mneuvgjy8s5qqkyypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmjsyqrzymjxzydqkkw24ufxqslttwlj3s608f0rx2slc7etw0833zgs75syqh67zqzcyypyh6pttjn5axdynlppvpkvgdfwc76g8p58zvhqtf0jl2rc7hcayf9qnqpqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmsrsxwsc6gcgpj4j7w8d5ag9cgu50ewtywpt5ht8n45mpsxzfnu5kqszqnm77ygz0d5rrg004dh0w8cmlajs3npev2zmvuq96688kxe9ex07qqr9nrhgtm7626jel28j8rtwvtuyf3qnsdx5rar05tmp2sjj4aqkqcyn9gu240rgrmaareuhhfamjvvme9x0gsuqqqqqqqqqqqqqqq2qqqqqqqqqqqqq8fykt06c5sqqqqqpfqyvuvtxnagyzrw8es9ssvkykxftlhfx873fyezzad3reqqamr7yqj5gvtjfggfr2syqh67zq9syypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmhsgqtlq2fc932jthm4x4ja9wytxd83lnxzhxa7wgkjfklycvc3da86j8sxhte0w6fxgkfhm5daf6lv0jm93jwzdj69r5h54x7hv0hgu6tg",
				offer:   "lno1qgsqvgnwgcg35z6ee2h3yczraddm72xrfua9uve2rlrm9deu7xyfzrc2qqtzzqs0mht2rn9pzxwrqv8fs7qac9nacc3al5cd334e2wn0jztx0syfac",
			},
			true,
		},
		{

			"Valid",
			args{
				invoice: "lni1qqgxy08evcguecrs38c7z0s5ptj7xq3qqc3xu3s3rg94nj40zfsy866mhu5vxne6tcej5878k2mneuvgjy8s5qqkyypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmjsyqrzymjxzydqkkw24ufxqslttwlj3s608f0rx2slc7etw0833zgs75syqh67zqzcyypyh6pttjn5axdynlppvpkvgdfwc76g8p58zvhqtf0jl2rc7hcayf9qnqpqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmsrsxwsc6gcgpj4j7w8d5ag9cgu50ewtywpt5ht8n45mpsxzfnu5kqszqnm77ygz0d5rrg004dh0w8cmlajs3npev2zmvuq96688kxe9ex07qqr9nrhgtm7626jel28j8rtwvtuyf3qnsdx5rar05tmp2sjj4aqkqcyn9gu240rgrmaareuhhfamjvvme9x0gsuqqqqqqqqqqqqqqq2qqqqqqqqqqqqq8fykt06c5sqqqqqpfqyvuvtxnagyzrw8es9ssvkykxftlhfx873fyezzad3reqqamr7yqj5gvtjfggfr2syqh67zq9syypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmhsgqtlq2fc932jthm4x4ja9wytxd83lnxzhxa7wgkjfklycvc3da86j8sxhte0w6fxgkfhm5daf6lv0jm93jwzdj69r5h54x7hv0hgu6tg",
				offer:   "lno1qgsqvgnwgcg35z6ee2h3yczraddm72xrfua9uve2rlrm9deu7xyfzrcgqgn3qzsyw3jhxaqkyypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnms",
			},
			true,
		},
		{
			"Invalid",
			args{
				invoice: "lni1qqgxy08evcguecrs38c7z0s5ptj7xq3qqc3xu3s3rg94nj40zfsy866mhu5vxne6tcej5878k2mneuvgjy8s5qqkyypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmjsyqrzymjxzydqkkw24ufxqslttwlj3s608f0rx2slc7etw0833zgs75syqh67zqzcyypyh6pttjn5axdynlppvpkvgdfwc76g8p58zvhqtf0jl2rc7hcayf9qnqpqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmsrsxwsc6gcgpj4j7w8d5ag9cgu50ewtywpt5ht8n45mpsxzfnu5kqszqnm77ygz0d5rrg004dh0w8cmlajs3npev2zmvuq96688kxe9ex07qqr9nrhgtm7626jel28j8rtwvtuyf3qnsdx5rar05tmp2sjj4aqkqcyn9gu240rgrmaareuhhfamjvvme9x0gsuqqqqqqqqqqqqqqq2qqqqqqqqqqqqq8fykt06c5sqqqqqpfqyvuvtxnagyzrw8es9ssvkykxftlhfx873fyezzad3reqqamr7yqj5gvtjfggfr2syqh67zq9syypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnmhsgqtlq2fc932jthm4x4ja9wytxd83lnxzhxa7wgkjfklycvc3da86j8sxhte0w6fxgkfhm5daf6lv0jm93jwzdj69r5h54x7hv0hgu6tg",
				offer:   "lno1qgsqvgnwgcg35z6ee2h3yczraddm72xrfua9uve2rlrm9deu7xyfzrcgqgn3qzsyw3jhxaqkyypqlhwk58x2zyvuxqcwnpupmst8m33rmlfsmrrtj5axlyykvlqgnms",
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, CheckInvoiceIsForOffer(tt.args.invoice, tt.args.offer), "CheckInvoiceIsForOffer(%v, %v)", tt.args.invoice, tt.args.offer)
		})
	}
}
