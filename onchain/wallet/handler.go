package wallet

import (
	"github.com/BoltzExchange/boltz-client/logger"
)

// #include "gdk.h"
import "C"

type Notification string

var (
	blockNotification Notification = "block"
)

type handlerFunc = func(map[string]any)

var handlers = make(map[string]handlerFunc)

// this has to be in a seperate file to avoid c redefition issues
//
//export go_notification_handler
func go_notification_handler(details Json) {
	var v map[string]any
	if err := parseJson(details, &v); err != nil {
		logger.Error("Could not parse notification details")
	}

	event, ok := v["event"].(string)
	if !ok {
		logger.Error("Could not parse notification event")
	}
	handler, ok := handlers[event]
	if ok {
		handler(v[event].(map[string]any))
	}
	logger.Debugf("Received gdk notification: %v", v)
}

func registerHandler(notification Notification, handler handlerFunc) {
	handlers[string(notification)] = handler
}

func removeHandler(notification Notification) {
	delete(handlers, string(notification))
}
