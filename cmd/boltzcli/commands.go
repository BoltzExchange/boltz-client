package main

import (
	"github.com/urfave/cli"
)

var getInfoCommand = cli.Command{
	Name:     "getinfo",
	Category: "Info",
	Usage:    "Returns basic information",
	Action:   getInfo,
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
	Name:     "swapinfo",
	Category: "Info",
	Usage:    "Gets all available information about a Swap",
	Action:   swapInfo,
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
	Category:  "Manual",
	Usage:     "Creates a new Swap",
	ArgsUsage: "amount",
	Action:    createSwap,
}

func createSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	swap, err := client.CreateSwap(
		parseInt64(ctx.Args().First(), "amount"),
	)

	if err != nil {
		return err
	}

	printJson(swap)

	return err
}

var createChannelCreationCommand = cli.Command{
	Name:      "createchannel",
	Category:  "Manual",
	Usage:     "Creates a new Channel Creation",
	ArgsUsage: "amount inbound",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "private",
			Usage: "Whether the channel should be private",
		},
	},
	Action: createChannelCreation,
}

func createChannelCreation(ctx *cli.Context) error {
	client := getClient(ctx)

	private := ctx.Bool("private")
	channelCreation, err := client.CreateChannelCreation(
		parseInt64(ctx.Args().First(), "amount"),
		uint32(parseInt64(ctx.Args().Get(1), "inbound liquidity")),
		private,
	)

	if err != nil {
		return err
	}

	printJson(channelCreation)

	return err
}

var createReverseSwapCommand = cli.Command{
	Name:      "createreverseswap",
	Category:  "Manual",
	Usage:     "Creates a new Reverse Swap",
	ArgsUsage: "amount [address]",
	Action:    createReverseSwap,
}

func createReverseSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	swap, err := client.CreateReverseSwap(
		parseInt64(ctx.Args().First(), "amount"),
		ctx.Args().Get(1),
	)

	if err != nil {
		return err
	}

	printJson(swap)

	return err
}
