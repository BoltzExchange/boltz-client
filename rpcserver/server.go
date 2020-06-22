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
	"net"
	"strconv"
)

type RpcServer struct {
	Host string `long:"rpc.host" description:"gRPC host to which Boltz should listen"`
	Port int    `long:"rpc.port" short:"p" description:"gRPC port to which Boltz should listen"`
}

// TODO: support TLS
func (server *RpcServer) Start(symbol string, chainParams *chaincfg.Params, lnd *lnd.LND, boltz *boltz.Boltz, nursery *nursery.Nursery, database *database.Database) error {
	rpcUrl := server.Host + ":" + strconv.Itoa(server.Port)

	logger.Info("Starting RPC server on: " + rpcUrl)

	listener, err := net.Listen("tcp", rpcUrl)

	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
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
