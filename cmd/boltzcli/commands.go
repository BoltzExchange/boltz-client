package main

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/urfave/cli"
	"strconv"
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

var depositCommand = cli.Command{
	Name:     "deposit",
	Category: "Auto",
	Usage:    "Deposits into your lightning node",
	Action:   deposit,
}

func deposit(ctx *cli.Context) error {
	client := getClient(ctx)
	response, err := client.Deposit()

	if err != nil {
		return err
	}

	info, err := client.GetInfo()

	if err != nil {
		return err
	}

	var blockTime float64 = 0

	switch info.Symbol {
	case "BTC":
		blockTime = utils.BitcoinBlockTime

	case "LTC":
		blockTime = utils.LitecoinBlockTime
	}

	timeoutHours := utils.BlocksToHours(response.TimeoutBlockHeight-info.BlockHeight, blockTime)

	fmt.Println("You will receive your deposit in a lightning channel. If you do not have a channel with sufficient capacity yet, Boltz will open a channel.")
	fmt.Println("The fees for this service are:")
	fmt.Println("  - Service fee: " + strconv.Itoa(int(response.Fees.Percentage)) + "%")
	fmt.Println("  - Miner fee: " + strconv.Itoa(int(response.Fees.Miner.Normal)) + " satoshis")
	fmt.Println()
	fmt.Println(
		"Please deposit between " + strconv.Itoa(int(response.Limits.Minimal)) + " and " + strconv.Itoa(int(response.Limits.Maximal)) +
			" satoshis into " + response.Address + " in the next ~" + timeoutHours + " hours " +
			"(block height " + strconv.Itoa(int(response.TimeoutBlockHeight)) + ")",
	)

	return nil
}

var withdrawCommand = cli.Command{
	Name:     "withdraw",
	Category: "Auto",
	Usage:    "Withdraw from your lightning node",
	Action:   withdraw,
}

func withdraw(ctx *cli.Context) error {
	client := getClient(ctx)

	address := ctx.Args().Get(1)

	if address == "" {
		fmt.Println("No withdraw address was specified")
		return nil
	}

	amount := parseInt64(ctx.Args().First(), "amount")

	serviceInfo, err := client.GetServiceInfo()

	if err != nil {
		return err
	}

	fmt.Println("You will receive the withdrawal to the specified onchain address")
	fmt.Println("The fees for this service are:")
	fmt.Println("  - Service fee: " + strconv.Itoa(int(serviceInfo.Fees.Percentage)) + "%")
	fmt.Println("  - Miner fee: " + strconv.Itoa(int(serviceInfo.Fees.Miner.Reverse)) + " satoshis")
	fmt.Println()

	if !prompt("Do you want to continue?") {
		return nil
	}

	fmt.Println("Withdrawing...")

	response, err := client.CreateReverseSwap(amount, address, true)

	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Routing fee paid: " + utils.FormatMilliSat(int64(response.RoutingFeeMilliSat)) + " sats")
	fmt.Println("Transaction id: " + response.ClaimTransactionId)

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

// TODO: allow zero conf via cli argument
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
		false,
	)

	if err != nil {
		return err
	}

	printJson(swap)

	return err
}
