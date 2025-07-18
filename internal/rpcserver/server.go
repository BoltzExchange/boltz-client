package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/config"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/electrum"
	"github.com/BoltzExchange/boltz-client/v2/internal/mempool"
	"github.com/BoltzExchange/boltz-client/v2/internal/nursery"
	liquid_wallet "github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"google.golang.org/grpc/keepalive"

	"github.com/BoltzExchange/boltz-client/v2/internal/autoswap"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/autoswaprpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type RpcServer struct {
	cfg            *config.Config
	grpc           *grpc.Server
	boltzServer    *routedBoltzServer
	autoswapServer *routedAutoSwapServer
}

func NewRpcServer(cfg *config.Config) *RpcServer {
	return &RpcServer{cfg: cfg}
}

func (server *RpcServer) Init() error {
	rpcCfg := server.cfg.RPC

	if err := server.cfg.Database.Connect(); err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}

	keepalivePolicy := keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}
	serverOpts := []grpc.ServerOption{grpc.KeepaliveEnforcementPolicy(keepalivePolicy)}

	swapper := &autoswap.AutoSwap{}
	server.boltzServer = &routedBoltzServer{
		database:   server.cfg.Database,
		stop:       make(chan bool),
		state:      stateLightningSyncing,
		swapper:    swapper,
		referralId: server.cfg.ReferralId,
	}
	server.autoswapServer = &routedAutoSwapServer{
		database: server.cfg.Database,
		swapper:  swapper,
	}
	unaryInterceptors := []grpc.UnaryServerInterceptor{server.boltzServer.UnaryServerInterceptor()}
	streamInterceptors := []grpc.StreamServerInterceptor{server.boltzServer.StreamServerInterceptor()}

	if rpcCfg.NoTls {
		// cleanup previous certificates to avoid confusion
		if err := os.Remove(rpcCfg.TlsCertPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Remove(rpcCfg.TlsKeyPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		certData, err := loadCertificate(rpcCfg.TlsCertPath, rpcCfg.TlsKeyPath, false)

		if err != nil {
			return err
		}
		serverCreds := grpc.Creds(credentials.NewTLS(certData))

		serverOpts = append(serverOpts, serverCreds)
	}

	if rpcCfg.Password != "" {
		passwordAuth := NewPasswordAuth(rpcCfg.Password)
		unaryInterceptors = append(unaryInterceptors, passwordAuth.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, passwordAuth.StreamServerInterceptor())
		logger.Info("Using password authentication")
	} else if !rpcCfg.NoMacaroons {
		macaroon, err := server.generateMacaroons(server.cfg.Database)
		if err != nil {
			return err
		}
		unaryInterceptors = append(unaryInterceptors, macaroon.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, macaroon.StreamServerInterceptor())
		server.boltzServer.macaroon = macaroon
	} else {
		logger.Warn("No authentication mechanism enabled")
	}

	if len(unaryInterceptors) != 0 || len(streamInterceptors) != 0 {
		chainedUnary := grpc.ChainUnaryInterceptor(unaryInterceptors...)
		chainedStream := grpc.ChainStreamInterceptor(streamInterceptors...)

		serverOpts = append(serverOpts, chainedUnary, chainedStream)
	}

	server.grpc = grpc.NewServer(serverOpts...)
	boltzrpc.RegisterBoltzServer(server.grpc, server.boltzServer)
	autoswaprpc.RegisterAutoSwapServer(server.grpc, server.autoswapServer)

	return nil
}

func (server *routedBoltzServer) initLightning(cfg *config.Config) error {
	if server.lightning != nil {
		return nil
	}
	if cfg.Standalone {
		if cfg.Network == "" {
			return errors.New("standalone mode requires a network to be set")
		}
		return nil
	}
	isClnConfigured := cfg.Cln.RootCert != ""
	isLndConfigured := cfg.LND.Macaroon != ""

	if strings.EqualFold(cfg.Node, "CLN") {
		server.lightning = cfg.Cln
	} else if strings.EqualFold(cfg.Node, "LND") {
		server.lightning = cfg.LND
	} else if isClnConfigured && isLndConfigured {
		return errors.New("both CLN and LND are configured. Set --node to specify which node to use")
	} else if isClnConfigured {
		server.lightning = cfg.Cln
	} else if isLndConfigured {
		server.lightning = cfg.LND
	} else {
		return errors.New("no lightning node configured. Configure either CLN or LND")
	}
	return nil
}

func (server *routedBoltzServer) start(cfg *config.Config) (err error) {
	if err := server.initLightning(cfg); err != nil {
		return fmt.Errorf("could not init lightning: %w", err)
	}
	if server.lightning != nil {
		info, err := connectLightning(server.stop, server.lightning)
		if err != nil {
			return err
		}

		if server.state == stateStopping {
			return nil
		}

		if server.lightning.Name() == string(lightning.NodeTypeCln) {
			checkClnVersion(info)
		} else if server.lightning.Name() == string(lightning.NodeTypeLnd) {
			checkLndVersion(info)
		}

		logger.Info(fmt.Sprintf("Connected to lightning node %v (%v): %v", server.lightning.Name(), info.Version, info.Pubkey))

		cfg.Network = info.Network
	}

	server.network, err = boltz.ParseChain(cfg.Network)
	if err != nil {
		return err
	}

	if server.boltz == nil {
		server.boltz, err = initBoltz(cfg, server.network)
		if err != nil {
			return fmt.Errorf("could not init Boltz API: %w", err)
		}
	}

	if server.onchain == nil {
		server.onchain, err = initOnchain(cfg, server.boltz, server.network)
		if err != nil {
			return fmt.Errorf("could not init onchain: %v", err)
		}
	}

	server.nursery = nursery.New(
		cfg.MaxZeroConfAmount,
		cfg.Lightning.RoutingFeeLimitPpm,
		server.network,
		server.lightning,
		server.onchain,
		server.boltz,
		server.database,
	)

	liquidConfig := liquid_wallet.Config{
		Network:     server.network,
		DataDir:     cfg.DataDir + "/liquid-wallet",
		TxProvider:  server.onchain.Liquid.Tx,
		FeeProvider: server.onchain.Liquid.Blocks,
		Persister:   database.NewWalletPersister(server.database),
	}
	electrumConfig := cfg.Electrum()
	if electrumConfig.Liquid.Url != "" {
		liquidConfig.Electrum = &electrumConfig.Liquid
	}
	server.liquidBackend, err = liquid_wallet.NewBlockchainBackend(liquidConfig)
	if err != nil {
		return fmt.Errorf("could not init liquid wallet backend: %v", err)
	}

	autoConfPath := path.Join(cfg.DataDir, "autoswap.toml")
	server.swapper.Init(server.database, server.onchain, autoConfPath, server)

	return server.unlock("")
}

func (server *RpcServer) Start() chan error {
	go func() {
		if err := server.boltzServer.start(server.cfg); err != nil {
			logger.Fatal(fmt.Sprintf("Could not start Boltz server: %v", err))
		}
	}()

	errChannel := make(chan error, 2)

	cfg := server.cfg.RPC

	rpcUrl := cfg.Host + ":"
	if cfg.Port != 0 {
		rpcUrl += strconv.Itoa(cfg.Port)
	}

	// Because the RPC and REST servers are blocking, they are started Go routines

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		logger.Info("Starting RPC server on: " + rpcUrl)

		listener, err := net.Listen("tcp", rpcUrl)

		if err != nil {
			errChannel <- err
			return
		}

		if err := server.grpc.Serve(listener); err != nil {
			errChannel <- err
		}
		wg.Done()
	}()

	var httpServer *http.Server

	if !cfg.RestDisabled {
		wg.Add(1)
		go func() {
			restUrl := cfg.RestHost + ":" + strconv.Itoa(cfg.RestPort)
			logger.Info("Starting REST cfg on: " + restUrl)

			creds := insecure.NewCredentials()
			var err error
			if !cfg.NoTls {
				creds, err = credentials.NewClientTLSFromFile(cfg.TlsCertPath, "")
				if err != nil {
					errChannel <- err
					return
				}
			}

			mux := runtime.NewServeMux()

			var sanitizedRpcUrl string

			if cfg.Host == "0.0.0.0" {
				sanitizedRpcUrl = "127.0.0.1:" + strconv.Itoa(cfg.Port)
			} else {
				sanitizedRpcUrl = rpcUrl
			}

			err = boltzrpc.RegisterBoltzHandlerFromEndpoint(
				context.Background(),
				mux,
				sanitizedRpcUrl,
				[]grpc.DialOption{grpc.WithTransportCredentials(creds)},
			)
			if err != nil {
				errChannel <- err
				return
			}

			httpServer = &http.Server{Addr: restUrl, Handler: mux}

			c := cors.AllowAll()
			httpServer.Handler = c.Handler(httpServer.Handler)

			if cfg.NoTls {
				err = httpServer.ListenAndServe()
			} else {
				err = httpServer.ListenAndServeTLS(cfg.TlsCertPath, cfg.TlsKeyPath)
			}
			if err != nil && err.Error() != "http: Server closed" {
				errChannel <- err
			}
			wg.Done()
		}()
	}

	go func() {
		<-server.boltzServer.stop
		logger.Info("Shutting down")
		if httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(ctx); err != nil {
				errChannel <- err
			}
		}
		server.grpc.GracefulStop()
		wg.Wait()
		close(errChannel)
	}()

	return errChannel
}

