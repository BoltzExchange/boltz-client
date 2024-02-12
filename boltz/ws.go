package boltz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/gorilla/websocket"
)

const reconnectInterval = 15

type SwapUpdate = SwapStatusResponse

type BoltzWebsocket struct {
	apiUrl  string
	Updates chan SwapUpdate

	subscriptions chan bool
	conn          *websocket.Conn
	closed        bool
}

type wsResponse struct {
	Event   string `json:"event"`
	Error   string `json:"error"`
	Channel string `json:"channel"`
	Args    []any  `json:"args"`
}

func NewBoltzWebsocket(apiUrl string) *BoltzWebsocket {
	ws := &BoltzWebsocket{
		apiUrl:        apiUrl,
		subscriptions: make(chan bool),
		Updates:       make(chan SwapUpdate),
	}

	return ws
}

func (boltz *BoltzWebsocket) SendJson(data any) error {
	send, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return boltz.conn.WriteMessage(websocket.TextMessage, send)
}

func (boltz *BoltzWebsocket) Connect() error {
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

	conn, _, err := websocket.DefaultDialer.Dial(wsUrl.String(), nil)
	boltz.conn = conn
	if err != nil {
		return fmt.Errorf("could not connect to boltz ws at %s: %w", wsUrl, err)
	}

	logger.Infof("Connected to boltz ws at %s", wsUrl)

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
			case websocket.PingMessage:
				if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
					logger.Errorf("could not send pong: %s", err)
				}
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
						var updates []SwapUpdate
						data, _ := json.Marshal(response.Args)
						if err := json.Unmarshal(data, &updates); err != nil {
							logger.Errorf("invalid boltz response: %v", err)
						}
						for _, update := range updates {
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
			logger.Errorf("lost connection to boltz ws, reconnecting in %d seconds", reconnectInterval)
			time.Sleep(reconnectInterval * time.Second)
			err := boltz.Connect()
			if err == nil {
				return
			}
		}
	}()

	return nil
}

func (boltz *BoltzWebsocket) Subscribe(swapIds []string) error {
	if len(swapIds) == 0 {
		return nil
	}
	if err := boltz.SendJson(map[string]any{
		"op":      "subscribe",
		"channel": "swap.update",
		"args":    swapIds,
	}); err != nil {
		return err
	}
	select {
	case <-boltz.subscriptions:
		return nil
	case <-time.After(time.Second):
		return errors.New("no answer from boltz")
	}
}

func (boltz *BoltzWebsocket) Close() error {
	boltz.closed = true
	return boltz.conn.Close()
}
