package boltz_lnd

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/BoltzExchange/boltz-client/onchain/liquid"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/build"
	"github.com/BoltzExchange/boltz-client/cln"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/lnd"
	"github.com/BoltzExchange/boltz-client/rpcserver"
	"github.com/BoltzExchange/boltz-client/utils"
)

type helpOptions struct {
	ShowHelp    bool `short:"h" long:"help" description:"Display this help message"`
	ShowVersion bool `short:"v" long:"version" description:"Display version and exit"`
}

type Config struct {
	DataDir string `short:"d" long:"datadir" description:"Data directory of boltz-client"`

	ConfigFile string `short:"c" long:"configfile" description:"Path to configuration file"`

	LogFile  string `short:"l" long:"logfile" description:"Path to the log file"`
	LogLevel string `long:"loglevel" description:"Log level (fatal, error, warn, info, debug, silly)"`

	Boltz *boltz.Boltz `group:"Boltz Options"`
	LND   *lnd.LND     `group:"LND Options"`
	Cln   *cln.Cln     `group:"Cln Options"`

	Node string `long:"node" description:"Lightning node to use (cln or lnd)"`

	Lightning lightning.LightningNode

	RPC      *rpcserver.RpcServer `group:"RPC options"`
	Database *database.Database   `group:"Database options"`

	MempoolApi       string `long:"mempool" description:"mempool.space API to use for fee estimations; set to empty string to disable"`
	MempoolLiquidApi string `long:"mempool-liquid" description:"mempool.space liquid API to use for fee estimations; set to empty string to disable"`

	Help *helpOptions `group:"Help Options"`

	// there might be some config for liquid in the future
	LiquidWallet *liquid.Wallet
}

func LoadConfig(dataDir string) *Config {
	cfg := Config{
		DataDir: dataDir,

		ConfigFile: "",

		LogFile:  "",
		LogLevel: "info",

		Boltz: &boltz.Boltz{
			URL: "",
		},

		LND: &lnd.LND{
			Host:        "127.0.0.1",
			Port:        10009,
			Macaroon:    "",
			Certificate: "",
		},

		Cln: &cln.Cln{
			Host: "",
			Port: 10009,

			RootCert:   "",
			PrivateKey: "",
			CertChain:  "",
		},

		RPC: &rpcserver.RpcServer{
			Host: "127.0.0.1",
			Port: 9002,

			RestHost:     "127.0.0.1",
			RestPort:     9003,
			RestDisabled: false,

			TlsCertPath: "",
			TlsKeyPath:  "",

			NoMacaroons:          false,
			AdminMacaroonPath:    "",
			ReadonlyMacaroonPath: "",
		},

		Database: &database.Database{
			Path: "",
		},
	}

	parser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	_, err := parser.Parse()

	if cfg.Help.ShowVersion {
		fmt.Println(build.GetVersion())
		fmt.Println("Built with: " + runtime.Version())
		os.Exit(0)
	}

	if cfg.Help.ShowHelp {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	if err != nil {
		printCouldNotParse(err)
	}

	cfg.ConfigFile = utils.ExpandDefaultPath(cfg.DataDir, cfg.ConfigFile, "boltz.toml")

	if cfg.ConfigFile != "" {
		_, err := toml.DecodeFile(cfg.ConfigFile, &cfg)

		if err != nil {
			fmt.Printf("Could not read config file: " + err.Error() + "\n")
		}
	}

	fmt.Println("Using data dir: " + cfg.DataDir)

	cfg.LND.Macaroon = utils.ExpandHomeDir(cfg.LND.Macaroon)
	cfg.LND.Certificate = utils.ExpandHomeDir(cfg.LND.Certificate)

	cfg.Cln.RootCert = utils.ExpandHomeDir(cfg.Cln.RootCert)
	cfg.Cln.PrivateKey = utils.ExpandHomeDir(cfg.Cln.PrivateKey)
	cfg.Cln.CertChain = utils.ExpandHomeDir(cfg.Cln.CertChain)

	cfg.LogFile = utils.ExpandDefaultPath(cfg.DataDir, cfg.LogFile, "boltz.log")
	cfg.Database.Path = utils.ExpandDefaultPath(cfg.DataDir, cfg.Database.Path, "boltz.db")

	cfg.RPC.TlsKeyPath = utils.ExpandDefaultPath(cfg.DataDir, cfg.RPC.TlsKeyPath, "tls.key")
	cfg.RPC.TlsCertPath = utils.ExpandDefaultPath(cfg.DataDir, cfg.RPC.TlsCertPath, "tls.cert")

	macaroonDir := path.Join(cfg.DataDir, "macaroons")

	cfg.RPC.AdminMacaroonPath = utils.ExpandDefaultPath(macaroonDir, cfg.RPC.AdminMacaroonPath, "admin.macaroon")
	cfg.RPC.ReadonlyMacaroonPath = utils.ExpandDefaultPath(macaroonDir, cfg.RPC.ReadonlyMacaroonPath, "readonly.macaroon")

	createDirIfNotExists(cfg.DataDir)
	createDirIfNotExists(macaroonDir)

	return &cfg
}

func createDirIfNotExists(dir string) {
	if !utils.FileExists(dir) {
		err := os.Mkdir(dir, 0700)

		if err != nil {
			fmt.Println("Could not create directory: " + err.Error())
			os.Exit(1)
		}
	}
}

func printCouldNotParse(err error) {
	fmt.Println("Could not parse arguments: " + err.Error())
	os.Exit(1)
}
