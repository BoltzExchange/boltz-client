package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchUri(t *testing.T) {
	uris := []string{
		"02d96eadea3d780104449aca5c93461ce67c1564e2e1d73225fa67dd3b997a6018@45.86.229.190:9736",
		"02d96eadea3d780104449aca5c93461ce67c1564e2e1d73225fa67dd3b997a6018@2a10:1fc0:3::270:a9dc:9736",
		"02d96eadea3d780104449aca5c93461ce67c1564e2e1d73225fa67dd3b997a6018@oo5tkbbpgnqjopdjxepyfavx3yemtylgzul67s7zzzxfeeqpde6yr7yd.onion:9736",
	}
	require.Equal(t, uris[0], searchUri("ip4", uris))
	require.Equal(t, uris[1], searchUri("ip6", uris))
	require.Equal(t, uris[2], searchUri("tor", uris))

	uris = []string{""}
	require.Equal(t, "", searchUri("ip4", uris))

}
