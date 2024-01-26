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
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

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

//nolint:staticcheck
func getInfo(ctx *cli.Context) error {
	client := getClient(ctx)
	info, err := client.GetInfo()

	if err != nil {
		return err
	}

	// a bit hacky, but we dont want to show the deprected fields
	info.LndPubkey = ""
	info.Symbol = ""
	info.BlockHeight = 0
	info.PendingReverseSwaps = nil
	info.PendingSwaps = nil

	jsonMarshaler := &protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: false,
	}

	fmt.Println(jsonMarshaler.Format(info))

	if info.AutoSwapStatus == "error" {
		color.New(color.Bold).Println("Autoswap encountered an error. See autoswap status for details.")
	}

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

var walletFlag = &cli.StringFlag{
	Name:  "wallet",
	Usage: "Which wallet to use",
}

var pairFlag = &cli.StringFlag{
	Name:  "pair",
	Value: "BTC/BTC",
	Usage: "Pair id to create a swap for",
}

var pairFilterFlag = &cli.StringFlag{
	Name:  "pair",
	Usage: "Filter swaps by pair",
}
var pendingFilterFlag = &cli.BoolFlag{
	Name:  "pending",
	Usage: "Shorthand for --state pending",
}
var stateFilterFlag = &cli.StringFlag{
	Name:  "state",
	Usage: "Filter swaps by state",
}

var listSwapsCommand = &cli.Command{
	Name:     "listswaps",
	Category: "Info",
	Usage:    "Lists all swaps and reverse swaps",
	Action: func(ctx *cli.Context) error {
		isAuto := ctx.Bool("auto")
		return listSwaps(ctx, &isAuto)
	},
	Flags: []cli.Flag{
		jsonFlag,
		pairFilterFlag,
		pendingFilterFlag,
		stateFilterFlag,
		&cli.BoolFlag{
			Name:  "auto",
			Usage: "Only show swaps by autoswapper",
		},
	},
}

func listSwaps(ctx *cli.Context, isAuto *bool) error {
	client := getClient(ctx)
	request := &boltzrpc.ListSwapsRequest{
		IsAuto: isAuto,
	}
	if pair := ctx.String("pair"); pair != "" {
		request.PairId = &pair
	}
	if ctx.Bool("pending") {
		state := boltzrpc.SwapState_PENDING
		request.State = &state
	} else if state := ctx.String("state"); state != "" {
		stateValue, ok := boltzrpc.SwapState_value[strings.ToUpper(state)]
		if !ok {
			return errors.New("invalid state")
		}
		state := boltzrpc.SwapState(stateValue)
		if ok {
			request.State = &state
		}
	}
	list, err := client.ListSwaps(request)

	if err != nil {
		return err
	}

	if ctx.Bool("json") {
		printJson(list)
	} else {
		headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
		columnFmt := color.New(color.FgYellow).SprintfFunc()

		if len(list.Swaps) == 0 && len(list.ReverseSwaps) == 0 {
			fmt.Println("No swaps found")
			return nil
		}

		if len(list.Swaps) > 0 {

			tbl := table.New("ID", "Pair", "State", "Status", "Amount", "Service Fee", "Onchain Fee", "Created At")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, swap := range list.Swaps {
				tbl.AddRow(swap.Id, swap.PairId, swap.State, swap.Status, swap.ExpectedAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt))
			}

			if _, err := yellowBold.Println("Swaps"); err != nil {
				return err
			}

			tbl.Print()
			fmt.Println()
		}

		if len(list.ReverseSwaps) > 0 {

			tbl := table.New("ID", "Pair", "State", "Status", "Amount", "Service Fee", "Onchain Fee", "Created At")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, swap := range list.ReverseSwaps {
				tbl.AddRow(swap.Id, swap.PairId, swap.State, swap.Status, swap.OnchainAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt))
			}

			if _, err := yellowBold.Println("Reverse Swaps"); err != nil {
				return err
			}
			tbl.Print()
		}
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
	Action: requireNArgs(1, func(ctx *cli.Context) error {
		return swapInfoStream(ctx, ctx.Args().First(), ctx.Bool("json"))
	}),
	Flags: []cli.Flag{jsonFlag},
}

