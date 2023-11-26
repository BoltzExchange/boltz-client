package main

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"strconv"
	"time"
)

func prompt(message string) bool {
	confirm := &survey.Confirm{
		Message: message,
	}

	var answer bool
	err := survey.AskOne(confirm, &answer)

	if err != nil {
		fmt.Println("Could not read input: " + err.Error())
		os.Exit(1)
	}
	return answer
}

func printJson(resp proto.Message) {
	jsonMarshaler := &protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: true,
	}

	fmt.Println(jsonMarshaler.Format(resp))
}

func liquidAccountType(accountType string) string {
	switch accountType {
	case "p2sh-p2wpkh":
		return "Legacy SegWit"
	case "p2wpkh":
		return "SegWit"
	}
	return accountType
}

func parseDate(timestamp int64) string {
	return time.Unix(timestamp, 0).Format(time.RFC3339)
}

func optionalInt[V constraints.Integer](value *V) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(int(*value))
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

func getPair(ctx *cli.Context) string {
	if ctx.Bool("liquid") {
		return "L-BTC/BTC"
	}
	return ctx.String("pair")
}

func requireNArgs(n int, action cli.ActionFunc) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		if ctx.NArg() < n {
			return cli.ShowSubcommandHelp(ctx)
		}
		return action(ctx)
	}
}
