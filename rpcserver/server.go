package rpcserver

import (
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"net"
	"strconv"
)

type RpcServer struct {
	Host string `long:"rpc.host" description:"gRPC host to which Boltz should listen"`
	Port int    `long:"rpc.port" short:"p" description:"gRPC port to which Boltz should listen"`

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
) error {
	rpcUrl := server.Host + ":" + strconv.Itoa(server.Port)

	certData, err := loadCertificate(server.TlsCertPath, server.TlsKeyPath, true)

	if err != nil {
		return err
	}

	var macaroonService *macaroons.Service

	if !server.NoMacaroons {
		macaroonService, err = server.generateMacaroons(database)

		if err != nil {
			return err
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

	logger.Info("Starting RPC server on: " + rpcUrl)

	listener, err := net.Listen("tcp", rpcUrl)

	if err != nil {
		return err
	}

	return grpcServer.Serve(listener)
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
