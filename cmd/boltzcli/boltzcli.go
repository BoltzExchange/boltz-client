package main

import (
	"fmt"
	"os"
	"path"

	"github.com/BoltzExchange/boltz-client/boltzrpc/client"
	"github.com/BoltzExchange/boltz-client/build"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/urfave/cli/v2"
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
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "127.0.0.1",
			Usage: "gRPC host of Boltz",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 9002,
			Usage: "gRPC port of Boltz",
		},
		&cli.StringFlag{
			Name:  "datadir",
			Value: defaultDataDir,
			Usage: "Data directory of boltz-client",
		},
		&cli.StringFlag{
			Name:  "tlscert",
			Value: "",
			Usage: "Path to the gRPC TLS certificate of Boltz",
		},
		&cli.BoolFlag{
			Name:  "no-macaroons",
			Usage: "Disables Macaroon authentication",
		},
		&cli.StringFlag{
			Name:  "macaroon",
			Value: "",
			Usage: "Path to a gRPC Macaroon of Boltz",
		},
	}
	app.Commands = []*cli.Command{
		getInfoCommand,
		getSwapCommand,
		swapInfoStreamCommand,
		listSwapsCommand,

		createSwapCommand,
		createReverseSwapCommand,

		autoSwapCommands,

		liquidWalletCommands,

		formatMacaroonCommand,
		shellCompletionsCommand,
		stopCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getConnection(ctx *cli.Context) client.Connection {
	dataDir := ctx.String("datadir")
	macaroonDir := path.Join(dataDir, "macaroons")

	tlsCert := ctx.String("tlscert")
	macaroon := ctx.String("macaroon")

	tlsCert = utils.ExpandDefaultPath(dataDir, tlsCert, "tls.cert")
	macaroon = utils.ExpandDefaultPath(macaroonDir, macaroon, "admin.macaroon")

	boltz := client.Connection{
		Host: ctx.String("host"),
		Port: ctx.Int("port"),

		TlsCertPath: tlsCert,

		NoMacaroons:  ctx.Bool("no-macaroons"),
		MacaroonPath: macaroon,
	}

	err := boltz.Connect()

	if err != nil {
		fmt.Println("Could not connect to Boltz: " + err.Error())
		os.Exit(1)
	}

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
