package main

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/client"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/BurntSushi/toml"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/urfave/cli/v2"
)

var yellowBold = color.New(color.FgHiYellow, color.Bold)

var getInfoCommand = &cli.Command{
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

var jsonFlag = &cli.BoolFlag{
	Name:  "json",
	Usage: "Prints the output as JSON",
}

var liquidFlag = &cli.BoolFlag{
	Name:  "liquid",
	Usage: "Shorthand for --pair L-BTC/BTC",
}

var pairFlag = &cli.StringFlag{
	Name:  "pair",
	Value: "BTC/BTC",
	Usage: "Pair id to create a swap for",
}

var listSwapsCommand = &cli.Command{
	Name:     "listswaps",
	Category: "Info",
	Usage:    "Lists all swaps and reverse swaps",
	Action:   listSwaps,
	Flags:    []cli.Flag{jsonFlag},
}

func listSwaps(ctx *cli.Context) error {
	client := getClient(ctx)
	list, err := client.ListSwaps()

	if err != nil {
		return err
	}

	if ctx.Bool("json") {
		printJson(list)
	} else {
		headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
		columnFmt := color.New(color.FgYellow).SprintfFunc()
		tbl := table.New("ID", "State", "Status", "Amount", "Service Fee", "Onchain Fee", "Created At", "Pair")
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

		for _, swap := range list.Swaps {
			tbl.AddRow(swap.Id, swap.State, swap.Status, swap.ExpectedAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt), swap.PairId)
		}

		if _, err := yellowBold.Println("Swaps"); err != nil {
			return err
		}

		tbl.Print()

		tbl = table.New("ID", "State", "Status", "Amount", "Service Fee", "Onchain Fee", "Created At", "Pair")
		tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

		for _, swap := range list.ReverseSwaps {
			tbl.AddRow(swap.Id, swap.State, swap.Status, swap.OnchainAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt), swap.PairId)
		}

		fmt.Println()
		if _, err := yellowBold.Println("Reverse Swaps"); err != nil {
			return err
		}
		tbl.Print()
	}

	return nil
}

