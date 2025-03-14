//go:build !unit

package boltz

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func setupWs(t *testing.T) *Websocket {
	logger.Init(logger.Options{Level: "debug"})
	api := Api{URL: "http://localhost:9001"}
	ws := api.NewWebsocket()
	require.False(t, ws.Connected())
	return ws
}

func TestWebsocketLazy(t *testing.T) {
	ws := setupWs(t)

	firstId := "swapId"
	secondId := "anotherSwapId"
	err := ws.Subscribe([]string{firstId, secondId})
	require.NoError(t, err)

	require.Equal(t, []string{firstId, secondId}, ws.swapIds)
	require.True(t, ws.Connected())
	require.NotNil(t, ws.conn)

	ws.Unsubscribe(firstId)
	require.True(t, ws.Connected())
	require.Equal(t, []string{secondId}, ws.swapIds)
	ws.Unsubscribe(secondId)

	require.False(t, ws.Connected())
	require.Nil(t, ws.conn)
}

func TestWebsocketReconnect(t *testing.T) {
	setup := func(t *testing.T) *Websocket {
		ws := setupWs(t)
		require.NoError(t, ws.Connect())
		require.True(t, ws.Connected())
		return ws
	}

	t.Run("Automatic", func(t *testing.T) {
		ws := setup(t)
		ws.reconnectInterval = 50 * time.Millisecond
		firstConn := ws.conn
		require.NoError(t, ws.conn.Close())

		waitFor := time.Second

		require.Eventually(t, func() bool {
			return !ws.Connected()
		}, waitFor, ws.reconnectInterval/2)

		require.Eventually(t, func() bool {
			return ws.Connected()
		}, waitFor, ws.reconnectInterval/2)

		newConn := ws.conn
		require.NotNil(t, newConn)
		require.NotEqual(t, firstConn, newConn)
	})

	t.Run("Force", func(t *testing.T) {
		ws := setup(t)
		firstConn := ws.conn

		err := ws.Subscribe([]string{"swapId"})
		require.NoError(t, err)

		require.NoError(t, ws.conn.Close())

		err = ws.Subscribe([]string{"anotherSwapId"})
		require.NoError(t, err)
		require.NotEqual(t, firstConn, ws.conn, "subscribe should reconnect forcefully")
		require.True(t, ws.Connected())
	})

}

func TestWebsocketShutdown(t *testing.T) {
	ws := setupWs(t)
	require.NoError(t, ws.Connect())
	require.True(t, ws.Connected())

	require.NoError(t, ws.Close())
	require.True(t, ws.closed)
	require.False(t, ws.Connected())
	require.Eventually(t, func() bool {
		_, ok := <-ws.Updates
		return !ok
	}, time.Second, 10*time.Millisecond)

	require.Error(t, ws.Subscribe([]string{"swapId"}))
}