func swapInfoStream(ctx *cli.Context, id string, json bool) error {
	client := getClient(ctx)

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
			Name:  "listswaps",
			Usage: "List swaps created by autoswap",
			Action: func(ctx *cli.Context) error {
				isAuto := true
				return listSwaps(ctx, &isAuto)
			},
			Flags: []cli.Flag{jsonFlag, pairFilterFlag, stateFilterFlag, pendingFilterFlag},
		},
		{
			Name:   "setup",
			Usage:  "Setup autoswap interactively",
			Action: autoSwapSetup,
			Flags:  []cli.Flag{jsonFlag, pairFilterFlag, stateFilterFlag, pendingFilterFlag},
		},
		{
			Name:        "config",
			Usage:       "Manage configuration",
			Description: configDescription,
			Action:      autoSwapConfig,
			ArgsUsage:   "[key] [value]",
			BashComplete: func(ctx *cli.Context) {
				var lastArg string

				if len(os.Args) > 2 {
					lastArg = os.Args[len(os.Args)-2]
				}

				if strings.HasPrefix(lastArg, "-") {
					cli.DefaultCompleteWithFlags(ctx.Command)(ctx)
				} else {
					client := getAutoSwapClient(ctx)

					config, err := client.GetConfig("")
					if err != nil {
						return
					}
					for key := range config.(map[string]any) {
						fmt.Println(key)
					}
				}
			},
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
			Name:  "enable",
			Usage: "Enables the autoswapper",
			Action: func(ctx *cli.Context) error {
				return enableAutoSwap(ctx, true)
			},
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

func printConfig(client client.AutoSwap, key string, asJson, hideZero bool) error {
	config, err := client.GetConfig(key)
	if err != nil {
		return err
	}

	if asJson {
		if hideZero {
			return errors.New("hide zero is not supported for json output")
		}
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

			elements := config.(map[string]any)

			if hideZero {
				for key, value := range elements {
					if reflect.ValueOf(value).IsZero() {
						delete(elements, key)
					}
				}
			}
			if err := toml.NewEncoder(&pretty).Encode(elements); err != nil {
				return err
			}

			fmt.Print(pretty.String())
		}
	}
	return nil
}

