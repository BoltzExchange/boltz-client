package main

import (
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/utils"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

func parseUint64(value string, name string) uint64 {
	parsed, err := strconv.ParseUint(value, 10, 64)

	if err != nil {
		fmt.Println("Could not parse " + name + ": " + err.Error())
		os.Exit(1)
	}

	return parsed
}

func requireNArgs(n int, action cli.ActionFunc) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		if ctx.NArg() < n {
			return cli.ShowSubcommandHelp(ctx)
		}
		return action(ctx)
	}
}

func checkName(name string) error {
	if matched, err := regexp.MatchString("[^a-zA-Z\\d]", name); matched || err != nil {
		return errors.New("wallet name must only contain alphabetic characters and numbers")
	}
	return nil
}
func printFees(fees *boltzrpc.SwapFees) {
	fmt.Println("The fees for this service are:")
	fmt.Printf("  - Service fee: %s%%\n", fmt.Sprint(fees.Percentage))
	fmt.Printf("  - Miner fee: %s\n", utils.Satoshis(fees.MinerFees))
	fmt.Printf("Total: %s\n", utils.Satoshis(utils.CalculateFeeEstimate(fees, 0)))
	fmt.Println()
}

func printDeposit(amount uint64, address string, hours float32, blockHeight uint64, limits *boltzrpc.Limits) {
	var amountString string
	if amount == 0 {
		amountString = fmt.Sprintf("between %d and %d satoshis", limits.Minimal, limits.Maximal)
	} else {
		amountString = utils.Satoshis(amount)
	}

	fmt.Printf("Please deposit %s into %s in the next ~%.1f hours (block height %d)\n",
		amountString, address, hours, blockHeight)
}
