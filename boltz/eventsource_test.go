package boltz

import (
	"github.com/r3labs/sse"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"strconv"
	"testing"
)

const streamName = "test"

var server *sse.Server

func spawnServer(portChan chan int) {
	server = sse.New()
	server.CreateStream(streamName)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.HTTPHandler)

	listener, err := net.Listen("tcp", ":0")

	if err != nil {
		panic(err)
	}

	portChan <- listener.Addr().(*net.TCPAddr).Port

	err = http.Serve(listener, mux)

	if err != nil {
		panic(err)
	}
}

func TestStreamSwapStatus(t *testing.T) {
	portChan := make(chan int)
	go spawnServer(portChan)
	port := <-portChan

	events := make(chan *SwapStatusResponse)
	stopListening := make(chan bool)

	streamTerminated := make(chan bool)
	var expectedError string

	go func() {
		err := streamSwapStatus("http://127.0.0.1:"+strconv.Itoa(port)+"?stream="+streamName, events, stopListening)

		assert.NotNil(t, err)
		assert.Equal(t, expectedError, err.Error())

		streamTerminated <- true
	}()

	// Should handle and parse events
	publishedStatus := "asdf123"
	server.Publish(streamName, &sse.Event{
		Data: []byte("{\"status\":\"" + publishedStatus + "\"}"),
	})

	parsedEvent := <-events

	assert.Equal(t, publishedStatus, parsedEvent.Status)

	// Should handle errors and terminate
	expectedError = "unexpected end of JSON input"

	server.Publish(streamName, &sse.Event{
		Data: []byte("{"),
	})

	<-streamTerminated

	// Should terminate when data is sent to "stopListening"
	go func() {
		err := streamSwapStatus("http://127.0.0.1:"+strconv.Itoa(port)+"?stream="+streamName, events, stopListening)

		assert.Nil(t, err)
		streamTerminated <- true
	}()

	stopListening <- true

	<-streamTerminated
}
