package main

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/build"
	"github.com/urfave/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "boltz-cli"
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
	}
	app.Commands = []cli.Command{
		getInfoCommand,
		getSwapCommand,
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
	}

	err := boltz.Connect()

	if err != nil {
		fmt.Println("Could not connect to Boltz: " + err.Error())
		os.Exit(1)
	}

	return boltz
}
