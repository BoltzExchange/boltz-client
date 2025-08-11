package boltz

import "testing"

func u64p(v uint64) *uint64   { return &v }
func f64p(v float64) *float64 { return &v }

func TestFeeMethodsAndValidity(t *testing.T) {
	tests := []struct {
		name        string
		fee         Fee
		wantHasSats bool
		wantHasSPV  bool
		wantIsValid bool
	}{
		{
			name:        "only_sats_nonzero",
			fee:         Fee{Sats: u64p(123), SatsPerVbyte: nil},
			wantHasSats: true, wantHasSPV: false, wantIsValid: true,
		},
		{
			name:        "only_sats_zero",
			fee:         Fee{Sats: u64p(0), SatsPerVbyte: nil},
			wantHasSats: true, wantHasSPV: false, wantIsValid: true,
		},
		{
			name:        "only_spv_nonzero",
			fee:         Fee{Sats: nil, SatsPerVbyte: f64p(2.5)},
			wantHasSats: false, wantHasSPV: true, wantIsValid: true,
		},
		{
			name:        "only_spv_zero",
			fee:         Fee{Sats: nil, SatsPerVbyte: f64p(0)},
			wantHasSats: false, wantHasSPV: true, wantIsValid: true,
		},
		{
			name:        "both_nil",
			fee:         Fee{Sats: nil, SatsPerVbyte: nil},
			wantHasSats: false, wantHasSPV: false, wantIsValid: false,
		},
		{
			name:        "both_set",
			fee:         Fee{Sats: u64p(1), SatsPerVbyte: f64p(1)},
			wantHasSats: true, wantHasSPV: true, wantIsValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fee.HasSats(); got != tc.wantHasSats {
				t.Fatalf("HasSats() = %v, want %v", got, tc.wantHasSats)
			}
			if got := tc.fee.HasSatsPerVbyte(); got != tc.wantHasSPV {
				t.Fatalf("HasSatsPerVbyte() = %v, want %v", got, tc.wantHasSPV)
			}
			if got := tc.fee.IsValid(); got != tc.wantIsValid {
				t.Fatalf("IsValid() = %v, want %v", got, tc.wantIsValid)
			}
		})
	}
}