func autoSwapSetup(ctx *cli.Context) error {
	client := getClient(ctx)
	autoSwap := getAutoSwapClient(ctx)

	_, err := autoSwap.GetConfig("")
	if err == nil {
		if !prompt("You already have an autoswap configuration. Do you want to reset it?") {
			return nil
		}
	}
	response, err := autoSwap.ResetConfig()
	if err != nil {
		return err
	}
	config := response.(map[string]any)

	var answers struct {
		Currency           string
		Type               string
		AutoBudget         uint64
		AutoBudgetInterval uint64
		MinBalance         uint64
		MaxBalance         uint64
		MinBalancePercent  float64
		MaxBalancePercent  float64
		AcceptZeroConf     bool
		BalanceType        string `json:"-"`
		Wallet             string
	}

	var qs = []*survey.Question{
		{
			Name: "currency",
			Prompt: &survey.Select{
				Message: "Which currency should autoswaps be performed on?",
				Options: []string{"L-BTC", "BTC"},
				Default: fmt.Sprint(config["Currency"]),
			},
		},
		{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Which type of swaps should be executed?",
				Options: []string{"reverse", "normal", "both"},
				Description: func(value string, index int) string {
					switch value {
					case "reverse":
						return "keeps your local balance below set threshold, supports read-only wallet"
					case "normal":
						return "keeps your local balance above set threshold"
					case "both":
						return "maintain a balanced channel between two thresholds"
					}
					return ""
				},
			},
		},
	}

	if err := survey.Ask(qs, &answers); err != nil {
		return err
	}

	readonly := answers.Type != "reverse"
	wallets, err := client.GetWallets(answers.Currency, readonly)
	if err != nil {
		return err
	}

	createNew := "Create New"
	importExisting := "Import Existing"
	var options []string
	for _, wallet := range wallets.Wallets {
		options = append(options, wallet.Name)
	}
	options = append(options, createNew)
	options = append(options, importExisting)

	prompt := &survey.Select{
		Message: fmt.Sprintf("Select %s wallet", answers.Currency),
		Options: options,
	}

	var choice string
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}
	if choice != createNew && choice != importExisting {
		answers.Wallet = choice
	} else {
		input := &survey.Input{
			Message: "Enter a name for the new wallet",
			Default: "autoswap",
		}
		err = survey.AskOne(input, &answers.Wallet, survey.WithValidator(func(ans interface{}) error {
			return checkWalletName(ctx, ans.(string))
		}))

		info := &boltzrpc.WalletInfo{
			Name:     answers.Wallet,
			Currency: answers.Currency,
		}
		if choice == createNew {
			err = createWallet(ctx, info)
		} else if choice == importExisting {
			err = importWallet(ctx, info, answers.Type == "reverse")
		}
	}
	if err != nil {
		return err
	}

	var balanceType string
	prompt = &survey.Select{
		Message: "How do you want to specify min/max balance values?",
		Options: []string{"percentage", "sats"},
	}
	if err := survey.AskOne(prompt, &balanceType); err != nil {
		return err
	}

	qs = []*survey.Question{}
	if answers.BalanceType == "sats" {
		if answers.Type == "both" || answers.Type == "normal" {
			qs = append(qs, &survey.Question{
				Name:     "minBalance",
				Prompt:   &survey.Input{Message: "What is the minimum amount of sats you want to keep in your channels?"},
				Validate: survey.Required,
			})
		}
		if answers.Type == "both" || answers.Type == "reverse" {
			qs = append(qs, &survey.Question{
				Name:     "maxBalance",
				Prompt:   &survey.Input{Message: "What is the maximum amount of sats you want to keep in your channels?"},
				Validate: survey.Required,
			})
		}
	} else {
		if answers.Type == "both" || answers.Type == "normal" {
			qs = append(qs, &survey.Question{
				Name: "minBalancePercent",
				Prompt: &survey.Input{Message: "What is the minimum percentage of total capacity you want to keep in your channels?",
					Default: fmt.Sprint(config["MinBalancePercent"]),
				},
				Validate: survey.Required,
			})
		}
		if answers.Type == "both" || answers.Type == "reverse" {
			qs = append(qs, &survey.Question{
				Name: "maxBalancePercent",
				Prompt: &survey.Input{Message: "What is the maximum percentage of total capacity you want to keep in your channels?",
					Default: fmt.Sprint(config["MaxBalancePercent"]),
				},
				Validate: survey.Required,
			})
		}
	}

	budgetDuration := config["AutoBudgetInterval"].(float64) / (24 * time.Hour).Seconds()
	qs = append(
		qs,
		&survey.Question{
			Name: "AutoBudgetInterval",
			Prompt: &survey.Input{
				Message: "In which interval should the fee budget of the auto swapper be reset? (days)",
				Default: fmt.Sprint(budgetDuration),
			},
		},
		&survey.Question{
			Name: "autoBudget",
			Prompt: &survey.Input{
				Message: "How many sats do you want to spend max on fees per budget interval?",
				Default: fmt.Sprint(config["AutoBudget"]),
			},
		},
		&survey.Question{
			Name: "acceptZeroConf",
			Prompt: &survey.Confirm{
				Message: "Do you want to accept zero conf swaps?",
				Default: config["AcceptZeroConf"].(bool),
			},
		},
	)

	if err := survey.Ask(qs, &answers); err != nil {
		return err
	}

	answers.AutoBudgetInterval = answers.AutoBudgetInterval * 24 * uint64(time.Hour.Seconds())

	_, err = autoSwap.SetConfig(answers)
	if err != nil {
		return err
	}

	fmt.Println("Config was saved successfully!")

	return enableAutoSwap(ctx, false)
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

	return printConfig(client, key, ctx.Bool("json"), false)
}

