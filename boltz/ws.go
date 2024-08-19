package boltz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/BoltzExchange/boltz-client/logger"
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
	Updates chan SwapUpdate

	apiUrl        string
	subscriptions chan bool
	conn          *websocket.Conn
	closed        bool
	dialer        *websocket.Dialer
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
		apiUrl:        boltz.URL,
		subscriptions: make(chan bool),
		dialer:        &dialer,
		Updates:       make(chan SwapUpdate),
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
	boltz.conn = conn
	if err != nil {
		return fmt.Errorf("could not connect to boltz ws at %s: %w", wsUrl, err)
	}

	logger.Infof("Connected to Boltz ws at %s", wsUrl)

	setDeadline := func() error {
		return conn.SetReadDeadline(time.Now().Add(pingInterval + pongWait))
	}
	_ = setDeadline()
	conn.SetPongHandler(func(string) error {
		logger.Silly("Received pong")
		return setDeadline()
	})

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for range ticker.C {
			// Will not wait longer with writing than for the response
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pongWait))
			if err != nil {
				if boltz.closed {
					return
				}
				logger.Errorf("could not send ping; closing connection: %s", err)
				_ = boltz.Close()
			}
		}
	}()

	go func() {
		for {
			msgType, message, err := conn.ReadMessage()
			if err != nil {
				if boltz.closed {
					close(boltz.Updates)
					return
				}
				logger.Error("could not receive message: " + err.Error())
				break
			}

			logger.Silly("Received websocket message: " + string(message))

			switch msgType {
			case websocket.TextMessage:
				var response wsResponse
				if err := json.Unmarshal(message, &response); err != nil {
					logger.Errorf("could not parse websocket response: %s", err)
					continue
				}
				if response.Error != "" {
					logger.Errorf("boltz websocket error: %s", response.Error)
					continue
				}

				switch response.Event {
				case "update":
					switch response.Channel {
					case "swap.update":
						for _, arg := range response.Args {
							var update SwapUpdate
							if err := mapstructure.Decode(arg, &update); err != nil {
								logger.Errorf("invalid boltz response: %v", err)
							}
							boltz.Updates <- update
						}
					default:
						logger.Warnf("unknown update channel: %s", response.Channel)
					}
				case "subscribe":
					boltz.subscriptions <- true
					continue
				default:
					logger.Warnf("unknown event: %s", response.Event)
				}
			}
		}
		for {
			logger.Errorf("lost connection to boltz ws, reconnecting in %s", reconnectInterval)
			time.Sleep(reconnectInterval)
			err := boltz.Connect()
			if err == nil {
				return
			}
		}
	}()

	return nil
}

func (boltz *Websocket) Subscribe(swapIds []string) error {
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

func (boltz *Websocket) Close() error {
	boltz.closed = true
	return boltz.conn.Close()
}