var getSwapCommand = &cli.Command{
	Name:      "swapinfo",
	Category:  "Info",
	Usage:     "Gets all available information about a swap",
	ArgsUsage: "id",
	Action:    requireNArgs(1, swapInfo),
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

var swapInfoStreamCommand = &cli.Command{
	Name:      "swapinfostream",
	Category:  "Info",
	Usage:     "Streams updates of a swap",
	ArgsUsage: "id",
	Action:    requireNArgs(1, swapInfoStreamAction),
	Flags:     []cli.Flag{jsonFlag},
}

func swapInfoStreamAction(ctx *cli.Context) error {
	return swapInfoStream(getClient(ctx), ctx.Args().First(), ctx.Bool("json"))
}

func swapInfoStream(client client.Boltz, id string, json bool) error {
	stream, err := client.GetSwapInfoStream(id)
	if err != nil {
		return err
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Waiting for next update..."

	for {
		if !json {
			s.Start()
		}
		info, err := stream.Recv()
		if !json {
			s.Stop()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if json {
			printJson(info)
		} else {
			if info.Swap != nil {
				yellowBold.Printf("Swap Status: %s\n", info.Swap.Status)

				switch info.Swap.State {
				case boltzrpc.SwapState_ERROR:
					fmt.Printf("Error: %s\n", info.Swap.Error)
				case boltzrpc.SwapState_REFUNDED:
					fmt.Println("Swap was refunded")
				}

				status := boltz.ParseEvent(info.Swap.Status)
				switch status {
				case boltz.SwapCreated:
					fmt.Printf("Swap ID: %s\n", info.Swap.Id)
				case boltz.TransactionMempool:
					fmt.Printf("Transaction ID: %s\nAmount: %dsat\n", info.Swap.LockupTransactionId, info.Swap.ExpectedAmount)
				case boltz.InvoiceSet:
					fmt.Printf("Invoice: %s\n", info.Swap.Invoice)
				case boltz.TransactionClaimed:
					fmt.Printf("Paid %dsat onchain fee and %dsat service fee\n", *info.Swap.OnchainFee, *info.Swap.ServiceFee)
					return nil
				}
			} else if info.ReverseSwap != nil {
				yellowBold.Printf("Swap Status: %s\n", info.ReverseSwap.Status)

				swap := info.ReverseSwap
				switch swap.State {
				case boltzrpc.SwapState_ERROR:
					fmt.Printf("Error: %s", info.ReverseSwap.Error)
					return nil
				}

				status := boltz.ParseEvent(swap.Status)
				switch status {
				case boltz.SwapCreated:
					fmt.Printf("Swap ID: %s\n", swap.Id)
				case boltz.TransactionMempool:
					fmt.Printf("Lockup Transaction ID: %s\n", swap.LockupTransactionId)
				case boltz.InvoiceSettled:
					fmt.Printf("Claim Transaction ID: %s\n", swap.ClaimTransactionId)
					fmt.Printf("Paid %dmsat routing fee, %dsat onchain fee and %dsat service fee\n", *swap.RoutingFeeMsat, *swap.OnchainFee, *swap.ServiceFee)
					return nil
				}
			}
			fmt.Println()
		}
	}

	return nil
}

var configDescription string = `View and edit configuration of the autoswapper.
By default, the whole config is shown, altough a certain key can be specified.
A new value for the key can also be provided.
The configuration file autoswap.toml is located inside the data directory of the daemon and can be edited manually too.`

var autoSwapCommands = &cli.Command{
	Name:    "autoswap",
	Aliases: []string{"auto"},
	Usage:   "Manage the autoswapper",
	Description: "Autoswap keeps your lightning node balanced by automatically executing swaps.\n" +
		"It regularly checks your nodes channels and creates swaps based on your configuration, which can be managed with the `config` command.\n" +
		"You can also configure the autoswapper without starting it and see what it would do with the `recommendations` command.\n" +
		"Once you are confident with the configuration, you can enable the autoswapper with the `enable` command.\n",
	Subcommands: []*cli.Command{
		{
			Name:   "status",
			Usage:  "Show status of autoswap",
			Action: autoSwapStatus,
			Flags:  []cli.Flag{jsonFlag},
		},
		{
			Name:   "recommendations",
			Usage:  "List recommended swaps",
			Action: listSwapRecommendations,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "no-dismissed",
					Usage: "Do not show dismissed recommendations",
				},
			},
		},
		{
			Name:        "config",
			Usage:       "Manage configuration",
			Description: configDescription,
			Action:      autoSwapConfig,
			ArgsUsage:   "[key] [value]",
			Flags: []cli.Flag{
				jsonFlag,
				&cli.BoolFlag{
					Name:  "reload",
					Usage: "Reloads the config from the filesystem before any action is taken. Use if you manually changed the configuration file",
				},
				&cli.BoolFlag{
					Name:  "reset",
					Usage: "Resets to the default configuration",
				},
			},
		},
		{
			Name:   "enable",
			Usage:  "Enables the autoswapper",
			Action: enableAutoSwap,
		},
		{
			Name:   "disable",
			Usage:  "Disables the autoswapper",
			Action: disableAutoSwap,
		},
	},
}

func listSwapRecommendations(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)
	list, err := client.GetSwapRecommendations(ctx.Bool("no-dismissed"))

	if err != nil {
		return err
	}

	printJson(list)

	return nil
}