func enableAutoSwap(ctx *cli.Context, showConfig bool) error {
	client := getAutoSwapClient(ctx)

	if showConfig {
		fmt.Println("Enabling autoswap with the following config:")
		fmt.Println()
		if err := printConfig(client, "", false, true); err != nil {
			return err
		}
	}

	recommendations, err := client.GetSwapRecommendations(true)
	if err != nil {
		return err
	}

	if len(recommendations.Swaps) > 0 {
		fmt.Println("Based on above config the following swaps will be performed immediately:")
		printJson(recommendations)
	}

	fmt.Println()
	if !prompt("Do you want to enable autoswap now?") {
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
		walletFlag,
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

	wallet := ctx.String("wallet")
	swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
		Amount:        amount,
		PairId:        string(pair),
		RefundAddress: ctx.String("refund"),
		AutoSend:      autoSend,
		Wallet:        &wallet,
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

		height := info.BlockHeights[string(boltz.CurrencyForPair(pair))]
		timeoutHours := utils.BlocksToHours(swap.TimeoutBlockHeight-height, utils.GetBlockTime(pair))
		fmt.Printf(
			"Please deposit %s into %s in the next ~%s hours (block height %d)\n",
			amountString, swap.Address, timeoutHours, swap.TimeoutBlockHeight,
		)
		fmt.Println()
	}

	return swapInfoStream(ctx, swap.Id, false)
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
		walletFlag,
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

	wallet := ctx.String("wallet")
	response, err := client.CreateReverseSwap(&boltzrpc.CreateReverseSwapRequest{
		Address:        address,
		Amount:         amount,
		AcceptZeroConf: !ctx.Bool("no-zero-conf"),
		PairId:         pair,
		Wallet:         &wallet,
	})
	if err != nil {
		return err
	}

	if json {
		printJson(response)
	} else {
		return swapInfoStream(ctx, response.Id, false)
	}
	return nil
}

var walletCommands = &cli.Command{
	Name:     "wallet",
	Category: "wallet",
	Usage:    "Manage the wallets used by the client",
	Subcommands: []*cli.Command{
		{
			Name:      "create",
			Usage:     "Create a new wallet",
			ArgsUsage: "name currency",
			Action: requireNArgs(2, func(ctx *cli.Context) error {
				return createWallet(ctx, walletInfo(ctx))
			}),
		},
		{
			Name:      "import",
			Usage:     "Imports an existing wallet",
			ArgsUsage: "name currency",
			Action: requireNArgs(2, func(ctx *cli.Context) error {
				return importWallet(ctx, walletInfo(ctx), true)
			}),
		},
		{
			Name:        "credentials",
			ArgsUsage:   "name",
			Usage:       "Show the credentials of a wallet",
			Description: "Shows the credentials of a wallet. These will be a xpub or core descriptor in case of a readonly wallet and a mnemonic otherwise.",
			Action:      requireNArgs(1, showCredentials),
		},
		{
			Name:   "list",
			Usage:  "List currently used wallets",
			Action: listWallets,
			Flags:  []cli.Flag{jsonFlag},
		},
		{
			Name:      "subaccount",
			Usage:     "Select the subaccount for a wallet",
			ArgsUsage: "name",
			Action:    requireNArgs(1, selectSubaccount),
		},
		{
			Name:      "remove",
			Usage:     "Remove a wallet",
			ArgsUsage: "name",
			Action:    requireNArgs(1, removeWallet),
		},
	},
}