func (server *RpcServer) Stop() error {
	_, err := server.boltzServer.Stop(context.Background(), nil)
	return err
}

func initBoltz(cfg *config.Config, network *boltz.Network) (*boltz.Api, error) {
	boltzUrl := cfg.Boltz.URL
	if boltzUrl == "" {
		boltzUrl = network.DefaultBoltzUrl
		logger.Infof("Using default Boltz endpoint for network %s: %s", network.Name, boltzUrl)
	} else {
		logger.Info("Using configured Boltz endpoint: " + boltzUrl)
	}

	boltzApi := &boltz.Api{URL: boltzUrl, Referral: cfg.ReferralId}
	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxy)
		boltzApi.Client = http.Client{
			Transport: transport,
		}
	}

	return boltzApi, nil
}

func initOnchain(cfg *config.Config, boltzApi *boltz.Api, network *boltz.Network) (*onchain.Onchain, error) {
	chain := &onchain.Onchain{
		Btc:     &onchain.Currency{},
		Liquid:  &onchain.Currency{},
		Network: network,
	}

	chain.Init()

	var btcProviders []onchain.TxProvider
	var liquidProviders []onchain.TxProvider

	electrumConfig := cfg.Electrum()
	if network == boltz.Regtest && electrumConfig.Btc.Url == "" && electrumConfig.Liquid.Url == "" {
		electrumConfig = onchain.RegtestElectrumConfig
	}

	if !wallet.Initialized() {
		if cfg.AutoConsolidateThreshold == nil {
			threshold := wallet.DefaultAutoConsolidateThreshold
			cfg.AutoConsolidateThreshold = &threshold
		}
		err := wallet.Init(wallet.Config{
			DataDir:                  cfg.DataDir,
			Network:                  network,
			Debug:                    false,
			Electrum:                 electrumConfig,
			AutoConsolidateThreshold: *cfg.AutoConsolidateThreshold,
		})
		if err != nil {
			return nil, fmt.Errorf("could not init wallet: %v", err)
		}
	}

	if electrumConfig.Btc.Url != "" {
		logger.Info("Using configured Electrum BTC RPC: " + electrumConfig.Btc.Url)
		client, err := electrum.NewClient(electrumConfig.Btc)
		if err != nil {
			return nil, fmt.Errorf("could not connect to electrum: %v", err)
		}
		chain.Btc.Blocks = client
		btcProviders = append(btcProviders, client)
	}
	if electrumConfig.Liquid.Url != "" {
		logger.Info("Using configured Electrum Liquid RPC: " + electrumConfig.Liquid.Url)
		client, err := electrum.NewClient(electrumConfig.Liquid)
		if err != nil {
			return nil, fmt.Errorf("could not connect to electrum: %v", err)
		}
		chain.Liquid.Blocks = client
		liquidProviders = append(liquidProviders, client)
	}
	switch network {
	case boltz.MainNet:
		cfg.MempoolApi = "https://mempool.space/api"
		cfg.MempoolLiquidApi = "https://liquid.bullbitcoin.com/api"
	case boltz.TestNet:
		cfg.MempoolApi = "https://mempool.space/testnet/api"
		cfg.MempoolLiquidApi = "https://liquid.network/liquidtestnet/api"
	}

	if cfg.MempoolApi != "" {
		logger.Info("mempool.space API: " + cfg.MempoolApi)
		client := mempool.InitClient(cfg.MempoolApi)
		chain.Btc.Blocks = client
		btcProviders = append(btcProviders, client)
	}

	if cfg.MempoolLiquidApi != "" {
		logger.Info("liquid.network API: " + cfg.MempoolLiquidApi)
		client := mempool.InitClient(cfg.MempoolLiquidApi)
		chain.Liquid.Blocks = client
		liquidProviders = append(liquidProviders, client)
	}

	chain.Btc.Tx = onchain.MultiTxProvider{
		Providers: append(btcProviders, onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyBtc)),
	}
	chain.Liquid.Tx = onchain.MultiTxProvider{
		Providers: append(liquidProviders, onchain.NewBoltzTxProvider(boltzApi, boltz.CurrencyLiquid)),
	}

	return chain, nil
}