func autoSwapStatus(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)
	response, err := client.GetStatus()

	if err != nil {
		return err
	}

	if ctx.Bool("json") {
		printJson(response)
	} else {
		if response.Running {
			color.New(color.FgGreen, color.Bold).Println("Running")
		} else if response.Error != "" {
			color.New(color.FgRed, color.Bold).Println("Failed to start")
			fmt.Println("Error: " + response.Error)
		} else {
			color.New(color.FgYellow, color.Bold).Println("Disabled")
		}
		if response.Strategy != "" {
			fmt.Printf("Strategy: %s\n", response.Strategy)
		}
		if response.Budget != nil {
			yellowBold.Println("\nBudget")
			fmt.Printf(" - From %s until %s\n", parseDate(response.Budget.StartDate), parseDate(response.Budget.EndDate))
			fmt.Println(" - Total: " + utils.Satoshis(response.Budget.Total))
			fmt.Println(" - Remaining: " + utils.Satoshis(response.Budget.Remaining))

			yellowBold.Println("Stats")
			fmt.Println(" - Swaps: " + strconv.Itoa(int(response.Stats.Count)))
			fmt.Println(" - Amount: " + utils.Satoshis(response.Stats.TotalAmount) + " (avg " + utils.Satoshis(response.Stats.AvgAmount) + ")")
			fmt.Println(" - Fees: " + utils.Satoshis(response.Stats.TotalFees) + " (avg " + utils.Satoshis(response.Stats.AvgFees) + ")")
		}

	}

	return nil
}

func printConfig(client client.AutoSwap, key string, asJson bool) error {
	response, err := client.GetConfig(key)
	if err != nil {
		return err
	}

	var config any
	if err := json.Unmarshal([]byte(response.Json), &config); err != nil {
		return err
	}

	if asJson {
		pretty, err := json.MarshalIndent(config, "", "   ")
		if err != nil {
			return err
		}
		fmt.Println(string(pretty))
	} else {
		if key != "" {
			fmt.Println(config)
		} else {
			var pretty bytes.Buffer
			if err := toml.NewEncoder(&pretty).Encode(config); err != nil {
				return err
			}

			fmt.Print(pretty.String())
		}
	}
	return nil
}

func autoSwapConfig(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)

	if ctx.Bool("reset") {
		if _, err := client.ResetConfig(); err != nil {
			return err
		}
	}
	if ctx.Bool("reload") {
		if _, err := client.ReloadConfig(); err != nil {
			return err
		}
	}

	key := ctx.Args().First()
	if ctx.NArg() == 2 {
		args := ctx.Args()
		if _, err := client.SetConfigValue(args.Get(0), args.Get(1)); err != nil {
			return err
		}
	}

	return printConfig(client, key, ctx.Bool("json"))
}

func enableAutoSwap(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)

	fmt.Println("Enabling autoswap with the following config:")

	fmt.Println()
	if err := printConfig(client, "", false); err != nil {
		return err
	}
	fmt.Println()

	recommendations, err := client.GetSwapRecommendations(true)
	if err != nil {
		return err
	}

	if len(recommendations.Swaps) > 0 {
		fmt.Println("Based on above config the following swaps will be performed immediately after enabling autoswap:")
		printJson(recommendations)
	}

	fmt.Println()
	if !prompt("Do you want to continue?") {
		return nil
	}

	if _, err := client.Enable(); err != nil {
		return err
	}
	return autoSwapStatus(ctx)
}

func disableAutoSwap(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)
	_, err := client.Disable()
	fmt.Println("Disabled")
	return err
}

var createSwapCommand = &cli.Command{
	Name:      "createswap",
	Category:  "Swaps",
	Usage:     "Create a new swap",
	ArgsUsage: "[amount]",
	Description: "Creates a new swap (onchain -> lightning) specifying the amount in satoshis.\n" +
		"If the --any-amount flag is specified, any amount within the displayed limits can be paid to the lockup address.\n" +
		"\nExamples\n" +
		"Create a swap for 100000 satoshis that will be immediately paid by the clients wallet:\n" +
		"> boltzcli createswap --auto-send 100000\n" +
		"Create a swap for any amount of satoshis on liquid:\n" +
		"> boltzcli createswap --any-amount --pair L-BTC/BTC",
	Action: createSwap,
	Flags: []cli.Flag{
		jsonFlag,
		pairFlag,
		liquidFlag,
		&cli.BoolFlag{
			Name:  "auto-send",
			Usage: "Whether to automatically send the specified amount from the daemon wallet.",
		},
		&cli.BoolFlag{
			Name:  "any-amount",
			Usage: "Allow any amount within the limits to be paid to the lockup address.",
		},
		&cli.StringFlag{
			Name:  "refund",
			Usage: "Address to refund to in case the swap fails",
		},
	},
}

func createSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	var amount int64
	if ctx.Args().First() != "" {
		amount = parseInt64(ctx.Args().First(), "amount")
	} else if !ctx.Bool("any-amount") {
		return cli.ShowSubcommandHelp(ctx)
	}

	pair, err := boltz.ParsePair(getPair(ctx))
	if err != nil {
		return err
	}

	autoSend := ctx.Bool("auto-send")
	json := ctx.Bool("json")

	serviceInfo, err := client.GetServiceInfo(string(pair))
	if err != nil {
		return err
	}

	if !json {
		fmt.Println("You will receive your deposit via lightning.")
		fmt.Println("The fees for this service are:")
		fmt.Println("  - Service fee: " + formatPercentageFee(serviceInfo.Fees.Percentage) + "%")
		fmt.Println("  - Miner fee: " + utils.Satoshis(int(serviceInfo.Fees.Miner.Normal)))
		fmt.Println()

		if !prompt("Do you want to continue?") {
			return nil
		}
	}

	swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
		Amount:        parseInt64(ctx.Args().First(), "amount"),
		PairId:        ctx.String("pair"),
		RefundAddress: ctx.String("refund"),
	})
	if err != nil {
		return err
	}

	if json {
		printJson(swap)
		return nil
	}

	if !autoSend || amount == 0 {
		info, err := client.GetInfo()
		if err != nil {
			return err
		}

		var amountString string
		if amount == 0 {
			amountString = fmt.Sprintf("between %d and %d satoshis", serviceInfo.Limits.Minimal, serviceInfo.Limits.Maximal)
		} else {
			amountString = utils.Satoshis(int(amount))
		}

		height := info.BlockHeights[string(pair)]
		timeoutHours := utils.BlocksToHours(swap.TimeoutBlockHeight-height, utils.GetBlockTime(pair))
		fmt.Printf(
			"Please deposit %s into %s in the next ~%s hours (block height %d)\n",
			amountString, swap.Address, timeoutHours, swap.TimeoutBlockHeight,
		)
		fmt.Println()
	}

	return swapInfoStream(client, swap.Id, false)
}

var createReverseSwapCommand = &cli.Command{
	Name:      "createreverseswap",
	Category:  "Swaps",
	Usage:     "Create a new reverse swap",
	ArgsUsage: "amount [address]",
	Description: "Creates a new reverse swap (lightning -> onchain) for `amount` satoshis, optionally specifying the destination address.\n" +
		"If no address is specified, it will be generated by the clients wallet.\n" +
		"\nExamples\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the clients btc wallet:\n" +
		"> boltzcli createreverseswap 100000\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the specified btc address:\n" +
		"> boltzcli createreverseswap 100000 bcrt1qkp70ncua3dqp6syqu24jw5mnpf3gdxqrm3gn2a\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the clients liquid wallet:\n" +
		"> boltzcli createreverseswap --pair L-BTC 100000",
	Action: requireNArgs(1, createReverseSwap),
	Flags: []cli.Flag{
		jsonFlag,
		pairFlag,
		liquidFlag,
		&cli.BoolFlag{
			Name:  "no-zero-conf",
			Usage: "Disable zero-conf for this swap",
		},
	},
}

func createReverseSwap(ctx *cli.Context) error {
	client := getClient(ctx)

	address := ctx.Args().Get(1)

	pair := getPair(ctx)

	amount := parseInt64(ctx.Args().First(), "amount")
	json := ctx.Bool("json")

	if !json {
		serviceInfo, err := client.GetServiceInfo(pair)
		if err != nil {
			return err
		}

		fmt.Println("You will receive the withdrawal to the specified onchain address")
		fmt.Println("The fees for this service are:")
		fmt.Println("  - Service fee: " + formatPercentageFee(serviceInfo.Fees.Percentage) + "%")
		fmt.Println("  - Miner fee: " + utils.Satoshis(int(serviceInfo.Fees.Miner.Reverse)))
		fmt.Println()

		if !prompt("Do you want to continue?") {
			return nil
		}
	}

	response, err := client.CreateReverseSwap(amount, address, !ctx.Bool("no-zero-conf"), pair)
	if err != nil {
		return err
	}

	if json {
		printJson(response)
	} else {
		return swapInfoStream(client, response.Id, false)
	}
	return nil
}

