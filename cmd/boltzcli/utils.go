package main

import (
	"bufio"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"strconv"
	"strings"
)

func prompt(message string) bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(message + " [yes/no] ")

	input, err := reader.ReadString('\n')

	if err != nil {
		fmt.Println("Could not read input: " + err.Error())
		os.Exit(1)
	}

	switch strings.ToLower(strings.TrimSpace(input)) {
	case "yes":
		return true

	case "no":
		return false

	default:
		return prompt(message)
	}
}

func printJson(resp proto.Message) {
	jsonMarshaler := &protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: true,
	}

	fmt.Println(jsonMarshaler.Format(resp))
}

func parseInt64(value string, name string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		fmt.Println("Could not parse " + name + ": " + err.Error())
		os.Exit(1)
	}

	return parsed
}

func formatPercentageFee(percentageFee float32) string {
	return strconv.FormatFloat(float64(percentageFee), 'f', 1, 32)
}
