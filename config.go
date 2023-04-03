package boltz_lnd

import (
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/build"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/rpcserver"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"os"
	"path"
	"runtime"
)

type helpOptions struct {
	ShowHelp    bool `short:"h" long:"help" description:"Display this help message"`
	ShowVersion bool `short:"v" long:"version" description:"Display version and exit"`
}

type Config struct {
	DataDir string `short:"d" long:"datadir" description:"Data directory of boltz-lnd"`

	ConfigFile string `short:"c" long:"configfile" description:"Path to configuration file"`

	LogFile   string `short:"l" long:"logfile" description:"Path to the log file"`
	LogPrefix string `long:"logprefix" description:"Prefix of all log messages"`

	Boltz    *boltz.Boltz         `group:"Boltz Options"`
	LND      *lnd.LND             `group:"LND Options"`
	RPC      *rpcserver.RpcServer `group:"RPC options"`
	Database *database.Database   `group:"Database options"`

	MempoolApi string `long:"mempool" description:"mempool.space API to use for fee estimations"`

	Help *helpOptions `group:"Help Options"`
}

func LoadConfig() *Config {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	cfg := Config{
		DataDir: defaultDataDir,

		ConfigFile: "",

		LogFile:   "",
		LogPrefix: "",

		Boltz: &boltz.Boltz{
			URL: "",
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

		MempoolApi: "https://mempool.space/api",
	}

	parser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	_, err = parser.Parse()

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

	_, err = flags.Parse(&cfg)

	if err != nil {
		printCouldNotParse(err)
	}

	fmt.Println("Using data dir: " + cfg.DataDir)

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
