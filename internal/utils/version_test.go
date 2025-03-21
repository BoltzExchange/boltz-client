package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {

	tt := []struct {
		name    string
		version string
		min     string
		valid   bool
	}{
		{"cln", "v23.11rc3", "0.23.0", true},
		{"cln", "v23.11-modded", "0.23.0", true},
		{"cln", "basedon-v24.08.2", "0.23.0", true},
		{"lnd", "0.15.0-beta commit=234234", "0.15.0", true},
		{"invalid", "invalid", "0.15.0", false},
		{"invalid-min", "0.15.0", "invalid", false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckVersion("test", tc.version, tc.min)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}

}
