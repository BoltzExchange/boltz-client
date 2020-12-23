package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/urfave/cli"
	"io/ioutil"
	"path"
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

	return nil
}

var listSwapsCommand = cli.Command{
	Name:     "listswaps",
	Category: "Info",
	Usage:    "Lists all Swaps, Channel Creations and Reverse Swaps",
	Action:   listSwaps,
}

func listSwaps(ctx *cli.Context) error {
	client := getClient(ctx)
	list, err := client.ListSwaps()

	if err != nil {
		return err
	}

	printJson(list)

	return nil
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

	return nil
}

var depositCommand = cli.Command{
	Name:     "deposit",
	Category: "Auto",
	Usage:    "Deposits into your lightning node",
	Action:   deposit,
	Flags: []cli.Flag{
		cli.UintFlag{
			Name:  "inbound",
			Value: 25,
			Usage: "Amount of inbound liquidity in percent in case a channel gets created for the Swap",
		},
	},
}

func deposit(ctx *cli.Context) error {
	client := getClient(ctx)
	response, err := client.Deposit(ctx.Uint("inbound"))

	if err != nil {
		return err
	}

	info, err := client.GetInfo()

	if err != nil {
		return err
	}

	serviceInfo, err := client.GetServiceInfo()

	if err != nil {
		return err
	}

	smallestUnitName := utils.GetSmallestUnitName(info.Symbol) + "s"
	timeoutHours := utils.BlocksToHours(response.TimeoutBlockHeight-info.BlockHeight, utils.GetBlockTime(info.Symbol))

	fmt.Println("You will receive your deposit in a lightning channel. If you do not have a channel with sufficient capacity yet, Boltz will open a channel.")
	fmt.Println("The fees for this service are:")
	fmt.Println("  - Service fee: " + formatPercentageFee(serviceInfo.Fees.Percentage) + "%")
	fmt.Println("  - Miner fee: " + strconv.Itoa(int(serviceInfo.Fees.Miner.Normal)) + " " + smallestUnitName)
	fmt.Println()
	fmt.Println(
		"Please deposit between " + strconv.Itoa(int(serviceInfo.Limits.Minimal)) + " and " + strconv.Itoa(int(serviceInfo.Limits.Maximal)) +
			" " + smallestUnitName + " into " + response.Address + " in the next ~" + timeoutHours + " hours " +
			"(block height " + strconv.Itoa(int(response.TimeoutBlockHeight)) + ")",
	)

	return nil
}

var withdrawCommand = cli.Command{
	Name:      "withdraw",
	Category:  "Auto",
	Usage:     "Withdraw from your lightning node",
	ArgsUsage: "amount address",
	Action:    withdraw,
}

func withdraw(ctx *cli.Context) error {
	client := getClient(ctx)

	address := ctx.Args().Get(1)

	amount := parseInt64(ctx.Args().First(), "amount")

	if address == "" {
		fmt.Println("No withdraw address was specified")
		return nil
	}

	info, err := client.GetInfo()

	if err != nil {
		return err
	}

	serviceInfo, err := client.GetServiceInfo()

	if err != nil {
		return err
	}

	smallestUnitName := utils.GetSmallestUnitName(info.Symbol) + "s"

	fmt.Println("You will receive the withdrawal to the specified onchain address")
	fmt.Println("The fees for this service are:")
	fmt.Println("  - Service fee: " + formatPercentageFee(serviceInfo.Fees.Percentage) + "%")
	fmt.Println("  - Miner fee: " + strconv.Itoa(int(serviceInfo.Fees.Miner.Reverse)) + " " + smallestUnitName)
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
	fmt.Println("Routing fee paid: " + utils.FormatMilliSat(int64(response.RoutingFeeMilliSat)) + " " + smallestUnitName)
	fmt.Println("Transaction id: " + response.ClaimTransactionId)

	return nil
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

	return nil
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

	return nil
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

	return nil
}

var formatMacaroonCommand = cli.Command{
	Name:     "formatmacaroon",
	Category: "Debug",
	Usage:    "Formats the specified macaroon in hex",
	Action:   formatMacaroon,
}

func formatMacaroon(ctx *cli.Context) error {
	macaroonDir := path.Join(ctx.GlobalString("datadir"), "macaroons")
	macaroonPath := ctx.GlobalString("macaroon")

	macaroonPath = utils.ExpandDefaultPath(macaroonDir, macaroonPath, "admin.macaroon")

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)

	if err != nil {
		return errors.New("could not read macaroon file \"" + macaroonPath + "\": " + err.Error())
	}

	fmt.Println(hex.EncodeToString(macaroonBytes))
	return nil
}
