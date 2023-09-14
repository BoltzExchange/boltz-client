package rpcserver

import (
	"context"
	"net"
	"net/http"
	"strconv"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/macaroons"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/chaincfg"
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

	TlsCertPath string `long:"rpc.tlscert" description:"Path to the TLS certificate of boltz-lnd"`
	TlsKeyPath  string `long:"rpc.tlskey" description:"Path to the TLS private key of boltz-lnd"`
	NoTls       bool   `long:"rpc.no-tls" description:"Disables TLS"`

	NoMacaroons          bool   `long:"rpc.no-macaroons" description:"Disables Macaroon authentication"`
	AdminMacaroonPath    string `long:"rpc.adminmacaroonpath" description:"Path to the admin Macaroon"`
	ReadonlyMacaroonPath string `long:"rpc.readonlymacaroonpath" description:"Path to the readonly macaroon"`

	Grpc *grpc.Server
}

func (server *RpcServer) Init(
	symbol string,
	chainParams *chaincfg.Params,
	lnd *lnd.LND,
	boltz *boltz.Boltz,
	nursery *nursery.Nursery,
	database *database.Database,
) error {

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
	var err error

	if !server.NoMacaroons {
		macaroonService, err = server.generateMacaroons(database)

		if err != nil {
			return err
		}
	} else {
		logger.Warning("Disabled Macaroon authentication")
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
	boltzrpc.RegisterBoltzServer(server.Grpc, &routedBoltzServer{
		symbol:      symbol,
		chainParams: chainParams,

		lnd:       lnd,
		lightning: lnd,
		boltz:     boltz,
		nursery:   nursery,
		database:  database,
	})

	return nil
}

func (server *RpcServer) Start() chan error {
	errChannel := make(chan error)

	rpcUrl := server.Host + ":"
	if server.Port != 0 {
		rpcUrl += strconv.Itoa(server.Port)
	}

	// Because the RPC and REST servers are blocking, they are started Go routines

	go func() {
		logger.Info("Starting RPC server on: " + rpcUrl)

		listener, err := net.Listen("tcp", rpcUrl)

		if err != nil {
			errChannel <- err
			return
		}

		errChannel <- server.Grpc.Serve(listener)
	}()

	if !server.RestDisabled {
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

			errChannel <- http.ListenAndServeTLS(restUrl, server.TlsCertPath, server.TlsKeyPath, mux)
		}()
	}

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
