package config

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"

	"github.com/BoltzExchange/boltz-client/build"
	"github.com/BoltzExchange/boltz-client/cln"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/lnd"
	"github.com/BoltzExchange/boltz-client/utils"
)

type helpOptions struct {
	ShowHelp    bool `short:"h" long:"help" description:"Display this help message"`
	ShowVersion bool `short:"v" long:"version" description:"Display version and exit"`
}

type boltzOptions struct {
	URL string `long:"boltz.url" description:"Boltz API URL"`
}

type RpcOptions struct {
	Host string `long:"rpc.host" description:"gRPC host to which Boltz should listen"`
	Port int    `long:"rpc.port" short:"p" description:"gRPC port to which Boltz should listen"`

	RestHost     string `long:"rpc.rest.host" description:"REST host to which Boltz should listen"`
	RestPort     int    `long:"rpc.rest.port" description:"REST port to which Boltz should listen"`
	RestDisabled bool   `long:"rpc.rest.disable" description:"Disables the REST API proxy"`

	TlsCertPath string `long:"rpc.tlscert" description:"Path to the TLS certificate of boltz-client"`
	TlsKeyPath  string `long:"rpc.tlskey" description:"Path to the TLS private key of boltz-client"`
	NoTls       bool   `long:"rpc.no-tls" description:"Disables TLS"`

	NoMacaroons          bool   `long:"rpc.no-macaroons" description:"Disables Macaroon authentication"`
	AdminMacaroonPath    string `long:"rpc.adminmacaroonpath" description:"Path to the admin Macaroon"`
	ReadonlyMacaroonPath string `long:"rpc.readonlymacaroonpath" description:"Path to the readonly macaroon"`
}

type Config struct {
	DataDir string `short:"d" long:"datadir" description:"Data directory of boltz-client"`

	ConfigFile string `short:"c" long:"configfile" description:"Path to configuration file"`

	LogFile    string `short:"l" long:"logfile" description:"Path to the log file"`
	LogLevel   string `long:"loglevel" description:"Log level (fatal, error, warn, info, debug, silly)"`
	LogMaxSize int    `long:"logmaxsize" description:"Maximum size of the log file in megabytes before it gets rotated"`
	LogMaxAge  int    `long:"logmaxage" description:"Maximum age of old log files in days before they get deleted"`

	Log logger.Options

	Network string `long:"network" description:"Network to use (mainnet, testnet, regtest)"`

	Boltz *boltzOptions `group:"Boltz Options"`
	LND   *lnd.LND      `group:"LND Options"`
	Cln   *cln.Cln      `group:"Cln Options"`

	Node string `long:"node" description:"Lightning node to use (cln or lnd)"`

	Standalone bool `long:"standalone" description:"Run boltz-client without a lightning node"`

	Lightning lightning.LightningNode

	RPC      *RpcOptions        `group:"RPC options"`
	Database *database.Database `group:"Database options"`

	MempoolApi       string `long:"mempool" description:"mempool.space API to use for fee estimations; set to empty string to disable"`
	MempoolLiquidApi string `long:"mempool-liquid" description:"mempool.space liquid API to use for fee estimations; set to empty string to disable"`

	ElectrumUrl       string `long:"electrum" description:"electrum rpc to use for fee estimations; set to empty string to disable"`
	ElectrumSSL       bool   `long:"electrum-ssl" description:"whether the electrum server uses ssl"`
	ElectrumLiquidUrl string `long:"electrum-liquid" description:"electrum rpc to use for fee estimations; set to empty string to disable"`
	ElectrumLiquidSSL bool   `long:"electrum-liquid-ssl" description:"whether the electrum server uses ssl"`

	Proxy string `long:"proxy" description:"Proxy URL to use for all Boltz API requests"`

	Help *helpOptions `group:"Help Options"`
}

func (c *Config) Electrum() onchain.ElectrumConfig {
	return onchain.ElectrumConfig{
		Btc:    onchain.ElectrumOptions{Url: c.ElectrumUrl, SSL: c.ElectrumSSL},
		Liquid: onchain.ElectrumOptions{Url: c.ElectrumLiquidUrl, SSL: c.ElectrumLiquidSSL},
	}
}