var liquidWalletCommands = &cli.Command{
	Name:     "liquid",
	Category: "wallet",
	Usage:    "Manage the liquid wallet used by the client",
	Subcommands: []*cli.Command{
		{
			Name:   "create",
			Usage:  "Creates a new liquid wallet",
			Action: createWallet,
		},
		{
			Name:   "import",
			Usage:  "Imports a liquid wallet",
			Action: importWallet,
		},
		{
			Name:   "showmnemonic",
			Usage:  "Shows the mnemonic of the currently used liquid wallet",
			Action: showMnemonic,
		},
		{
			Name:   "info",
			Usage:  "Shows information about currently used liquid wallet",
			Action: showLiquidWalletInfo,
			Flags:  []cli.Flag{jsonFlag},
		},
		{
			Name:   "subaccount",
			Usage:  "Select the subaccount to be used",
			Action: selectSubaccountAction,
		},
		{
			Name:   "remove",
			Usage:  "Remove the liquid wallet",
			Action: removeWallet,
		},
	},
}

func printSubaccount(info *boltzrpc.LiquidSubaccount) {
	fmt.Printf("Subaccount: %d (%s)\n", info.Pointer, liquidAccountType(info.Type))
	balance := info.Balance
	fmt.Printf("Balance: %s (%s unconfirmed)\n", utils.Satoshis(balance.Total), utils.Satoshis(balance.Unconfirmed))
}

func deleteExistingWallet(client client.Boltz) (bool, error) {
	if _, err := client.GetLiquidWalletInfo(); err == nil {
		if !prompt("There is an existing liquid wallet, make sure to have a backup of the mnemonic. Do you want to continue?") {
			return false, nil
		}
		if _, err := client.RemoveLiquidWallet(); err != nil {
			return false, errors.New("could not delete existing wallet: " + err.Error())
		}
	}
	return true, nil
}

func importWallet(ctx *cli.Context) error {
	client := getClient(ctx)
	if ok, err := deleteExistingWallet(client); !ok {
		return err
	}

	mnemonic := ""
	prompt := &survey.Input{
		Message: "Please type your mnemonic",
	}

	if err := survey.AskOne(prompt, &mnemonic); err != nil {
		return err
	}

	_, err := client.ImportLiquidWallet(mnemonic)
	if err != nil {
		return errors.New("could not login: " + err.Error())
	}

	fmt.Println("Successfully imported wallet!")
	return selectSubaccount(client)
}

func selectSubaccountAction(ctx *cli.Context) error {
	return selectSubaccount(getClient(ctx))
}

func selectSubaccount(client client.Boltz) error {
	info, _ := client.GetLiquidWalletInfo()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Fetching subaccounts..."
	s.Start()

	subaccounts, err := client.GetLiquidSubaccounts()
	if err != nil {
		return err
	}
	s.Stop()

	var options = []string{"new"}
	for _, subaccount := range subaccounts.Subaccounts {
		options = append(options, fmt.Sprint(subaccount.Pointer))
	}

	accountPrompt := &survey.Select{
		Message: "Which subaccount should be used?",
		Options: options,
		Description: func(_ string, index int) string {
			if index == 0 {
				return ""
			}
			subaccount := subaccounts.Subaccounts[index-1]
			return fmt.Sprintf("%s (%s)", utils.Satoshis(subaccount.Balance.Total), liquidAccountType(subaccount.Type))
		},
	}
	if info != nil {
		accountPrompt.Default = fmt.Sprint(info.Subaccount.Pointer)
	}

	var subaccountRaw string

	if err := survey.AskOne(accountPrompt, &subaccountRaw); err != nil {
		return err
	}
	var subaccount *uint64

	if subaccountRaw != "new" {
		parsed, err := strconv.ParseUint(subaccountRaw, 10, 64)
		if err != nil {
			return err
		}
		subaccount = &parsed
	}

	info, err = client.SetLiquidSubaccount(subaccount)
	if err != nil {
		return err
	}

	printSubaccount(info.Subaccount)
	return nil
}

