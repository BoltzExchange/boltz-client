package rpcserver

import (
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/chaincfg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"strconv"
)

type RpcServer struct {
	Host string `long:"rpc.host" description:"gRPC host to which Boltz should listen"`
	Port int    `long:"rpc.port" short:"p" description:"gRPC port to which Boltz should listen"`

	TlsCertPath string `long:"rpc.tlscert" description:"Path to the TLS certificate of boltz-lnd"`
	TlsKeyPath  string `long:"rpc.tlskey" description:"Path to the TLS private key of boltz-lnd"`
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

	certData, err := loadCertificate(server.TlsCertPath, server.TlsKeyPath)

	if err != nil {
		return err
	}

	logger.Info("Starting RPC server on: " + rpcUrl)

	serverCreds := credentials.NewTLS(certData)
	listener, err := net.Listen("tcp", rpcUrl)

	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	boltzrpc.RegisterBoltzServer(grpcServer, &routedBoltzServer{
		symbol:      symbol,
		chainParams: chainParams,

		lnd:      lnd,
		boltz:    boltz,
		nursery:  nursery,
		database: database,
	})

	return grpcServer.Serve(listener)
}
