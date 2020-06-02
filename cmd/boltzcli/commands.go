package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/urfave/cli"
	"os"
)

var getInfoCommand = cli.Command{
	Name:   "getinfo",
	Usage:  "Returns basic information",
	Action: getInfo,
}

func getInfo(ctx *cli.Context) error {
	client := getClient(ctx)
	info, err := client.GetInfo()

	if err != nil {
		return err
	}

	printJson(info)

	return err
}

var getSwapCommand = cli.Command{
	Name:   "swapinfo",
	Usage:  "Gets all information about a Swap",
	Action: swapInfo,
}

func swapInfo(ctx *cli.Context) error {
	client := getClient(ctx)
	swapInfo, err := client.GetSwapInfo(ctx.Args().First())

	if err != nil {
		return err
	}

	printJson(swapInfo)

	return err
}

var createSwapCommand = cli.Command{
	Name:      "createswap",
	Usage:     "Creates a new Swap",
	ArgsUsage: "amount",
	Action:    createSwap,
}

func createSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	swap, err := client.CreateSwap(ctx.Args().First())

	if err != nil {
		return err
	}

	printJson(swap)

	return err
}

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
