package wallet

import (
	"github.com/BoltzExchange/boltz-client/logger"
	"sync"
)

// #include "gdk.h"
import "C"

type Notification string

var (
	subaccountNotification  Notification = "subaccount"
	transactionNotification Notification = "transaction"
)

type handlerFunc = func(map[string]any)

var handlers = make(map[Notification]handlerFunc)
var handlerLock = sync.Mutex{}

// this has to be in a separate file to avoid c redefinition issues
//
//export go_notification_handler
func go_notification_handler(details Json) {
	var v map[string]any
	if err := parseJson(details, &v); err != nil {
		logger.Error("Could not parse notification details")
	}
	logger.Debugf("Received gdk notification: %v", v)

	event, ok := v["event"].(string)
	if !ok {
		logger.Error("Could not parse notification event")
	}
	handler, ok := handlers[Notification(event)]
	if ok {
		handler(v[event].(map[string]any))
	}
}

func registerHandler(notification Notification, handler handlerFunc) {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	handlers[notification] = handler
}

func removeHandler(notification Notification) {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	delete(handlers, notification)
}
