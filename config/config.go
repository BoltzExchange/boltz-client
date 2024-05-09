package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"

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

	Network string `long:"network" description:"Network to use (mainnet, testnet, regtest)"`

	Boltz *boltz.Boltz `group:"Boltz Options"`
	LND   *lnd.LND     `group:"LND Options"`
	Cln   *cln.Cln     `group:"Cln Options"`

	Node string `long:"node" description:"Lightning node to use (cln or lnd)"`

	Standalone bool `long:"standalone" description:"Run boltz-client without a lightning node"`

	Lightning lightning.LightningNode

	RPC      *rpcserver.RpcServer `group:"RPC options"`
	Database *database.Database   `group:"Database options"`

	MempoolApi       string `long:"mempool" description:"mempool.space API to use for fee estimations; set to empty string to disable"`
	MempoolLiquidApi string `long:"mempool-liquid" description:"mempool.space liquid API to use for fee estimations; set to empty string to disable"`

	ElectrumUrl            string `long:"electrum" description:"electrum rpc to use for fee estimations; set to empty string to disable"`
	ElectrumSSL            bool   `long:"electrum-ssl" description:"whether the electrum server uses ssl"`
	ElectrumLiquidUrl      string `long:"electrum-liquid" description:"electrum rpc to use for fee estimations; set to empty string to disable"`
	ElectrumLiquiLiquidSSL bool   `long:"electrum-liquid-ssl" description:"whether the electrum server uses ssl"`

	Proxy string `long:"proxy" description:"Proxy URL to use for all Boltz API requests"`

	Help *helpOptions `group:"Help Options"`
}

func LoadConfig(dataDir string) (*Config, error) {
	cfg := Config{
		DataDir: dataDir,

		ConfigFile: "",

		LogFile:  "",
		LogLevel: "info",

		Network: "mainnet",

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
			Host: "127.0.0.1",
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

	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxy)
		cfg.Boltz.Client = http.Client{
			Transport: transport,
		}
	}

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
