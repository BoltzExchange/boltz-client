package main

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/boltzrpc/client"
	"google.golang.org/protobuf/proto"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/AlecAivazis/survey/v2"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
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

var listSwapsCommand = &cli.Command{
	Name:     "listswaps",
	Category: "Info",
	Usage:    "Lists all swaps",
	Action:   listSwaps,
	Flags: []cli.Flag{
		jsonFlag,
		&cli.StringFlag{
			Name:  "from",
			Usage: "Originating swap currency",
		},
		&cli.StringFlag{
			Name:  "to",
			Usage: "Destinaion swap currency",
		},
		&cli.BoolFlag{
			Name:  "pending",
			Usage: "Shorthand for --state pending",
		},
		&cli.StringFlag{
			Name:  "state",
			Usage: "Filter swaps by state",
		},
		&cli.BoolFlag{
			Name:  "auto",
			Usage: "Only show swaps created by autoswap",
		},
		&cli.BoolFlag{
			Name:  "manual",
			Usage: "Only show swaps created manually",
		},
	},
}

func getIncludeSwaps(ctx *cli.Context) boltzrpc.IncludeSwaps {
	if ctx.Bool("manual") {
		return boltzrpc.IncludeSwaps_MANUAL
	}
	if ctx.Bool("auto") {
		return boltzrpc.IncludeSwaps_AUTO
	}
	return boltzrpc.IncludeSwaps_ALL
}

var getStatsCommand = &cli.Command{
	Name:     "stats",
	Category: "Info",
	Usage:    "Get swap related stats",
	Action: func(ctx *cli.Context) error {
		client := getClient(ctx)
		response, err := client.GetStats(&boltzrpc.GetStatsRequest{Include: getIncludeSwaps(ctx)})
		if err != nil {
			return err
		}
		if ctx.Bool("json") {
			printJson(response)
		} else {
			printStats(response.Stats)
		}
		return nil
	},
	Flags: []cli.Flag{
		jsonFlag,
		&cli.BoolFlag{
			Name:  "auto",
			Usage: "Only show swaps created by autoswap",
		},
		&cli.BoolFlag{
			Name:  "manual",
			Usage: "Only show swaps created manually",
		},
	},
}

func listSwaps(ctx *cli.Context) error {
	client := getClient(ctx)
	request := &boltzrpc.ListSwapsRequest{
		Include: getIncludeSwaps(ctx),
	}
	if from := ctx.String("from"); from != "" {
		currency, err := parseCurrency(from)
		if err != nil {
			return err
		}
		request.From = &currency
	}
	if to := ctx.String("to"); to != "" {
		currency, err := parseCurrency(to)
		if err != nil {
			return err
		}
		request.To = &currency
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

		if len(list.Swaps) == 0 && len(list.ReverseSwaps) == 0 && len(list.ChainSwaps) == 0 {
			fmt.Println("No swaps found")
			return nil
		}

		if len(list.Swaps) > 0 {

			tbl := table.New("ID", "From", "To", "State", "Status", "Amount", "Boltz Fee", "Onchain Fee", "Created At")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, swap := range list.Swaps {
				tbl.AddRow(swap.Id, swap.Pair.From, swap.Pair.To, swap.State, swap.Status, swap.ExpectedAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt))
			}

			if _, err := yellowBold.Println("Swaps"); err != nil {
				return err
			}

			tbl.Print()
			fmt.Println()
		}

		if len(list.ReverseSwaps) > 0 {

			tbl := table.New("ID", "From", "To", "State", "Status", "Amount", "Boltz Fee", "Onchain Fee", "Created At")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, swap := range list.ReverseSwaps {
				tbl.AddRow(swap.Id, swap.Pair.From, swap.Pair.To, swap.State, swap.Status, swap.OnchainAmount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt))
			}

			if _, err := yellowBold.Println("Reverse Swaps"); err != nil {
				return err
			}
			tbl.Print()
			fmt.Println()
		}

		if len(list.ChainSwaps) > 0 {
			tbl := table.New("ID", "From", "To", "State", "Status", "Amount", "Boltz Fee", "Onchain Fee", "Created At")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, swap := range list.ChainSwaps {
				tbl.AddRow(swap.Id, swap.Pair.From, swap.Pair.To, swap.State, swap.Status, swap.FromData.Amount, optionalInt(swap.ServiceFee), optionalInt(swap.OnchainFee), parseDate(swap.CreatedAt))
			}

			if _, err := yellowBold.Println("Chain Swaps"); err != nil {
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
	Usage:     "Streams updates of a specific swap or of all swaps",
	ArgsUsage: "[id]",
	Action: func(ctx *cli.Context) error {
		return swapInfoStream(ctx, ctx.Args().First(), ctx.Bool("json"))
	},
	Flags: []cli.Flag{jsonFlag},
}

func swapInfoStream(ctx *cli.Context, id string, json bool) error {
	client := getClient(ctx)

	stream, err := client.GetSwapInfoStream(id)
	if err != nil {
		return err
	}

	isGlobal := id == ""

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
			var state boltzrpc.SwapState
			if info.Swap != nil {
				swap := info.Swap
				state = swap.State
				if isGlobal {
					yellowBold.Printf("Swap %s Status: %s\n", swap.Id, swap.Status)
				} else {
					yellowBold.Printf("Status: %s\n", swap.Status)
				}

				switch info.Swap.State {
				case boltzrpc.SwapState_ERROR:
					fmt.Printf("Error: %s\n", info.Swap.Error)
				case boltzrpc.SwapState_REFUNDED:
					fmt.Println("Swap was refunded")
				}

				status := boltz.ParseEvent(info.Swap.Status)
				switch status {
				case boltz.TransactionMempool:
					fmt.Printf("Transaction ID: %s\nAmount: %dsat\n", info.Swap.LockupTransactionId, info.Swap.ExpectedAmount)
				case boltz.InvoiceSet:
					fmt.Printf("Invoice: %s\n", info.Swap.Invoice)
				case boltz.TransactionClaimed:
					fmt.Printf("Paid %d sat onchain fee and %d sat boltz fee\n", *info.Swap.OnchainFee, *info.Swap.ServiceFee)
				}
			} else if info.ReverseSwap != nil {
				swap := info.ReverseSwap
				state = swap.State
				if isGlobal {
					yellowBold.Printf("Reverse Swap %s Status: %s\n", swap.Id, swap.Status)
				} else {
					yellowBold.Printf("Status: %s\n", swap.Status)
				}

				switch swap.State {
				case boltzrpc.SwapState_ERROR:
					fmt.Printf("Error: %s", swap.Error)
				}

				status := boltz.ParseEvent(swap.Status)
				switch status {
				case boltz.SwapCreated:
					if swap.ExternalPay {
						fmt.Printf("Invoice: %s\n", swap.Invoice)
					}
				case boltz.TransactionMempool:
					fmt.Printf("Lockup transaction ID: %s\n", swap.LockupTransactionId)
				case boltz.InvoiceSettled:
					fmt.Printf("Claim transaction ID: %s\n", swap.ClaimTransactionId)
					if swap.ExternalPay {
						fmt.Printf("Paid %d sat onchain fee and %d sat boltz fee\n", *swap.OnchainFee, *swap.ServiceFee)
					} else {
						fmt.Printf("Paid %d msat routing fee, %d sat onchain fee and %d sat boltz fee\n", *swap.RoutingFeeMsat, *swap.OnchainFee, *swap.ServiceFee)
					}
				}
			} else if info.ChainSwap != nil {
				swap := info.ChainSwap
				state = swap.State
				if isGlobal {
					yellowBold.Printf("Chain Swap %s Status: %s\n", swap.Id, swap.Status)
				} else {
					yellowBold.Printf("Status: %s\n", swap.Status)
				}

				switch swap.State {
				case boltzrpc.SwapState_ERROR:
					fmt.Printf("Error: %s\n", swap.Error)
				case boltzrpc.SwapState_REFUNDED:
					fmt.Println("Swap was refunded")
				}

				status := boltz.ParseEvent(swap.Status)
				switch status {
				case boltz.TransactionMempool:
					fmt.Printf("User transaction ID (%s): %s\nAmount: %dsat\n", swap.Pair.From, swap.FromData.GetLockupTransactionId(), swap.FromData.Amount)
				case boltz.TransactionServerMempoool:
					fmt.Printf("Server transaction ID (%s): %s\nAmount: %dsat\n", swap.Pair.To, swap.ToData.GetLockupTransactionId(), swap.ToData.Amount)
				case boltz.TransactionClaimed:
					fmt.Printf("Paid %d sat onchain fee and %d sat boltz fee\n", *swap.OnchainFee, *swap.ServiceFee)
				}
			}
			if state == boltzrpc.SwapState_SUCCESSFUL && !isGlobal {
				return nil
			}
			fmt.Println()
		}
	}

	return nil
}