var unlockCommand = &cli.Command{
	Name:  "unlock",
	Usage: "Unlock the server",
	Action: func(ctx *cli.Context) error {
		client := getClient(ctx)
		prompt := survey.Password{Message: "Enter wallet password:"}
		var password string
		if err := survey.AskOne(&prompt, &password); err != nil {
			return err
		}
		if err := client.Unlock(password); err != nil {
			status, _ := status.FromError(err)
			fmt.Println(status.Message())
			return nil
		}
		fmt.Println("boltzd successfully unlocked!")
		return nil
	},
}

var changePasswordCommand = &cli.Command{
	Name:  "changepassword",
	Usage: "Unlock the server",
	Action: func(ctx *cli.Context) error {
		client := getClient(ctx)
		var answers struct {
			Old string
			New string
		}
		qs := []*survey.Question{
			{
				Name:   "Old",
				Prompt: &survey.Password{Message: "Type your old wallet password"},
			},
			{
				Name:   "New",
				Prompt: &survey.Password{Message: "Type your new wallet password"},
			},
		}
		if err := survey.Ask(qs, &answers); err != nil {
			return err
		}
		if err := client.ChangeWalletPassword(answers.Old, answers.New); err != nil {
			return err
		}
		fmt.Println("Password changed")
		return nil
	},
}

var verifyPasswordCommand = &cli.Command{
	Name:  "verifypassword",
	Usage: "Verify the Password",
	Action: func(ctx *cli.Context) error {
		password, err := askPassword(ctx, false)
		if err != nil {
			return err
		}
		if password == "" {
			fmt.Println("No password set")
		} else {
			fmt.Println("Correct")
		}
		return nil
	},
}

func askPassword(ctx *cli.Context, askNew bool) (string, error) {
	client := getClient(ctx)
	hasPassword, err := client.HasPassword()
	if err != nil {
		return "", err
	}
	if !hasPassword {
		if askNew {
			if !prompt("Do you want to provide a wallet password to encrypt your wallet, which will be required on startup?") {
				return "", nil
			}
		} else {
			return "", nil
		}
	}
	prompt := survey.Password{Message: "Please enter your wallet password:"}
	var password string
	validator := survey.WithValidator(func(ans interface{}) error {
		if hasPassword {
			correct, err := client.VerifyWalletPassword(ans.(string))
			if err != nil {
				return err
			}
			if !correct {
				return errors.New("password is incorrect")
			}
		}
		return nil
	})
	if err := survey.AskOne(&prompt, &password, validator); err != nil {
		return "", err
	}
	if !hasPassword {
		prompt := survey.Password{Message: "Retype your new wallet password:"}
		validator := survey.WithValidator(func(ans interface{}) error {
			if ans.(string) != password {
				return errors.New("passwords do not match")
			}
			return nil
		})
		if err := survey.AskOne(&prompt, &password, validator); err != nil {
			return "", err
		}
	}
	return password, nil
}

func printSubaccount(info *boltzrpc.Subaccount) {
	fmt.Printf("Subaccount: %d (%s)\n", info.Pointer, liquidAccountType(info.Type))
	balance := info.Balance

	fmt.Printf("Balance: %s (%s unconfirmed)\n", utils.Satoshis(balance.Total), utils.Satoshis(balance.Unconfirmed))
}

func checkCurrency(currency string) error {
	upper := strings.ToUpper(currency)
	// TODO: allowed values should be retrieved from the server
	if upper != "BTC" && upper != "L-BTC" {
		return fmt.Errorf("invalid currency: %s, allowed values: BTC, L-BTC", currency)
	}
	return nil
}

func walletInfo(ctx *cli.Context) *boltzrpc.WalletInfo {
	return &boltzrpc.WalletInfo{
		Name:     ctx.Args().Get(0),
		Currency: ctx.Args().Get(1),
	}
}

func checkWalletName(ctx *cli.Context, name string) error {
	client := getClient(ctx)
	if err := checkName(name); err != nil {
		return err
	}
	if _, err := client.GetWallet(name); err == nil {
		return fmt.Errorf("wallet %s already exists", name)
	}
	return nil
}

