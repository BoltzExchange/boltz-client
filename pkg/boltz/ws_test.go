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
	require.Nil(t, ws.conn)
	require.Equal(t, disconnected, ws.state)
	return ws
}

func TestWebsocketLazy(t *testing.T) {
	ws := setupWs(t)

	firstId := "swapId"
	secondId := "anotherSwapId"
	err := ws.Subscribe([]string{firstId, secondId})
	require.NoError(t, err)

	require.Equal(t, []string{firstId, secondId}, ws.swapIds)
	require.Equal(t, connected, ws.state)
	require.NotNil(t, ws.conn)

	ws.Unsubscribe(firstId)
	require.Equal(t, connected, ws.state)
	require.Equal(t, []string{secondId}, ws.swapIds)
	ws.Unsubscribe(secondId)

	require.Equal(t, disconnected, ws.state)
	require.Nil(t, ws.conn)
}

func TestWebsocketReconnect(t *testing.T) {
	setup := func(t *testing.T) *Websocket {
		ws := setupWs(t)
		require.NoError(t, ws.Connect())
		require.NotNil(t, ws.conn)
		require.Equal(t, connected, ws.state)
		return ws
	}

	t.Run("Automatic", func(t *testing.T) {
		ws := setup(t)
		ws.reconnectInterval = 50 * time.Millisecond
		firstConn := ws.conn
		require.NoError(t, ws.conn.Close())

		waitFor := time.Second
		require.Eventually(t, func() bool {
			return ws.state == reconnecting
		}, waitFor, ws.reconnectInterval/2)

		require.Eventually(t, func() bool {
			return ws.state == connected
		}, waitFor, ws.reconnectInterval)

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
		require.Equal(t, connected, ws.state)
	})

}

func TestWebsocketShutdown(t *testing.T) {
	ws := setupWs(t)
	require.NoError(t, ws.Connect())
	require.NotNil(t, ws.conn)
	require.Equal(t, connected, ws.state)

	require.NoError(t, ws.Close())
	require.Eventually(t, func() bool {
		return ws.state == closed
	}, time.Second, 10*time.Millisecond)
	require.Nil(t, ws.conn)

	require.Error(t, ws.Subscribe([]string{"swapId"}))
}