func LoadConfig(dataDir string) (*Config, error) {
	cfg := Config{
		DataDir: dataDir,

		ConfigFile: "",

		LogLevel:   "info",
		LogMaxSize: 5,
		LogMaxAge:  30,

		Network: "mainnet",

		Boltz: &boltzOptions{},

		LND: &lnd.LND{
			Host:        "127.0.0.1",
			Port:        10009,
			Macaroon:    "",
			Certificate: "",
		},

		Cln: &cln.Cln{
			Host: "127.0.0.1",
			Port: 10009,

			RootCert:   "",
			PrivateKey: "",
			CertChain:  "",
			ServerName: "cln",
		},

		RPC: &RpcOptions{
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
	if err != nil {
		printCouldNotParse(err)
	}

	if cfg.Help.ShowVersion {
		fmt.Println(build.GetVersion())
		fmt.Println("Built with: " + runtime.Version())
		os.Exit(0)
	}

	if cfg.Help.ShowHelp {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	cfg.ConfigFile = utils.ExpandDefaultPath(cfg.DataDir, cfg.ConfigFile, "boltz.toml")

	if cfg.ConfigFile != "" && utils.FileExists(cfg.ConfigFile) {
		_, err := toml.DecodeFile(cfg.ConfigFile, &cfg)

		if err != nil {
			return nil, fmt.Errorf("Could not read config file: %v", err)
		}
	}

	// parse a second time to ensure cli flags go over config values
	_, err = parser.Parse()
	if err != nil {
		printCouldNotParse(err)
	}

	fmt.Println("Using data dir: " + cfg.DataDir)

	if strings.EqualFold(cfg.Node, "CLN") && cfg.Cln.DataDir == "" {
		cfg.Cln.DataDir = "~/.lightning"
	} else if strings.EqualFold(cfg.Node, "LND") && cfg.LND.DataDir == "" {
		cfg.LND.DataDir = "~/.lnd"
	}

	cfg.LND.Macaroon = utils.ExpandHomeDir(cfg.LND.Macaroon)
	cfg.LND.Certificate = utils.ExpandHomeDir(cfg.LND.Certificate)

	if cfg.LND.DataDir != "" {
		cfg.LND.DataDir = utils.ExpandHomeDir(cfg.LND.DataDir)
		if cfg.Network == "" {
			return nil, fmt.Errorf("network must be set when lnd datadir is configured")
		}

		defaultMacaroon := fmt.Sprintf("./data/chain/bitcoin/%s/admin.macaroon", cfg.Network)
		cfg.LND.Macaroon = utils.ExpandDefaultPath(cfg.LND.DataDir, cfg.LND.Macaroon, defaultMacaroon)
		cfg.LND.Certificate = utils.ExpandDefaultPath(cfg.LND.DataDir, cfg.LND.Certificate, "tls.cert")
	}

	cfg.Cln.RootCert = utils.ExpandHomeDir(cfg.Cln.RootCert)
	cfg.Cln.PrivateKey = utils.ExpandHomeDir(cfg.Cln.PrivateKey)
	cfg.Cln.CertChain = utils.ExpandHomeDir(cfg.Cln.CertChain)

	if cfg.Cln.DataDir != "" {
		cfg.Cln.DataDir = utils.ExpandHomeDir(cfg.Cln.DataDir)
		if cfg.Network == "" {
			return nil, fmt.Errorf("network must be set when cln datadir is configured")
		}
		if cfg.Network == "mainnet" {
			cfg.Cln.DataDir += "/bitcoin"
		} else {
			cfg.Cln.DataDir += "/" + cfg.Network
		}
		cfg.Cln.RootCert = utils.ExpandDefaultPath(cfg.Cln.DataDir, cfg.Cln.RootCert, "ca.pem")
		cfg.Cln.PrivateKey = utils.ExpandDefaultPath(cfg.Cln.DataDir, cfg.Cln.PrivateKey, "client-key.pem")
		cfg.Cln.CertChain = utils.ExpandDefaultPath(cfg.Cln.DataDir, cfg.Cln.CertChain, "client.pem")
	}

	cfg.LogFile = utils.ExpandDefaultPath(cfg.DataDir, cfg.LogFile, "boltz.log")
	cfg.Log = logger.Options{
		Level: cfg.LogLevel,
		Logger: &lumberjack.Logger{
			Filename: cfg.LogFile,
			MaxAge:   cfg.LogMaxAge,
			MaxSize:  cfg.LogMaxSize,
		},
	}
	cfg.Database.Path = utils.ExpandDefaultPath(cfg.DataDir, cfg.Database.Path, "boltz.db")

	cfg.RPC.TlsKeyPath = utils.ExpandDefaultPath(cfg.DataDir, cfg.RPC.TlsKeyPath, "tls.key")
	cfg.RPC.TlsCertPath = utils.ExpandDefaultPath(cfg.DataDir, cfg.RPC.TlsCertPath, "tls.cert")

	macaroonDir := path.Join(cfg.DataDir, "macaroons")

	cfg.RPC.AdminMacaroonPath = utils.ExpandDefaultPath(macaroonDir, cfg.RPC.AdminMacaroonPath, "admin.macaroon")
	cfg.RPC.ReadonlyMacaroonPath = utils.ExpandDefaultPath(macaroonDir, cfg.RPC.ReadonlyMacaroonPath, "readonly.macaroon")

	createDirIfNotExists(cfg.DataDir)
	createDirIfNotExists(macaroonDir)

	return &cfg, nil
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
