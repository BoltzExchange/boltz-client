//go:build !unit

package boltz

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWebsocketReconnect(t *testing.T) {
	logger.Init(logger.Options{Level: "debug"})
	api := Api{URL: "http://localhost:9001"}

	ws := api.NewWebsocket()
	err := ws.Connect()
	require.NoError(t, err)

	firstId := "swapId"
	err = ws.Subscribe([]string{firstId})
	require.NoError(t, err)

	firstConn := ws.conn
	err = firstConn.Close()
	require.NoError(t, err)

	anotherId := "anotherSwapId"
	err = ws.Subscribe([]string{anotherId})
	require.NoError(t, err)
	require.NotEqual(t, firstConn, ws.conn, "subscribe should reconnect forcefully")
	require.Equal(t, []string{firstId, anotherId}, ws.swapIds)
}
