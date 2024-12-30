package utils

import (
	"bytes"
	"encoding/json"
)

func FormatJson(resp interface{}) (string, error) {
	buf := new(bytes.Buffer)

	encoder := json.NewEncoder(buf)

	// Needs to be set to false for the BIP21 string to be formatted correctly
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(resp)

	return buf.String(), err
}