var configDescription string = `View and edit configuration of autoswap.
By default, the whole config is shown, altough a certain key can be specified.
A new value for the key can also be provided.
The configuration file autoswap.toml is located inside the data directory of the daemon and can be edited manually too.`

var autoSwapCommands = &cli.Command{
	Name:    "autoswap",
	Aliases: []string{"auto"},
	Usage:   "Manage autoswap",
	Description: "Autoswap keeps your lightning node balanced by automatically executing swaps.\n" +
		"It regularly checks your nodes channels and creates swaps based on your configuration, which can be managed with the `config` command.\n" +
		"You can also configure autoswap without starting it and see what it would do with the `recommendations` command.\n" +
		"Once you are confident with the configuration, you can enable autoswap with the `enable` command.\n",
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
		},
		{
			Name:  "setup",
			Usage: "Setup autoswap interactively",
			Action: func(context *cli.Context) error {
				return autoSwapSetup(context, nil)
			},
			Subcommands: []*cli.Command{
				{
					Name:  "ln",
					Usage: "Setup lightning configuration interactively",
					Action: func(context *cli.Context) error {
						return autoSwapSetup(context, &lightning)
					},
				},
				{
					Name:  "chain",
					Usage: "Setup chain configuration interactively",
					Action: func(context *cli.Context) error {
						return autoSwapSetup(context, &chain)
					},
				},
			},
		},
		{
			Name:        "config",
			Usage:       "Manage configuration",
			Description: configDescription,
			Action: func(ctx *cli.Context) error {
				return autoSwapConfig(ctx, nil)
			},
			Flags: []cli.Flag{
				jsonFlag,
				&cli.BoolFlag{
					Name:  "reload",
					Usage: "Reloads the config from the filesystem before any action is taken. Use if you manually changed the configuration file",
				},
			},
			Subcommands: []*cli.Command{
				{
					Name:      "ln",
					Usage:     "Manage lightning configuration",
					ArgsUsage: "[key] [value]",
					Action: func(ctx *cli.Context) error {
						return autoSwapConfig(ctx, &lightning)
					},
					BashComplete: func(ctx *cli.Context) {
						var lastArg string

						if len(os.Args) > 2 {
							lastArg = os.Args[len(os.Args)-2]
						}

						if strings.HasPrefix(lastArg, "-") {
							cli.DefaultCompleteWithFlags(ctx.Command)(ctx)
						} else {
							config := &autoswaprpc.LightningConfig{}
							fields := config.ProtoReflect().Descriptor().Fields()
							for i := 0; i < fields.Len(); i++ {
								fmt.Println(fields.Get(i).JSONName())
							}
						}
					},
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "reset",
							Usage: "Resets to the default configuration",
						},
					},
				},
				{
					Name:      "chain",
					Usage:     "Manage chain configuration",
					ArgsUsage: "[key] [value]",
					Action: func(ctx *cli.Context) error {
						return autoSwapConfig(ctx, &chain)
					},
					BashComplete: func(ctx *cli.Context) {
						var lastArg string

						if len(os.Args) > 2 {
							lastArg = os.Args[len(os.Args)-2]
						}

						if strings.HasPrefix(lastArg, "-") {
							cli.DefaultCompleteWithFlags(ctx.Command)(ctx)
						} else {
							config := &autoswaprpc.ChainConfig{}
							fields := config.ProtoReflect().Descriptor().Fields()
							for i := 0; i < fields.Len(); i++ {
								fmt.Println(fields.Get(i).JSONName())
							}
						}
					},
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "reset",
							Usage: "Resets to the default configuration",
						},
					},
				},
			},
		},
		{
			Name:  "enable",
			Usage: "Enables autoswap",
			Subcommands: []*cli.Command{
				{
					Name: "ln",
					Action: func(ctx *cli.Context) error {
						return enableAutoSwap(ctx, true, &lightning)
					},
				},
				{
					Name: "chain",
					Action: func(ctx *cli.Context) error {
						return enableAutoSwap(ctx, true, &chain)
					},
				},
			},
		},
		{
			Name:  "disable",
			Usage: "Disables autoswap",
			Subcommands: []*cli.Command{
				{
					Name: "ln",
					Action: func(ctx *cli.Context) error {
						return disableAutoSwap(ctx, lightning)
					},
				},
				{
					Name: "chain",
					Action: func(ctx *cli.Context) error {
						return disableAutoSwap(ctx, chain)
					},
				},
			},
		},
	},
}

func listSwapRecommendations(ctx *cli.Context) error {
	client := getAutoSwapClient(ctx)
	list, err := client.GetRecommendations()

	if err != nil {
		return err
	}

	printJson(list)

	return nil
}

