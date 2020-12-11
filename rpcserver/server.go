package rpcserver

import (
	"context"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/macaroons"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/btcsuite/btcd/chaincfg"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
)

type RpcServer struct {
	Host string `long:"rpc.host" description:"gRPC host to which Boltz should listen"`
	Port int    `long:"rpc.port" short:"p" description:"gRPC port to which Boltz should listen"`

	RestHost     string `long:"rpc.rest.host" description:"REST host to which Boltz should listen"`
	RestPort     int    `long:"rpc.rest.port" description:"REST port to which Boltz should listen"`
	RestDisabled bool   `long:"rpc.rest.disable" description:"Disables the REST API proxy"`

	TlsCertPath string `long:"rpc.tlscert" description:"Path to the TLS certificate of boltz-lnd"`
	TlsKeyPath  string `long:"rpc.tlskey" description:"Path to the TLS private key of boltz-lnd"`

	NoMacaroons       bool   `long:"rpc.no-macaroons" description:"Disables Macaroon authentication"`
	AdminMacaroonPath string `long:"rpc.adminmacaroonpath" description:"Path to the Admin Macaroon"`
}

func (server *RpcServer) Start(
	symbol string,
	chainParams *chaincfg.Params,
	lnd *lnd.LND,
	boltz *boltz.Boltz,
	nursery *nursery.Nursery,
	database *database.Database,
) chan error {
	errChannel := make(chan error)

	go func() {
		certData, err := loadCertificate(server.TlsCertPath, server.TlsKeyPath, false)

		if err != nil {
			errChannel <- err
			return
		}

		var macaroonService *macaroons.Service

		if !server.NoMacaroons {
			macaroonService, err = server.generateMacaroons(database)

			if err != nil {
				errChannel <- err
				return
			}
		} else {
			logger.Warning("Disabled Macaroon authentication")
		}

		serverCreds := grpc.Creds(credentials.NewTLS(certData))

		var serverOpts []grpc.ServerOption

		serverOpts = append(serverOpts, serverCreds)

		var unaryInterceptors []grpc.UnaryServerInterceptor
		var streamInterceptors []grpc.StreamServerInterceptor

		if macaroonService != nil {
			unaryInterceptors = append(unaryInterceptors, macaroonService.UnaryServerInterceptor())
			streamInterceptors = append(streamInterceptors, macaroonService.StreamServerInterceptor())
		}

		if len(unaryInterceptors) != 0 || len(streamInterceptors) != 0 {
			chainedUnary := grpcMiddleware.WithUnaryServerChain(unaryInterceptors...)
			chainedStream := grpcMiddleware.WithStreamServerChain(streamInterceptors...)

			serverOpts = append(serverOpts, chainedUnary, chainedStream)
		}

		grpcServer := grpc.NewServer(serverOpts...)
		boltzrpc.RegisterBoltzServer(grpcServer, &routedBoltzServer{
			symbol:      symbol,
			chainParams: chainParams,

			lnd:      lnd,
			boltz:    boltz,
			nursery:  nursery,
			database: database,
		})

		rpcUrl := server.Host + ":" + strconv.Itoa(server.Port)

		// Because the RPC and REST servers are blocking, they are started Go routines

		go func() {
			logger.Info("Starting RPC server on: " + rpcUrl)

			listener, err := net.Listen("tcp", rpcUrl)

			if err != nil {
				errChannel <- err
				return
			}

			errChannel <- grpcServer.Serve(listener)
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

				sanitizedRpcUrl := rpcUrl

				if server.Host == "0.0.0.0" {
					sanitizedRpcUrl = "127.0.0.1:" + strconv.Itoa(server.Port)
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
	}()

	return errChannel
}

// TODO: create readonly macaroon
func (server *RpcServer) generateMacaroons(database *database.Database) (*macaroons.Service, error) {
	logger.Info("Enabling Macaroon authentication")

	macaroonService := macaroons.Service{
		Database: database,
	}

	macaroonService.Init()

	if utils.FileExists(server.AdminMacaroonPath) {
		return &macaroonService, nil
	}

	logger.Warning("Could not find Macaroons")
	logger.Info("Generating new Macaroons")

	adminMacaroon, err := macaroonService.NewMacaroon(macaroons.AdminPermissions()...)

	if err != nil {
		return nil, err
	}

	adminMacaroonByte, err := adminMacaroon.M().MarshalBinary()

	if err != nil {
		return nil, err
	}

	return &macaroonService, ioutil.WriteFile(server.AdminMacaroonPath, adminMacaroonByte, 0600)
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
