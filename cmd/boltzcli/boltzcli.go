package main

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/build"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/urfave/cli"
	"os"
	"path"
)

func main() {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	app := cli.NewApp()
	app.Name = "boltzcli"
	app.Usage = ""
	app.Version = build.GetVersion()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "host",
			Value: "127.0.0.1",
			Usage: "gRPC host of Boltz",
		},
		cli.IntFlag{
			Name:  "port",
			Value: 9002,
			Usage: "gRPC port of Boltz",
		},
		cli.StringFlag{
			Name:  "datadir",
			Value: defaultDataDir,
			Usage: "Data directory of boltz-lnd",
		},
		cli.StringFlag{
			Name:  "tlscert",
			Value: "",
			Usage: "Path to the gRPC TLS certificate of Boltz",
		},
		cli.BoolFlag{
			Name:  "no-macaroons",
			Usage: "Disables Macaroon authentication",
		},
		cli.StringFlag{
			Name:  "macaroon",
			Value: "",
			Usage: "Path to a gRPC Macaroon of Boltz",
		},
	}
	app.Commands = []cli.Command{
		getInfoCommand,
		getSwapCommand,
		listSwapsCommand,

		depositCommand,
		withdrawCommand,

		createSwapCommand,
		createReverseSwapCommand,
		createChannelCreationCommand,

		formatMacaroonCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getClient(ctx *cli.Context) boltz {
	dataDir := ctx.GlobalString("datadir")
	macaroonDir := path.Join(dataDir, "macaroons")

	tlsCert := ctx.GlobalString("tlscert")
	macaroon := ctx.GlobalString("macaroon")

	tlsCert = utils.ExpandDefaultPath(dataDir, tlsCert, "tls.cert")
	macaroon = utils.ExpandDefaultPath(macaroonDir, macaroon, "admin.macaroon")

	boltz := boltz{
		Host: ctx.GlobalString("host"),
		Port: ctx.GlobalInt("port"),

		TlsCertPath: tlsCert,

		NoMacaroons:  ctx.GlobalBool("no-macaroons"),
		MacaroonPath: macaroon,
	}

	err := boltz.Connect()

	if err != nil {
		fmt.Println("Could not connect to Boltz: " + err.Error())
		os.Exit(1)
	}

	return boltz
}