func printStatus(prefix string, status *autoswaprpc.Status) {
	prefix += ": "
	if status.Running {
		color.New(color.FgGreen, color.Bold).Println(prefix + "Running")
	} else if status.Error != nil {
		color.New(color.FgRed, color.Bold).Println(prefix + "Failed to start")
		fmt.Println("Error: " + status.GetError())
	} else {
		color.New(color.FgYellow, color.Bold).Println(prefix + "Disabled")
	}
	if status.Description != "" {
		fmt.Printf("%s\n", status.Description)
	}
	if status.Budget != nil {
		budget := status.Budget
		yellowBold.Println("\nBudget")
		fmt.Printf(" - From %s until %s\n", parseDate(budget.StartDate), parseDate(budget.EndDate))
		fmt.Println(" - Total: " + utils.Satoshis(budget.Total))
		fmt.Println(" - Remaining: " + utils.Satoshis(budget.Remaining))

		printStats(budget.Stats)
	}
}

func printStats(stats *boltzrpc.SwapStats) {
	yellowBold.Println("Stats")
	fmt.Printf(" - Successfull Swaps: %d\n", stats.SuccessCount)
	fmt.Println(" - Amount: " + utils.Satoshis(stats.TotalAmount))
	fmt.Println(" - Fees: " + utils.Satoshis(stats.TotalFees))
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
		if response.Lightning != nil {
			printStatus("LN", response.Lightning)
			if response.Chain != nil {
				fmt.Println()
			}
		}
		if response.Chain != nil {
			printStatus("Chain", response.Chain)
		}
	}

	return nil
}

func printConfig(ctx *cli.Context, autoSwapType *client.AutoSwapType, key string, asJson, hideZero bool) error {
	client := getAutoSwapClient(ctx)

	var message proto.Message
	var err error

	if autoSwapType == nil {
		message, err = client.GetConfig()
	} else if *autoSwapType == lightning {
		message, err = client.GetLightningConfig()
	} else {
		message, err = client.GetChainConfig()
	}
	if err != nil {
		return err
	}

	marshal := protojson.MarshalOptions{EmitUnpopulated: !hideZero, Indent: "   "}
	marshalled, err := marshal.Marshal(message)
	if err != nil {
		return err
	}

	if asJson {
		fmt.Println(string(marshalled))
	} else {
		var configJson map[string]any
		if err := json.Unmarshal(marshalled, &configJson); err != nil {
			return err
		}
		if key != "" {
			fmt.Println(configJson[key])
		} else {
			var pretty bytes.Buffer
			if err := toml.NewEncoder(&pretty).Encode(configJson); err != nil {
				return err
			}
			fmt.Print(pretty.String())
		}
	}
	return nil
}

func autoSwapSetup(ctx *cli.Context, swapper *client.AutoSwapType) error {
	if swapper == nil || *swapper == lightning {
		if err := autoSwapLightningSetup(ctx); err != nil {
			return err
		}
	}
	if (swapper != nil && *swapper == chain) || (swapper == nil && prompt("Do you want to setup chain swaps as well?")) {
		if err := autoSwapChainSetup(ctx); err != nil {
			return err
		}
	}
	return enableAutoSwap(ctx, false, swapper)
}

func autoSwapLightningSetup(ctx *cli.Context) error {
	autoSwap := getAutoSwapClient(ctx)

	_, err := autoSwap.GetConfig()
	if err == nil {
		if !prompt("You already have an autoswap configuration. Do you want to reset it?") {
			return nil
		}
	}
	entireConfig, err := autoSwap.ResetConfig(client.LnAutoSwap)
	if err != nil {
		return err
	}
	config := entireConfig.Lightning[0]

	prompt := &survey.Select{
		Message: "Which type of swaps should be executed?",
		Options: []string{"reverse", "normal", "both"},
		Description: func(value string, index int) string {
			switch value {
			case "reverse":
				return "keeps your inbound balance above set threshold, supports read-only wallet"
			case "normal":
				return "keeps your outbound balance above set threshold"
			case "both":
				return "maintain a balanced channel between two thresholds"
			}
			return ""
		},
	}
	if err := survey.AskOne(prompt, &config.SwapType); err != nil {
		return err
	}
	allowReverse := true
	allowNormal := true
	if config.SwapType == "both" {
		config.SwapType = ""
	} else if config.SwapType == "reverse" {
		allowNormal = false
		config.OutboundBalancePercent = 0
	} else if config.SwapType == "normal" {
		allowReverse = false
		config.InboundBalancePercent = 0
	}

	readonly := config.SwapType == "reverse"
	wallet, err := askForWallet(ctx, "Select wallet which should be used for swaps", nil, readonly)
	if err != nil {
		return err
	}
	config.Wallet = wallet.Name
	config.Currency = wallet.Currency
	if allowNormal && wallet.Balance.GetTotal() == 0 {
		fmt.Println("Warning: Your selected wallet has no balance. Autoswap will not be able to execute normal swaps.")
	}

	var balanceType string
	prompt = &survey.Select{
		Message: "How do you want to specify balance values?",
		Options: []string{"percentage", "sats"},
	}
	if err := survey.AskOne(prompt, &balanceType); err != nil {
		return err
	}

	qs := []*survey.Question{}
	if balanceType == "sats" {
		if allowNormal {
			qs = append(qs, &survey.Question{
				Name:     "outboundBalance",
				Prompt:   &survey.Input{Message: "What is the minimum amount of sats you want to keep as your outbound balance?"},
				Validate: survey.ComposeValidators(survey.Required, uintValidator),
			})
		}
		if allowReverse {
			qs = append(qs, &survey.Question{
				Name:     "inboundBalance",
				Prompt:   &survey.Input{Message: "What is the minimum amount of sats you want to keep as your inbound balance?"},
				Validate: survey.ComposeValidators(survey.Required, uintValidator),
			})
		}
	} else {
		if allowNormal {
			qs = append(qs, &survey.Question{
				Name: "outboundBalancePercent",
				Prompt: &survey.Input{Message: "What is the minimum percentage of total capacity you want to keep as your outbound balance?",
					Default: fmt.Sprint(config.OutboundBalancePercent),
				},
				Validate: survey.ComposeValidators(survey.Required, percentValidator),
			})
		}
		if allowReverse {
			qs = append(qs, &survey.Question{
				Name: "inboundBalancePercent",
				Prompt: &survey.Input{Message: "What is the minimum percentage of total capacity you want to keep as your inbound balance?",
					Default: fmt.Sprint(config.InboundBalancePercent),
				},
				Validate: survey.ComposeValidators(survey.Required, percentValidator),
			})
		}
	}

	qs = append(qs, askBudget(config.MaxFeePercent, config.BudgetInterval, config.Budget)...)

	if err := survey.Ask(qs, config); err != nil {
		return err
	}

	config.BudgetInterval *= 24 * uint64(time.Hour.Seconds())

	_, err = autoSwap.UpdateLightningConfig(&autoswaprpc.UpdateLightningConfigRequest{Config: config})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Config was saved successfully!")
	if config.OutboundBalance == 0 && config.OutboundBalancePercent == 0 {
		fmt.Println("Autoswap will target 100% inbound balance when executing swaps.")
	} else if config.InboundBalance == 0 && config.InboundBalancePercent == 0 {
		fmt.Println("Autoswap will target 100% outbound balance when executing swaps.")
	} else {
		fmt.Println("Autoswap will target the middle between your two thresholds when executing swaps.")
	}
	return nil
}

