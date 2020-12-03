package main

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/build"
	"github.com/urfave/cli"
	"os"
)

func main() {
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
			Name:  "tlscert",
			Value: "./tls.cert",
			Usage: "Path to the gRPC TLS certificate of Boltz",
		},
		cli.BoolFlag{
			Name:  "no-macaroons",
			Usage: "Disables Macaroon authentication",
		},
		cli.StringFlag{
			Name:  "macaroon",
			Value: "./admin.macaroon",
			Usage: "Path to a gRPC Macaroon of Boltz",
		},
	}
	app.Commands = []cli.Command{
		getInfoCommand,

		listSwapsCommand,
		getSwapCommand,

		depositCommand,
		withdrawCommand,

		createSwapCommand,
		createChannelCreationCommand,
		createReverseSwapCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getClient(ctx *cli.Context) boltz {
	boltz := boltz{
		Host: ctx.GlobalString("host"),
		Port: ctx.GlobalInt("port"),

		TlsCertPath: ctx.GlobalString("tlscert"),

		NoMacaroons:  ctx.GlobalBool("no-macaroons"),
		MacaroonPath: ctx.GlobalString("macaroon"),
	}

	err := boltz.Connect()

	if err != nil {
		fmt.Println("Could not connect to Boltz: " + err.Error())
		os.Exit(1)
	}

	return boltz
}
