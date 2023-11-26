package lightning

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidChanId(t *testing.T) {
	tests := []struct {
		cln string
		lnd uint64
	}{
		{"166x1x0", 182518930276352},
		{"768154x1685x1", 844594255033073665},
		{"813118x897x0", 894032695812751360},
		{"732105x821x1", 804957960306753537},
	}

	for _, tc := range tests {
		chanIdCln, err := NewChanIdFromString(tc.cln)
		require.NoError(t, err)
		chanIdLnd, err := NewChanIdFromString(fmt.Sprint(tc.lnd))
		require.NoError(t, err)

		require.Equal(t, chanIdCln, chanIdLnd)
		require.Equal(t, tc.lnd, chanIdCln.ToLnd())
		require.Equal(t, tc.cln, chanIdCln.ToCln())
	}
}

func TestInvalidChanId(t *testing.T) {
	_, err := NewChanIdFromString("166x1x")
	require.Error(t, err)
	_, err = NewChanIdFromString("166x1x0x0")
	require.Error(t, err)
	_, err = NewChanIdFromString("abcdefg")
	require.Error(t, err)
}