func askForWallet(ctx *cli.Context, message string, currency *boltzrpc.Currency, allowReadonly bool) (result *boltzrpc.Wallet, err error) {
	client := getClient(ctx)

	wallets, err := client.GetWallets(currency, allowReadonly)
	if err != nil {
		return nil, err
	}

	createNew := "Create New"
	importExisting := "Import Existing"

	var walletsByName = make(map[string]*boltzrpc.Wallet)
	prompt := &survey.Select{Message: message, Description: func(value string, index int) string {
		wallet, ok := walletsByName[value]
		if ok {
			return fmt.Sprint(wallet.Currency)
		}
		return ""
	}}

	for _, wallet := range wallets.Wallets {
		prompt.Options = append(prompt.Options, wallet.Name)
		if wallet.Currency == boltzrpc.Currency_LBTC {
			prompt.Default = wallet.Name
		}
		walletsByName[wallet.Name] = wallet
	}
	prompt.Options = append(prompt.Options, createNew)
	prompt.Options = append(prompt.Options, importExisting)

	var choice string
	if err := survey.AskOne(prompt, &choice); err != nil {
		return nil, err
	}

	if choice != createNew && choice != importExisting {
		result = walletsByName[choice]
	} else {
		info := &boltzrpc.WalletParams{}
		if currency == nil {
			prompt := &survey.Select{
				Message: "Select the wallet currency",
				Options: []string{"LBTC", "BTC"},
				Default: "LBTC",
			}
			var currency string
			if err = survey.AskOne(prompt, &currency); err != nil {
				return
			}
			info.Currency, err = parseCurrency(currency)
			if err != nil {
				return nil, err
			}
		} else {
			info.Currency = *currency
		}
		input := &survey.Input{
			Message: "Enter a name for the new wallet",
			Default: "autoswap",
		}
		err = survey.AskOne(input, &info.Name, survey.WithValidator(func(ans interface{}) error {
			return checkWalletName(ctx, ans.(string))
		}))

		if choice == createNew {
			result, err = createWallet(ctx, info)
		} else if choice == importExisting {
			result, err = importWallet(ctx, info, allowReadonly)
		}
	}
	return result, err
}

func autoSwapChainSetup(ctx *cli.Context) error {
	autoSwap := getAutoSwapClient(ctx)
	client := getClient(ctx)

	_, err := autoSwap.GetChainConfig()
	if err == nil {
		if !prompt("You already have an autoswap configuration. Do you want to reset it?") {
			return nil
		}
	}
	config := &autoswaprpc.ChainConfig{}

	fromWallet, err := askForWallet(ctx, "Select source wallet", nil, false)
	if err != nil {
		return err
	}
	config.FromWallet = fromWallet.Name

	toCurrency := boltzrpc.Currency_BTC
	if fromWallet.Currency == boltzrpc.Currency_BTC {
		toCurrency = boltzrpc.Currency_LBTC
	}

	toWallet, err := askForWallet(ctx, "Select target wallet", &toCurrency, true)
	if err != nil {
		return err
	}
	config.ToWallet = toWallet.Name

	pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, &boltzrpc.Pair{
		From: fromWallet.Currency,
		To:   toWallet.Currency,
	})
	if err != nil {
		return err
	}

	questions := []*survey.Question{
		{
			Name: "MaxBalance",
			Prompt: &survey.Input{
				Message: "What is the maximum amount of sats you want to accumulate before a chain swap is started?",
			},
			Validate: survey.ComposeValidators(survey.Required, func(ans interface{}) error {
				num, err := strconv.ParseUint(ans.(string), 10, 64)
				if err != nil {
					return errors.New("not a valid number")
				}

				// TODO: remove buffer once proper sweep is implemented
				limit := pairInfo.Limits.Minimal + 10000
				if num < limit {
					return fmt.Errorf("must be at least %d", limit)
				}

				return nil
			}),
		},
	}
	questions = append(questions, askBudget(1, uint64((time.Hour*24*7).Seconds()), 100000)...)

	if err := survey.Ask(questions, config); err != nil {
		return err
	}

	config.BudgetInterval *= 24 * uint64(time.Hour.Seconds())
	_, err = autoSwap.UpdateChainConfig(&autoswaprpc.UpdateChainConfigRequest{Config: config})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Config was saved successfully!")
	return nil
}

var lightning = client.LnAutoSwap
var chain = client.ChainAutoSwap

func autoSwapConfig(ctx *cli.Context, swapper *client.AutoSwapType) (err error) {
	client := getAutoSwapClient(ctx)

	if ctx.Bool("reload") {
		if _, err := client.ReloadConfig(); err != nil {
			return err
		}
	}

	if ctx.Bool("reset") && swapper != nil {
		if _, err := client.ResetConfig(*swapper); err != nil {
			return err
		}
	}

	var key string
	if swapper != nil {
		key = ctx.Args().First()
		if ctx.NArg() == 2 {
			args := ctx.Args()
			if _, err := client.SetConfigValue(*swapper, args.Get(0), args.Get(1)); err != nil {
				return err
			}
		}
	}

	return printConfig(ctx, swapper, key, ctx.Bool("json"), false)
}

func enableAutoSwap(ctx *cli.Context, showConfig bool, swapper *client.AutoSwapType) error {
	client := getAutoSwapClient(ctx)

	if showConfig {
		fmt.Println("Enabling autoswap with the following config:")
		fmt.Println()
		if err := printConfig(ctx, swapper, "", false, true); err != nil {
			return err
		}
	}

	recommendations, err := client.GetRecommendations()
	if err != nil {
		return err
	}

	if len(recommendations.Lightning) > 0 || len(recommendations.Chain) > 0 {
		fmt.Println("Based on above config the following swaps will be performed:")
		printJson(recommendations)
	}

	fmt.Println()
	if !prompt("Do you want to enable autoswap now?") {
		return nil
	}

	if swapper == nil || *swapper == lightning {
		if _, err := client.SetConfigValue(lightning, "enabled", true); err != nil {
			return err
		}
	}
	if swapper == nil || *swapper == chain {
		if _, err := client.SetConfigValue(chain, "enabled", true); err != nil {
			return err
		}
	}
	return autoSwapStatus(ctx)
}

