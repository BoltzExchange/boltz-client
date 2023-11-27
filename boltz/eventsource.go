package boltz

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
)

var responseDelimiter = []byte{':', ' '}

func streamSwapStatus(url string, events chan *SwapStatusResponse, stopListening <-chan bool) error {
	handleError := make(chan error)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")

	var res *http.Response
	closed := false
	go func() {
		res, err = client.Do(req)

		if err != nil {
			handleError <- err
			return
		}

		reader := bufio.NewReader(res.Body)

		var currentEvent *SwapStatusResponse

		for {
			line, err := reader.ReadBytes('\n')

			if err != nil {
				handleError <- err
				return
			}

			split := bytes.Split(line, responseDelimiter)

			currentEvent = &SwapStatusResponse{}

			if string(split[0]) == "data" && !closed {
				data := bytes.TrimSpace(split[1])
				err = json.Unmarshal(data, currentEvent)

				if err != nil {
					handleError <- err
					return
				}

				events <- currentEvent
			}
		}
	}()

	for {
		select {
		case err = <-handleError:
			return err

		case <-stopListening:
			if res != nil {
				_ = res.Body.Close()
			}
			closed = true

			return nil
		}
	}
}
