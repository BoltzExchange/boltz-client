package rpcserver

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/boltzrpc/autoswaprpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/macaroons"
	"github.com/BoltzExchange/boltz-client/nursery"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type RpcServer struct {
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

	Grpc *grpc.Server

	Stop chan bool `json:"-"`
}

func (server *RpcServer) Init(
	network *boltz.Network,
	lightning lightning.LightningNode,
	boltz *boltz.Boltz,
	database *database.Database,
	onchain *onchain.Onchain,
	autoSwapConfigPath string,
) error {
	nursery := &nursery.Nursery{}
	err := nursery.Init(
		network,
		lightning,
		onchain,
		boltz,
		database,
	)

	if err != nil {
		logger.Fatal("Could not start Swap nursery: " + err.Error())
	}

	var serverOpts []grpc.ServerOption

	if !server.NoTls {
		certData, err := loadCertificate(server.TlsCertPath, server.TlsKeyPath, false)

		if err != nil {
			return err
		}
		serverCreds := grpc.Creds(credentials.NewTLS(certData))

		serverOpts = append(serverOpts, serverCreds)
	}
	var macaroonService *macaroons.Service

	if !server.NoMacaroons {
		macaroonService, err = server.generateMacaroons(database)

		if err != nil {
			return err
		}
	} else {
		logger.Warn("Disabled Macaroon authentication")
	}

	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	if macaroonService != nil {
		unaryInterceptors = append(unaryInterceptors, macaroonService.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, macaroonService.StreamServerInterceptor())
	}

	if len(unaryInterceptors) != 0 || len(streamInterceptors) != 0 {
		chainedUnary := grpc.ChainUnaryInterceptor(unaryInterceptors...)
		chainedStream := grpc.ChainStreamInterceptor(streamInterceptors...)

		serverOpts = append(serverOpts, chainedUnary, chainedStream)
	}

	server.Grpc = grpc.NewServer(serverOpts...)
	server.Stop = make(chan bool)
	routedServer := &routedBoltzServer{
		network: network,

		lightning: lightning,
		boltz:     boltz,
		nursery:   nursery,
		database:  database,
		onchain:   onchain,

		stop: server.Stop,
	}
	boltzrpc.RegisterBoltzServer(server.Grpc, routedServer)

	swapper := &autoswap.AutoSwapper{
		ExecuteSwap: func(request *boltzrpc.CreateSwapRequest) error {
			_, err := routedServer.createSwap(true, request)
			return err
		},
		ExecuteReverseSwap: func(request *boltzrpc.CreateReverseSwapRequest) error {
			_, err := routedServer.createReverseSwap(true, request)
			return err
		},
		ListChannels:   routedServer.lightning.ListChannels,
		GetServiceInfo: routedServer.getPairs,
	}
	swapper.Init(database, onchain, autoSwapConfigPath)
	if err := swapper.LoadConfig(); err != nil {
		logger.Warnf("Could not load autoswap config: %v", err)
	}

	routedAutoSwapServer := &routedAutoSwapServer{
		swapper:  swapper,
		database: database,
	}
	autoswaprpc.RegisterAutoSwapServer(server.Grpc, routedAutoSwapServer)

	routedServer.swapper = swapper

	return nil
}

func (server *RpcServer) Start() chan error {
	errChannel := make(chan error, 2)

	rpcUrl := server.Host + ":"
	if server.Port != 0 {
		rpcUrl += strconv.Itoa(server.Port)
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

		if err := server.Grpc.Serve(listener); err != nil {
			errChannel <- err
		}
		wg.Done()
	}()

	var httpServer *http.Server

	if !server.RestDisabled {
		wg.Add(1)
		go func() {
			restUrl := server.RestHost + ":" + strconv.Itoa(server.RestPort)
			logger.Info("Starting REST server on: " + restUrl)

			restCreds, err := getRestDialOptions(server.TlsCertPath)

			if err != nil {
				errChannel <- err
				return
			}

			mux := runtime.NewServeMux()

			var sanitizedRpcUrl string

			if server.Host == "0.0.0.0" {
				sanitizedRpcUrl = "127.0.0.1:" + strconv.Itoa(server.Port)
			} else {
				sanitizedRpcUrl = rpcUrl
			}

			err = boltzrpc.RegisterBoltzHandlerFromEndpoint(
				context.Background(),
				mux,
				sanitizedRpcUrl,
				restCreds,
			)

			if err != nil {
				errChannel <- err
				return
			}

			httpServer = &http.Server{Addr: restUrl, Handler: mux}

			if err := httpServer.ListenAndServeTLS(server.TlsCertPath, server.TlsKeyPath); err != nil {
				if err.Error() != "http: Server closed" {
					errChannel <- err
				}
			}
			wg.Done()
		}()
	}

	go func() {
		<-server.Stop
		logger.Info("Shutting down")
		if httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(ctx); err != nil {
				errChannel <- err
			}
		}
		server.Grpc.GracefulStop()
		wg.Wait()
		close(errChannel)
	}()

	return errChannel
}

func getRestDialOptions(tlsCertPath string) ([]grpc.DialOption, error) {
	restCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")

	if err != nil {
		return nil, err
	}

	return []grpc.DialOption{
		grpc.WithTransportCredentials(restCreds),
	}, nil
}
