package boltz_lnd

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/build"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/rpcserver"
	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"os"
)

type helpOptions struct {
	ShowHelp    bool `short:"h" long:"help" description:"Display this help message"`
	ShowVersion bool `short:"v" long:"version" description:"Display version and exit"`
}

type config struct {
	ConfigFile string `short:"c" long:"configfile" description:"Path to configuration file"`
	LogFile    string `short:"l" long:"logfile" description:"Path to the log file"`

	// TODO: set Boltz URL based on network by default
	Boltz    *boltz.Boltz         `group:"Boltz Options"`
	LND      *lnd.LND             `group:"LND Options"`
	RPC      *rpcserver.RpcServer `group:"RPC options"`
	Database *database.Database   `group:"Database options"`

	Help *helpOptions `group:"Help Options"`
}

func LoadConfig() *config {
	cfg := config{
		LogFile:    "./boltz.log",
		ConfigFile: "./boltz.toml",

		Boltz: &boltz.Boltz{
			URL: "http://127.0.0.1:9001",
		},

		LND: &lnd.LND{
			Host:        "127.0.0.1",
			Port:        10009,
			Macaroon:    "",
			Certificate: "",
		},

		RPC: &rpcserver.RpcServer{
			Host: "127.0.0.1",
			Port: 9002,
		},

		Database: &database.Database{
			Path: "./boltz.db",
		},
	}

	parser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	_, err := parser.Parse()

	if cfg.Help.ShowVersion {
		fmt.Println(build.GetVersion())
		os.Exit(0)
	}

	if cfg.Help.ShowHelp {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	if err != nil {
		printCouldNotParseCli(err)
	}

	if err != nil {
		printCouldNotParseCli(err)
	}

	if cfg.ConfigFile != "" {
		_, err := toml.DecodeFile(cfg.ConfigFile, &cfg)

		if err != nil {
			fmt.Printf("Could not read config file: " + err.Error() + "\n")
		}
	}

	_, err = flags.Parse(&cfg)

	if err != nil {
		printCouldNotParseCli(err)
	}

	return &cfg
}

func printCouldNotParseCli(err error) {
	printFatal("Could not parse CLI arguments: %s", err)
}