func importWallet(ctx *cli.Context, info *boltzrpc.WalletInfo, readonly bool) error {
	client := getClient(ctx)
	if err := checkWalletName(ctx, info.Name); err != nil {
		return err
	}

	if err := checkCurrency(info.Currency); err != nil {
		return err
	}

	mnemonic := ""
	importType := "mnemonic"
	if strings.EqualFold(info.Currency, "BTC") && readonly {
		prompt := &survey.Select{
			Message: "Which import type do you want to use?",
			Options: []string{"mnemonic", "xpub", "core descriptor"},
			Default: "mnemonic",
		}
		if err := survey.AskOne(prompt, &importType); err != nil {
			return err
		}
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please type your %s", importType),
	}
	if err := survey.AskOne(prompt, &mnemonic, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	credentials := &boltzrpc.WalletCredentials{}
	if importType == "mnemonic" {
		credentials.Mnemonic = &mnemonic
	} else if importType == "xpub" {
		credentials.Xpub = &mnemonic
	} else if importType == "core descriptor" {
		credentials.CoreDescriptor = &mnemonic
	}

	password, err := askPassword(ctx, true)
	if err != nil {
		return err
	}

	wallet, err := client.ImportWallet(info, credentials, password)
	if err != nil {
		return err
	}

	fmt.Println("Successfully imported wallet!")

	if !wallet.Readonly {
		return selectSubaccount(ctx)
	}
	return nil
}

func selectSubaccount(ctx *cli.Context) error {
	client := getClient(ctx)

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Fetching subaccounts..."
	s.Start()

	walletInfo := walletInfo(ctx)
	subaccounts, err := client.GetSubaccounts(walletInfo)
	s.Stop()
	if err != nil {
		return err
	}

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
	if subaccounts.Current != nil {
		accountPrompt.Default = fmt.Sprint(*subaccounts.Current)
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

	response, err := client.SetSubaccount(walletInfo.Name, subaccount)
	if err != nil {
		return err
	}

	printSubaccount(response)
	return nil
}

func removeWallet(ctx *cli.Context) error {
	if !prompt("Make sure to have a backup of the wallet. Do you want to continue?") {
		return nil
	}
	client := getClient(ctx)
	_, err := client.RemoveWallet(ctx.Args().First())
	return err
}

func createWallet(ctx *cli.Context, info *boltzrpc.WalletInfo) error {
	client := getClient(ctx)

	if err := checkWalletName(ctx, info.Name); err != nil {
		return err
	}
	if err := checkCurrency(info.Currency); err != nil {
		return err
	}

	password, err := askPassword(ctx, true)
	if err != nil {
		return err
	}

	credentials, err := client.CreateWallet(info, password)
	if err != nil {
		return err
	}
	fmt.Println("New wallet created!")
	fmt.Println()
	fmt.Println("Mnemonic:\n" + *credentials.Mnemonic)
	fmt.Println()
	fmt.Println("We highly recommend to import the mnemonic shown above into an external wallet like Blockstream Green (https://blockstream.com/green)." +
		"This serves as backup and allows you to view transactions and control your funds.")
	return nil
}

func showCredentials(ctx *cli.Context) error {
	client := getClient(ctx)
	if prompt("Make sure no one can see your screen. Do you want to continue?") {
		password, err := askPassword(ctx, false)
		if err != nil {
			return err
		}
		response, err := client.GetWalletCredentials(ctx.Args().First(), password)
		if err != nil {
			return err
		}
		printJson(response)
	}
	return nil
}

func listWallets(ctx *cli.Context) error {
	client := getClient(ctx)
	wallets, err := client.GetWallets("", true)
	if err != nil {
		return err
	}
	printJson(wallets)
	// if ctx.Bool("json") {
	// 	printJson(info)
	// } else {
	// 	printSubaccount(info.Subaccount)
	// }
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