func removeWallet(ctx *cli.Context) error {
	client := getClient(ctx)
	if _, err := client.GetLiquidWalletInfo(); err != nil {
		return errors.New("no wallet found")
	}
	if !prompt("Make sure to have a backup of the mnemonic. Do you want to continue?") {
		return nil
	}
	_, err := client.RemoveLiquidWallet()
	return err
}

func createWallet(ctx *cli.Context) error {
	client := getClient(ctx)
	if ok, err := deleteExistingWallet(client); !ok {
		return err
	}

	mnemonic, err := client.CreateLiquidWallet()
	if err != nil {
		return err
	}
	fmt.Println("New liquid wallet created with the following mnemonic:\n" + mnemonic.Mnemonic)
	return nil
}

func showMnemonic(ctx *cli.Context) error {
	client := getClient(ctx)
	if prompt("Make sure no one can see your screen. Do you want to continue?") {
		response, err := client.GetLiquidWalletMnemonic()
		if err != nil {
			return errors.New("could not get mnemonic: " + err.Error())
		}
		fmt.Println(response.Mnemonic)
	}
	return nil
}

func showLiquidWalletInfo(ctx *cli.Context) error {
	client := getClient(ctx)
	info, err := client.GetLiquidWalletInfo()
	if err != nil {
		return err
	}
	if ctx.Bool("json") {
		printJson(info)
	} else {
		printSubaccount(info.Subaccount)
	}
	return nil
}

var formatMacaroonCommand = &cli.Command{
	Name:     "formatmacaroon",
	Category: "Debug",
	Usage:    "Formats the macaroon for connecting to boltz-client in hex",
	Action:   formatMacaroon,
}

func formatMacaroon(ctx *cli.Context) error {
	macaroonDir := path.Join(ctx.String("datadir"), "macaroons")
	macaroonPath := ctx.String("macaroon")

	macaroonPath = utils.ExpandDefaultPath(macaroonDir, macaroonPath, "admin.macaroon")

	macaroonBytes, err := os.ReadFile(macaroonPath)

	if err != nil {
		return errors.New("could not read macaroon file \"" + macaroonPath + "\": " + err.Error())
	}

	fmt.Println(hex.EncodeToString(macaroonBytes))
	return nil
}

//go:embed autocomplete/bash_autocomplete
var bashComplete []byte

//go:embed autocomplete/zsh_autocomplete
var zshComplete []byte

var shellCompletionsCommand = &cli.Command{
	Name:  "completions",
	Usage: "Sets up shell completions for the cli",
	Action: func(ctx *cli.Context) error {
		dataDir := ctx.String("datadir")

		shell := os.Getenv("SHELL")
		var scriptPath, rc string
		var script []byte
		if strings.Contains(shell, "zsh") {
			scriptPath = path.Join(dataDir, "zsh_autocomplete")
			script = zshComplete
			rc = "~/.zshrc"
		} else if strings.Contains(shell, "bash") {
			scriptPath = path.Join(dataDir, "bash_autocomplete")
			script = bashComplete
			rc = "~/.bashrc"
		} else {
			return errors.New("unknown shell")
		}
		if err := os.WriteFile(scriptPath, script, 0666); err != nil {
			return err
		}
		file, err := os.OpenFile(utils.ExpandHomeDir(rc), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		if err != nil {
			return err
		}
		defer file.Close()
		content := fmt.Sprintf("\n# load completions for boltzcli\nPROG=boltzcli source %s", scriptPath)
		if _, err := file.WriteString(content); err != nil {
			return err
		}
		fmt.Printf("You should now get completions by hitting tab after restarting your shell or sourcing %s\n", rc)

		return nil
	},
}

var stopCommand = &cli.Command{
	Name:  "stop",
	Usage: "Stops the daemon",
	Action: func(ctx *cli.Context) error {
		client := getClient(ctx)
		return client.Stop()
	},
}
