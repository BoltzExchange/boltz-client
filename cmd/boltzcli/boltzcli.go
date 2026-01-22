package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/BoltzExchange/boltz-client/v2/internal/build"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/status"
)

func main() {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	app := cli.NewApp()
	app.Name = "boltzcli"
	app.Usage = "A command line interface for boltzd"
	app.Version = build.GetVersion()
	app.EnableBashCompletion = true
	app.ExitErrHandler = func(context *cli.Context, err error) {
		if err == nil {
			return
		}
		s, ok := status.FromError(err)
		if ok {
			msg := s.Message()
			if strings.Contains(msg, "connection refused") {
				conn := getConnection(context)
				fmt.Printf("could not connect to boltzd. make sure it is running at %s:%d and try again\n", conn.Host, conn.Port)
			} else if strings.Contains(msg, "autoswap") {
				fmt.Println(msg)
				fmt.Println("run autoswap setup to reset or initialize autoswap")
			} else {
				fmt.Println(msg)
			}
		} else {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Value:   "127.0.0.1",
			Usage:   "gRPC host of Boltz",
			EnvVars: []string{"BOLTZ_HOST"},
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   9002,
			Usage:   "gRPC port of Boltz",
			EnvVars: []string{"BOLTZ_PORT"},
		},
		&cli.StringFlag{
			Name:    "datadir",
			Aliases: []string{"d"},
			Value:   defaultDataDir,
			Usage:   "Data directory of boltz-client",
			EnvVars: []string{"BOLTZ_DATADIR"},
		},
		&cli.StringFlag{
			Name:    "tlscert",
			Value:   "",
			Usage:   "Path to the gRPC TLS certificate of Boltz",
			EnvVars: []string{"BOLTZ_TLSCERT"},
		},
		&cli.BoolFlag{
			Name:    "no-macaroons",
			Usage:   "Disables Macaroon authentication",
			EnvVars: []string{"BOLTZ_NO_MACAROONS"},
		},
		&cli.StringFlag{
			Name:    "macaroon",
			Value:   "",
			Usage:   "Path to a gRPC Macaroon of Boltz",
			EnvVars: []string{"BOLTZ_MACAROON"},
		},
		&cli.StringFlag{
			Name:    "tenant",
			Value:   "",
			Usage:   "Id or name of the tenant to use for the request",
			EnvVars: []string{"BOLTZ_TENANT"},
		},
		&cli.StringFlag{
			Name:    "password",
			Usage:   "Password for authentication",
			EnvVars: []string{"BOLTZ_PASSWORD"},
		},
	}
	app.Commands = []*cli.Command{
		getInfoCommand,
		getPairsCommand,
		getSwapQuoteCommand,
		getSwapCommand,
		swapInfoStreamCommand,
		listSwapsCommand,
		getStatsCommand,

		createSwapCommand,
		createReverseSwapCommand,
		createChainSwapCommand,
		refundSwapCommand,
		claimSwapsCommand,

		createFundingCommand,
		listFundingAddressesCommand,
		fundingAddressStreamCommand,
		fundSwapCommand,
		refundFundingAddressCommand,

		autoSwapCommands,

		walletCommands,
		bakeMacaroonCommand,
		tenantCommands,
		swapMnemonicCommands,

		formatMacaroonCommand,
		shellCompletionsCommand,
		stopCommand,
		unlockCommand,
		changePasswordCommand,
		verifyPasswordCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

type Key string

const ConnectionKey Key = "connection"

func getConnection(ctx *cli.Context) client.Connection {
	if ctx.Context.Value(ConnectionKey) != nil {
		return ctx.Context.Value(ConnectionKey).(client.Connection)
	}

	dataDir := ctx.String("datadir")
	macaroonDir := path.Join(dataDir, "macaroons")

	tlsCert := ctx.String("tlscert")
	macaroon := ctx.String("macaroon")
	password := ctx.String("password")

	if tlsCert == "" {
		defaultPath := path.Join(dataDir, "tls.cert")
		// only use the default path if it exists, since the server is probably running without tls
		if utils.FileExists(defaultPath) {
			tlsCert = defaultPath
		}
	}

	macaroon = utils.ExpandDefaultPath(macaroonDir, macaroon, "admin.macaroon")

	boltz := client.Connection{
		Host: ctx.String("host"),
		Port: ctx.Int("port"),

		TlsCertPath: tlsCert,

		NoMacaroons:  ctx.Bool("no-macaroons"),
		MacaroonPath: macaroon,
		Password:     password,
	}

	err := boltz.Connect()

	if tenant := ctx.String("tenant"); tenant != "" && ctx.Command.Name != "bakemacaroon" {
		boltz.SetTenant(tenant)
	}

	if err != nil {
		fmt.Println("Could not connect to Boltz: " + err.Error())
		os.Exit(1)
	}

	ctx.Context = context.WithValue(ctx.Context, ConnectionKey, boltz)

	return boltz
}

func getClient(ctx *cli.Context) client.Boltz {
	conn := getConnection(ctx)
	return client.NewBoltzClient(conn)
}

func getAutoSwapClient(ctx *cli.Context) client.AutoSwap {
	conn := getConnection(ctx)
	return client.NewAutoSwapClient(conn)
}
