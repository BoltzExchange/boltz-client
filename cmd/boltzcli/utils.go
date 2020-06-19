package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"os"
	"strconv"
)

func printJson(resp proto.Message) {
	encoder := json.NewEncoder(os.Stdout)
	// Needs to be set to false for the BIP21 string to be formatted correctly
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(resp)

	if err != nil {
		fmt.Println("Could not decode response: " + err.Error())
		return
	}
}

func parseInt64(value string, name string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		fmt.Println("Could not parse " + name + ": " + err.Error())
		os.Exit(1)
	}

	return parsed
}
