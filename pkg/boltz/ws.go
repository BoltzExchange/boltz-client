package boltz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
)

const reconnectInterval = 15 * time.Second
const pingInterval = 30 * time.Second
const pongWait = 5 * time.Second

type SwapUpdate struct {
	SwapStatusResponse `mapstructure:",squash"`
	Id                 string `json:"id"`
}

type Websocket struct {
	Updates     chan SwapUpdate
	updatesLock sync.Mutex

	apiUrl            string
	subscriptions     chan bool
	conn              *websocket.Conn
	closed            bool
	dialer            *websocket.Dialer
	swapIds           []string
	reconnectInterval time.Duration
}

type wsResponse struct {
	Event   string `json:"event"`
	Error   string `json:"error"`
	Channel string `json:"channel"`
	Args    []any  `json:"args"`
}

func (boltz *Api) NewWebsocket() *Websocket {
	httpTransport, ok := boltz.Client.Transport.(*http.Transport)

	dialer := *websocket.DefaultDialer
	if ok {
		dialer.Proxy = httpTransport.Proxy
	}

	return &Websocket{
		apiUrl:            boltz.URL,
		subscriptions:     make(chan bool),
		dialer:            &dialer,
		Updates:           make(chan SwapUpdate),
		reconnectInterval: reconnectInterval,
	}
}

func (boltz *Websocket) Connect() error {
	if boltz.closed {
		return errors.New("websocket is closed")
	}
	wsUrl, err := url.Parse(boltz.apiUrl)
	if err != nil {
		return err
	}
	wsUrl.Path += "/v2/ws"

	if wsUrl.Scheme == "https" {
		wsUrl.Scheme = "wss"
	} else if wsUrl.Scheme == "http" {
		wsUrl.Scheme = "ws"
	}

	conn, _, err := boltz.dialer.Dial(wsUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("could not connect to boltz ws at %s: %w", wsUrl, err)
	}
	boltz.conn = conn

	logger.Infof("Connected to Boltz ws at %s", wsUrl)

	setDeadline := func() error {
		return conn.SetReadDeadline(time.Now().Add(pingInterval + pongWait))
	}
	_ = setDeadline()
	conn.SetPongHandler(func(string) error {
		logger.Silly("Received pong")
		return setDeadline()
	})
	pingTicker := time.NewTicker(pingInterval)

	go func() {
		defer pingTicker.Stop()
		for range pingTicker.C {
			// Will not wait longer with writing than for the response
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pongWait))
			if err != nil {
				if boltz.closed {
					return
				}
				logger.Errorf("could not send ping: %s", err)
				return
			}
		}
	}()

	go func() {
		for {
			msgType, message, err := conn.ReadMessage()
			if err != nil {
				// if `close` was intentionally called, `Connected` will return false
				// since the connection has already been set to nil.
				if boltz.Connected() {
					boltz.conn = nil
					logger.Error("could not receive message: " + err.Error())
				} else {
					return
				}
				break
			}

			logger.Silly("Received websocket message: " + string(message))

			switch msgType {
			case websocket.TextMessage:
				if err := boltz.handleTextMessage(message); err != nil {
					logger.Errorf("could not handle websocket message: %s", err)
				}
			default:
				logger.Warnf("unknown message type: %v", msgType)
			}
		}
		pingTicker.Stop()
		for {
			logger.Errorf("lost connection to boltz ws, reconnecting in %s", boltz.reconnectInterval)
			time.Sleep(boltz.reconnectInterval)
			err := boltz.Connect()
			if err == nil {
				return
			}
		}
	}()
	return nil
}

func (boltz *Websocket) handleTextMessage(data []byte) error {
	boltz.updatesLock.Lock()
	defer boltz.updatesLock.Unlock()
	var response wsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("invalid json: %s", err)
	}
	if response.Error != "" {
		return fmt.Errorf("boltz error: %s", response.Error)
	}

	switch response.Event {
	case "update":
		switch response.Channel {
		case "swap.update":
			for _, arg := range response.Args {
				var update SwapUpdate
				if err := mapstructure.Decode(arg, &update); err != nil {
					return fmt.Errorf("invalid boltz response: %v", err)
				}
				boltz.Updates <- update
			}
		default:
			logger.Warnf("unknown update channel: %s", response.Channel)
		}
	case "subscribe":
		boltz.subscriptions <- true
	default:
		logger.Warnf("unknown ws event: %s", response.Event)
	}
	return nil
}

func (boltz *Websocket) subscribe(swapIds []string) error {
	if boltz.closed {
		return errors.New("websocket is closed")
	}
	logger.Infof("Subscribing to Swaps: %v", swapIds)
	if len(swapIds) == 0 {
		return nil
	}
	if err := boltz.conn.WriteJSON(map[string]any{
		"op":      "subscribe",
		"channel": "swap.update",
		"args":    swapIds,
	}); err != nil {
		return err
	}
	select {
	case <-boltz.subscriptions:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("no answer from boltz")
	}
}

func (boltz *Websocket) Subscribe(swapIds []string) error {
	if len(swapIds) == 0 {
		return nil
	}
	if !boltz.Connected() {
		if err := boltz.Connect(); err != nil {
			return fmt.Errorf("could not connect boltz ws: %w", err)
		}
	}
	if err := boltz.subscribe(swapIds); err != nil {
		// the connection might be dead, so forcefully reconnect
		if err := boltz.Reconnect(); err != nil {
			return fmt.Errorf("could not reconnect boltz ws: %w", err)
		}
		if err := boltz.subscribe(swapIds); err != nil {
			return err
		}
	}
	boltz.swapIds = append(boltz.swapIds, swapIds...)
	return nil
}

func (boltz *Websocket) Unsubscribe(swapId string) {
	boltz.swapIds = slices.DeleteFunc(boltz.swapIds, func(id string) bool {
		return id == swapId
	})
	logger.Debugf("Unsubscribed from swap %s", swapId)
	if len(boltz.swapIds) == 0 {
		logger.Debugf("No more pending swaps, disconnecting websocket")
		if err := boltz.close(); err != nil {
			logger.Warnf("could not close boltz ws: %v", err)
		}
	}
}

func (boltz *Websocket) close() error {
	if conn := boltz.conn; conn != nil {
		boltz.conn = nil
		return conn.Close()
	}
	return nil
}

func (boltz *Websocket) Close() error {
	// setting this flag will cause the `Updates` channel to be closed
	// in the receiving goroutine. this isn't done here to avoid a situation
	// where we close the channel while the receiving routine is processing an incoming message
	// and then tries to send on the closed channel.
	boltz.updatesLock.Lock()
	defer boltz.updatesLock.Unlock()
	close(boltz.Updates)
	boltz.closed = true
	return boltz.close()
}

func (boltz *Websocket) Connected() bool {
	return boltz.conn != nil
}

func (boltz *Websocket) Reconnect() error {
	logger.Infof("Force reconnecting to Boltz ws")
	if err := boltz.close(); err != nil {
		logger.Warnf("could not close boltz ws: %v", err)
	}
	return boltz.Connect()
}