func disableAutoSwap(ctx *cli.Context, autoSwapType client.AutoSwapType) error {
	client := getAutoSwapClient(ctx)
	_, err := client.SetConfigValue(autoSwapType, "enabled", false)
	return err
}

var createSwapCommand = &cli.Command{
	Name:      "createswap",
	Category:  "Swaps",
	Usage:     "Create a new chain-to-lightning swap",
	ArgsUsage: "currency [amount]",
	Description: "Creates a new swap (e.g. BTC -> Lightning) specifying the amount in satoshis.\n" +
		"If the --any-amount flag is specified, any amount within the displayed limits can be paid to the lockup address.\n" +
		"\nExamples\n" +
		"Create a swap from mainchain for 100000 satoshis that will be immediately paid by the clients wallet:\n" +
		"> boltzcli createswap btc 100000\n" +
		"Create a swap from liquid for any amount of satoshis that can be paid manually:\n" +
		"> boltzcli createswap --any-amount lbtc\n" +
		"Create a swap from mainchain using an existing invoice:\n" +
		"> boltzcli createswap --invoice lnbcrt1m1pja7adjpp59xdpx33l80wf8rsmqkwjyccdzccsedp9qgy9agf0k8m5g8ttrnzsdq8w3jhxaqcqp5xqzjcsp528qsd7mec4jml9zy302tmr0t995fe9uu80qwgg4zegerh3weyn8s9qyyssqpwecwyvndxh9ar0crgpe4crr93pr4g682u5sstzfk6e0g73s6urxm320j5yuamlszxnk5fzzrtx2hkxw8ehy6kntrx4cr4kcq6zc4uqqy7tcst btc",
	Action: requireNArgs(1, createSwap),
	Flags: []cli.Flag{
		jsonFlag,
		&cli.StringFlag{
			Name:  "from-wallet",
			Usage: "Internal wallet to fund the swap from",
		},
		&cli.BoolFlag{
			Name:  "external-pay",
			Usage: "Whether the swap should be paid externally",
		},
		&cli.BoolFlag{
			Name:  "any-amount",
			Usage: "Allow any amount within the limits to be paid to the lockup address.",
		},
		&cli.StringFlag{
			Name:  "refund",
			Usage: "Address to refund to in case the swap fails",
		},
		&cli.StringFlag{
			Name:  "invoice",
			Usage: "Invoice which should be paid",
		},
	},
}

func createSwap(ctx *cli.Context) error {
	client := getClient(ctx)

	currency, err := parseCurrency(ctx.Args().First())
	if err != nil {
		return err
	}

	pair := &boltzrpc.Pair{
		From: currency,
		To:   boltzrpc.Currency_BTC,
	}

	json := ctx.Bool("json")
	invoice := ctx.String("invoice")
	refundAddress := ctx.String("refund")
	externalPay := ctx.Bool("external-pay")
	var amount uint64
	if rawAmount := ctx.Args().Get(1); rawAmount != "" {
		amount = parseUint64(rawAmount, "amount")
	} else if ctx.Bool("any-amount") {
		externalPay = true
	} else if invoice == "" {
		return cli.ShowSubcommandHelp(ctx)
	}

	pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_SUBMARINE, pair)
	if err != nil {
		return err
	}

	if !json {
		printFees(pairInfo.Fees, amount)

		if !prompt("Do you want to continue?") {
			return nil
		}
	}

	walletId, err := getWalletId(ctx, ctx.String("from-wallet"))
	if err != nil {
		return err
	}
	swap, err := client.CreateSwap(&boltzrpc.CreateSwapRequest{
		Amount:           amount,
		Pair:             pair,
		RefundAddress:    &refundAddress,
		SendFromInternal: !externalPay,
		WalletId:         walletId,
		Invoice:          &invoice,
	})
	if err != nil {
		return err
	}

	if json {
		printJson(swap)
		return nil
	}

	if externalPay {
		printDeposit(amount, swap.Address, swap.TimeoutHours, uint64(swap.TimeoutBlockHeight), pairInfo.Limits)
		fmt.Println()
	}

	fmt.Println("Swap ID:", swap.Id)

	return swapInfoStream(ctx, swap.Id, false)
}

var createChainSwapCommand = &cli.Command{
	Name:      "createchainswap",
	Category:  "Swaps",
	Usage:     "Create a new chain-to-chain swap",
	ArgsUsage: "amount",
	Description: "Creates a new chain swap (e.g. BTC -> L-BTC) specifying the amount in satoshis.\n" +
		"\nExamples" +
		"\nCreate a chain swap for 100000 satoshis from the L-BTC wallet 'autoswap' to the BTC wallet 'cold':" +
		"\n> boltzcli createchainswap --from-wallet autoswap --to-wallet cold 100000" +
		"\nCreate a chain swap for 100000 satoshis from the L-BTC wallet 'autoswap' to a BTC address:" +
		"\n> boltzcli createchainswap --from-wallet autoswap --to-address bcrt1q0akydfs98pjmqqplz0kvaa5hphg237vcvgaez2 100000" +
		"\nCreate a chain swap for 100000 satoshis from BTC to the L-BTC wallet 'autoswap' which has to be paid manually:" +
		"\n> boltzcli createchainswap --from-external LBTC --to-wallet autoswap 100000",
	Action: createChainSwap,
	Flags: []cli.Flag{
		jsonFlag,
		&cli.BoolFlag{
			Name:  "no-zero-conf",
			Usage: "Disable zero-conf for this swap",
		},
		&cli.StringFlag{
			Name:  "from-external",
			Usage: "Currency to swap from; swap has to be funded externally",
		},
		&cli.StringFlag{
			Name:  "from-wallet",
			Usage: "Internal wallet to fund the swap from",
		},
		&cli.StringFlag{
			Name:  "to-wallet",
			Usage: "Internal wallet to swap to",
		},
		&cli.StringFlag{
			Name:  "to-address",
			Usage: "External address to swap to",
		},
		&cli.StringFlag{
			Name:  "refund-address",
			Usage: "Address to refund to in case the swap fails",
		},
	},
}

func createChainSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	var amount uint64
	if ctx.Args().First() != "" {
		amount = parseUint64(ctx.Args().First(), "amount")
	} else if !ctx.Bool("any-amount") {
		return cli.ShowSubcommandHelp(ctx)
	}

	pair := &boltzrpc.Pair{}
	var err error
	if from := ctx.String("from-external"); from != "" {
		pair.From, err = parseCurrency(from)
		if err != nil {
			return err
		}
	}

	acceptZeroConf := !ctx.Bool("no-zero-conf")
	fromWallet := ctx.String("from-wallet")
	externalPay := fromWallet == ""
	request := &boltzrpc.CreateChainSwapRequest{
		Amount:         amount,
		Pair:           pair,
		ExternalPay:    &externalPay,
		AcceptZeroConf: &acceptZeroConf,
	}

	info, err := client.GetInfo()
	if err != nil {
		return err
	}
	network, _ := boltz.ParseChain(info.Network)

	if toAddress := ctx.String("to-address"); toAddress != "" {
		to, err := boltz.GetAddressCurrency(network, toAddress)
		if err != nil {
			return err
		}
		pair.To = utils.SerializeCurrency(to)
		request.ToAddress = &toAddress
	}

	if refundAddress := ctx.String("refund-address"); refundAddress != "" {
		from, err := boltz.GetAddressCurrency(network, refundAddress)
		if err != nil {
			return err
		}
		pair.From = utils.SerializeCurrency(from)
		request.RefundAddress = &refundAddress
	}

	if fromWallet != "" {
		wallet, err := client.GetWallet(fromWallet)
		if err != nil {
			return err
		}
		pair.From = wallet.Currency
		request.FromWalletId = &wallet.Id
	}

	if toWallet := ctx.String("to-wallet"); toWallet != "" {
		wallet, err := client.GetWallet(toWallet)
		if err != nil {
			return err
		}
		pair.To = wallet.Currency
		request.ToWalletId = &wallet.Id
	}

	request.FromWalletId, err = getWalletId(ctx, ctx.String("from-wallet"))
	if err != nil {
		return err
	}

	pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_CHAIN, pair)
	if err != nil {
		return err
	}

	json := ctx.Bool("json")
	if !json {
		printFees(pairInfo.Fees, amount)
		if !prompt("Do you want to continue?") {
			return nil
		}
	}

	swap, err := client.CreateChainSwap(request)
	if err != nil {
		return err
	}

	if json {
		printJson(swap)
		return nil
	}

	if externalPay {
		height := info.BlockHeights.Btc
		if pair.From == boltzrpc.Currency_LBTC {
			height = info.BlockHeights.GetLiquid()
		}
		timeout := swap.FromData.TimeoutBlockHeight
		timeoutHours := boltz.BlocksToHours(timeout-height, utils.ParseCurrency(&pair.From))
		printDeposit(amount, swap.FromData.LockupAddress, float32(timeoutHours), uint64(timeout), pairInfo.Limits)
		fmt.Println()
	}

	fmt.Println("Swap ID:", swap.Id)

	return swapInfoStream(ctx, swap.Id, false)
}

var refundSwapCommand = &cli.Command{
	Name:      "refundswap",
	Category:  "Swaps",
	Usage:     "Refund a chain-to-x swap manually to an onchain address or internal wallet",
	ArgsUsage: "id address|wallet",
	Action:    requireNArgs(2, refundSwap),
}

func refundSwap(ctx *cli.Context) error {
	client := getClient(ctx)
	id := ctx.Args().First()
	destination := ctx.Args().Get(1)
	request := &boltzrpc.RefundSwapRequest{Id: id}
	walletId, err := getWalletId(ctx, destination)
	if err == nil {
		request.Destination = &boltzrpc.RefundSwapRequest_WalletId{WalletId: *walletId}
	} else {
		request.Destination = &boltzrpc.RefundSwapRequest_Address{Address: destination}
	}
	swap, err := client.RefundSwap(request)
	if err != nil {
		return err
	}
	tx := swap.ChainSwap.GetFromData().GetTransactionId()
	if tx == "" {
		tx = swap.Swap.GetRefundTransactionId()
	}
	fmt.Println("Refund transaction ID: " + tx)
	return nil
}

var claimSwapsCommand = &cli.Command{
	Name:      "claimswaps",
	Category:  "Swaps",
	Usage:     "Claim x-to-chain swaps manually",
	ArgsUsage: "addresss|wallet ids...",
	Action:    requireNArgs(2, claimSwaps),
}

func claimSwaps(ctx *cli.Context) error {
	client := getClient(ctx)
	request := &boltzrpc.ClaimSwapsRequest{SwapIds: ctx.Args().Tail()}
	address := ctx.Args().First()
	walletId, err := getWalletId(ctx, address)
	if err == nil {
		request.Destination = &boltzrpc.ClaimSwapsRequest_WalletId{WalletId: *walletId}
	} else {
		request.Destination = &boltzrpc.ClaimSwapsRequest_Address{Address: address}
	}
	response, err := client.ClaimSwaps(request)
	if err != nil {
		return err
	}
	fmt.Println("Claim transaction ID: " + response.TransactionId)
	return nil
}

var createReverseSwapCommand = &cli.Command{
	Name:      "createreverseswap",
	Category:  "Swaps",
	Usage:     "Create a new lightning-to-chain swap",
	ArgsUsage: "currency amount [address]",
	Description: "Creates a new reverse swap (e.g. Lightning -> BTC) for `amount` satoshis, optionally specifying the destination address.\n" +
		"If no address is specified, it will be generated by the clients wallet.\n" +
		"\nExamples\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the clients btc wallet:\n" +
		"> boltzcli createreverseswap btc 100000\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the specified btc address:\n" +
		"> boltzcli createreverseswap btc 100000 bcrt1qkp70ncua3dqp6syqu24jw5mnpf3gdxqrm3gn2a\n" +
		"create a reverse swap for 100000 satoshis that will be sent to the clients liquid wallet:\n" +
		"> boltzcli createreverseswap lbtc 100000",
	Action: requireNArgs(2, createReverseSwap),
	Flags: []cli.Flag{
		jsonFlag,
		&cli.StringFlag{
			Name:  "to-wallet",
			Usage: "Internal wallet to swap to",
		},
		&cli.BoolFlag{
			Name:  "no-zero-conf",
			Usage: "Disable zero-conf for this swap",
		},
		&cli.BoolFlag{
			Name:  "external-pay",
			Usage: "Do not automatically pay the swap from the connected lightning node",
		},
		&cli.StringFlag{
			Name:  "description",
			Usage: "Description of the swap invoice",
		},
		&cli.StringSliceFlag{
			Name: "chan-id",
		},
	},
}

func parseCurrency(currency string) (boltzrpc.Currency, error) {
	if currency == "" {
		return boltzrpc.Currency_BTC, errors.New("currency is required, allowed values: BTC, LBTC")
	}
	upper := strings.ToUpper(currency)
	if upper == "LBTC" || upper == "L-BTC" || upper == "LIQUID" {
		return boltzrpc.Currency_LBTC, nil
	} else if upper == "BTC" {
		return boltzrpc.Currency_BTC, nil
	}
	return boltzrpc.Currency_BTC, fmt.Errorf("invalid currency: %s, allowed values: BTC, LBTC", currency)
}

func getWalletId(ctx *cli.Context, name string) (*uint64, error) {
	if name != "" {
		client := getClient(ctx)
		wallet, err := client.GetWallet(name)
		if err != nil {
			return nil, err
		}
		return &wallet.Id, nil
	}
	return nil, nil
}

func createReverseSwap(ctx *cli.Context) error {
	client := getClient(ctx)

	currency, err := parseCurrency(ctx.Args().First())
	if err != nil {
		return err
	}
	pair := &boltzrpc.Pair{
		From: boltzrpc.Currency_BTC,
		To:   currency,
	}

	address := ctx.Args().Get(2)
	amount := parseUint64(ctx.Args().Get(1), "amount")
	description := ctx.String("description")
	json := ctx.Bool("json")

	if !json {
		pairInfo, err := client.GetPairInfo(boltzrpc.SwapType_REVERSE, pair)
		if err != nil {
			return err
		}

		printFees(pairInfo.Fees, amount)

		if !prompt("Do you want to continue?") {
			return nil
		}
	}

	returnImmediately := true
	walletId, err := getWalletId(ctx, ctx.String("to-wallet"))
	if err != nil {
		return err
	}
	request := &boltzrpc.CreateReverseSwapRequest{
		Address:           address,
		Amount:            amount,
		AcceptZeroConf:    !ctx.Bool("no-zero-conf"),
		Pair:              pair,
		WalletId:          walletId,
		ChanIds:           ctx.StringSlice("chan-id"),
		ReturnImmediately: &returnImmediately,
		Description:       &description,
	}
	if externalPay := ctx.Bool("external-pay"); externalPay {
		request.ExternalPay = &externalPay
	}

	response, err := client.CreateReverseSwap(request)
	if err != nil {
		return err
	}

	if json {
		printJson(response)
	} else {
		fmt.Println("Swap ID:", response.Id)
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
			Description: "Creates a new wallet for the specified currency and unique name.\n" +
				"Currency has to be BTC or LBTC (case insensitive).",
			Action: requireNArgs(2, func(ctx *cli.Context) error {
				info, err := walletParams(ctx)
				if err != nil {
					return err
				}
				_, err = createWallet(ctx, info)
				return err
			}),
		},
		{
			Name:      "import",
			Usage:     "Imports an existing wallet",
			ArgsUsage: "name currency",
			Description: "Imports an existing wallet for the specified currency with an unique name.\n" +
				"You can either choose to import a full mnemonic to give the daemon full control over the wallet or import a readonly wallet using a xpub or core descriptor.\n" +
				"Currency has to be BTC ot LBTC (case insensitive).",
			Action: requireNArgs(2, func(ctx *cli.Context) error {
				info, err := walletParams(ctx)
				if err != nil {
					return err
				}
				_, err = importWallet(ctx, info, true)
				return err
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
			Name:        "subaccount",
			Usage:       "Select the subaccount for a wallet",
			Description: "Select the subaccount for a wallet. Not possible for readonly wallets.",
			ArgsUsage:   "name",
			Action: requireNArgs(1, func(ctx *cli.Context) error {
				walletId, err := getWalletId(ctx, ctx.Args().First())
				if err != nil {
					return err
				}
				return selectSubaccount(ctx, *walletId)
			}),
		},
		{
			Name:      "remove",
			Usage:     "Remove a wallet",
			ArgsUsage: "name",
			Action:    requireNArgs(1, removeWallet),
		},
		{
			Name:      "send",
			Usage:     "Send from a wallet",
			ArgsUsage: "name destination amount",
			Flags: []cli.Flag{
				&cli.Float64Flag{
					Name: "sat-per-vbyte",
				},
			},
			Action: requireNArgs(3, walletSend),
		},
		{
			Name:      "receive",
			Usage:     "Get a new address for a wallet",
			ArgsUsage: "name",
			Action:    requireNArgs(1, walletReceive),
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
	Usage: "Change password for integrated wallets",
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
		if password == nil {
			fmt.Println("No password set")
		} else {
			fmt.Println("Correct")
		}
		return nil
	},
}

func askPassword(ctx *cli.Context, askNew bool) (*string, error) {
	client := getClient(ctx)
	hasPassword, err := client.HasPassword()
	if err != nil {
		return nil, err
	}
	if !hasPassword {
		if askNew {
			if !prompt("Do you want to provide a wallet password to encrypt your wallet, which will be required on startup?") {
				return nil, nil
			}
		} else {
			return nil, nil
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
		return nil, err
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
			return nil, err
		}
	}
	return &password, nil
}

func askBudget(defaultMaxFeePercent float32, defaultDuration, defaultBudget uint64) []*survey.Question {
	defaultDurationDays := fmt.Sprint(float64(defaultDuration) / (24 * time.Hour).Seconds())
	return []*survey.Question{
		{
			Name: "maxFeePercent",
			Prompt: &survey.Input{
				Message: "What is the maximum percentage of the total swap amount you are willing to pay as fees?",
				Default: fmt.Sprint(defaultMaxFeePercent),
			},
			Validate: survey.ComposeValidators(survey.Required, percentValidator),
		},
		{
			Name: "BudgetInterval",
			Prompt: &survey.Input{
				Message: "In which interval should the fee budget of the auto swapper be reset? (days)",
				Default: defaultDurationDays,
			},
			Validate: survey.ComposeValidators(survey.Required, uintValidator),
		},
		{
			Name: "Budget",
			Prompt: &survey.Input{
				Message: "How many sats do you want to spend max on fees per budget interval?",
				Default: fmt.Sprint(defaultBudget),
			},
			Validate: survey.ComposeValidators(survey.Required, uintValidator),
		},
	}
}

func printSubaccount(info *boltzrpc.Subaccount) {
	fmt.Printf("Subaccount: %d (%s)\n", info.Pointer, liquidAccountType(info.Type))
	balance := info.Balance

	fmt.Printf("Balance: %s (%s unconfirmed)\n", utils.Satoshis(balance.Total), utils.Satoshis(balance.Unconfirmed))
}

func walletParams(ctx *cli.Context) (*boltzrpc.WalletParams, error) {
	currency, err := parseCurrency(ctx.Args().Get(1))
	if err != nil {
		return nil, err
	}
	return &boltzrpc.WalletParams{
		Name:     ctx.Args().Get(0),
		Currency: currency,
	}, nil
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

func importWallet(ctx *cli.Context, params *boltzrpc.WalletParams, readonly bool) (wallet *boltzrpc.Wallet, err error) {
	client := getClient(ctx)
	if err := checkWalletName(ctx, params.Name); err != nil {
		return nil, err
	}

	mnemonic := ""
	importType := "mnemonic"
	if readonly {
		prompt := &survey.Select{
			Message: "Which import type do you want to use?",
			Options: []string{"mnemonic", "core descriptor"},
			Default: "mnemonic",
		}
		if params.Currency == boltzrpc.Currency_BTC {
			prompt.Options = append(prompt.Options, "xpub")
		}
		if err := survey.AskOne(prompt, &importType); err != nil {
			return nil, err
		}
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please type your %s", importType),
	}
	if err := survey.AskOne(prompt, &mnemonic, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	credentials := &boltzrpc.WalletCredentials{}
	if importType == "mnemonic" {
		credentials.Mnemonic = &mnemonic
	} else if importType == "xpub" {
		credentials.Xpub = &mnemonic
	} else if importType == "core descriptor" {
		credentials.CoreDescriptor = &mnemonic
	}

	params.Password, err = askPassword(ctx, true)
	if err != nil {
		return nil, err
	}

	wallet, err = client.ImportWallet(params, credentials)
	if err != nil {
		return nil, err
	}

	fmt.Println("Successfully imported wallet!")

	if !wallet.Readonly {
		return wallet, selectSubaccount(ctx, wallet.Id)
	}
	return wallet, nil
}

func selectSubaccount(ctx *cli.Context, walletId uint64) error {
	client := getClient(ctx)

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Fetching subaccounts..."
	s.Start()

	subaccounts, err := client.GetSubaccounts(walletId)
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

	response, err := client.SetSubaccount(walletId, subaccount)
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
	name := ctx.Args().First()
	if name == "" {
		return errors.New("wallet name can not be empty")
	}
	walletId, err := getWalletId(ctx, name)
	if err != nil {
		return err
	}
	_, err = client.RemoveWallet(*walletId)
	if err != nil {
		return err
	}
	fmt.Println("Wallet removed")
	return nil
}

func walletSend(ctx *cli.Context) error {
	client := getClient(ctx)
	walletId, err := getWalletId(ctx, ctx.Args().First())
	if err != nil {
		return err
	}
	address := ctx.Args().Get(1)
	amount := ctx.Args().Get(2)

	request := &boltzrpc.WalletSendRequest{
		Id:      *walletId,
		Address: address,
		Amount:  parseUint64(amount, "amount"),
	}

	if satPerVbyte := ctx.Float64("sat-per-vbyte"); satPerVbyte != 0 {
		request.SatPerVbyte = &satPerVbyte
	}

	txId, err := client.WalletSend(request)
	if err != nil {
		return err
	}
	fmt.Println("Transaction ID:", txId)
	return nil
}

func walletReceive(ctx *cli.Context) error {
	client := getClient(ctx)
	walletId, err := getWalletId(ctx, ctx.Args().First())
	if err != nil {
		return err
	}
	address, err := client.WalletReceive(*walletId)
	if err != nil {
		return err
	}
	fmt.Println("Address:", address)
	return nil
}

func createWallet(ctx *cli.Context, params *boltzrpc.WalletParams) (wallet *boltzrpc.Wallet, err error) {
	client := getClient(ctx)

	if err := checkWalletName(ctx, params.Name); err != nil {
		return nil, err
	}

	params.Password, err = askPassword(ctx, true)
	if err != nil {
		return nil, err
	}

	credentials, err := client.CreateWallet(params)
	if err != nil {
		return nil, err
	}
	fmt.Println("New wallet created!")
	fmt.Println()
	fmt.Println("Mnemonic:\n" + credentials.Mnemonic)
	fmt.Println()
	fmt.Println("We highly recommend to import the mnemonic shown above into an external wallet like Blockstream Green (https://blockstream.com/green). " +
		"This serves as backup and allows you to view transactions and control your funds.")
	return credentials.Wallet, nil
}

func showCredentials(ctx *cli.Context) error {
	client := getClient(ctx)
	if prompt("Make sure no one can see your screen. Do you want to continue?") {
		password, err := askPassword(ctx, false)
		if err != nil {
			return err
		}
		walletId, err := getWalletId(ctx, ctx.Args().First())
		if err != nil {
			return err
		}
		response, err := client.GetWalletCredentials(*walletId, password)
		if err != nil {
			return err
		}
		printJson(response)
	}
	return nil
}

func listWallets(ctx *cli.Context) error {
	client := getClient(ctx)
	wallets, err := client.GetWallets(nil, true)
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

var bakeMacaroonCommand = &cli.Command{
	Name:  "bakemacaroon",
	Usage: "Bakes a new macaroon",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "tenant",
			Usage: "name of the tenant",
		},
		&cli.StringFlag{
			Name:  "save",
			Usage: "file to save to",
		},
	},
	Action: requireNArgs(1, func(ctx *cli.Context) error {
		client := getClient(ctx)
		request := &boltzrpc.BakeMacaroonRequest{}
		if tenantName := ctx.String("tenant"); tenantName != "" {
			tenant, err := client.GetTenant(tenantName)
			if err != nil {
				return err
			}
			request.TenantId = &tenant.Id
		}
		args := ctx.Args()
		for i := 0; i < args.Len(); i++ {
			switch args.Get(i) {
			case "read":
				request.Permissions = append(request.Permissions, &boltzrpc.MacaroonPermissions{
					Action: boltzrpc.MacaroonAction_READ,
				})
			case "write":
				request.Permissions = append(request.Permissions, &boltzrpc.MacaroonPermissions{
					Action: boltzrpc.MacaroonAction_WRITE,
				})
			}
		}
		response, err := client.BakeMacaroon(request)
		if err != nil {
			return err
		}
		fmt.Println(response.Macaroon)
		if save := ctx.String("save"); save != "" {
			decoded, _ := hex.DecodeString(response.Macaroon)
			if err := os.WriteFile(save, decoded, 0666); err != nil {
				return err
			}
		}

		return nil
	}),
}

var tenantCommands = &cli.Command{
	Name:     "tenant",
	Category: "Tenant",
	Usage:    "Manage the wallets used by the client",
	Subcommands: []*cli.Command{
		{
			Name:      "create",
			Usage:     "Create a new tenant",
			ArgsUsage: "name",
			Description: "Creates a new wallet for the specified currency and unique name.\n" +
				"Currency has to be BTC or LBTC (case insensitive).",
			Action: requireNArgs(1, func(ctx *cli.Context) error {
				client := getClient(ctx)

				tenant, err := client.CreateTenant(ctx.Args().First())
				if err != nil {
					return err
				}
				printJson(tenant)
				return nil
			}),
		},
		{
			Name:  "list",
			Usage: "List all tenants",
			Action: func(ctx *cli.Context) error {
				client := getClient(ctx)

				response, err := client.ListTenants()
				if err != nil {
					return err
				}
				printJson(response)
				return nil
			},
		},
	},
}
