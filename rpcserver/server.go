package rpcserver

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
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
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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
	boltzApi *boltz.Api,
	database *database.Database,
	onchain *onchain.Onchain,
	autoSwapConfigPath string,
) error {
	var serverOpts []grpc.ServerOption
	var err error

	server.Stop = make(chan bool)
	routedServer := &routedBoltzServer{
		network: network,

		lightning: lightning,
		boltz:     boltzApi,
		database:  database,
		onchain:   onchain,

		stop:    server.Stop,
		locked:  true,
		syncing: false,
	}

	swapper := &autoswap.AutoSwapper{
		GetPairInfo: func(pair *boltzrpc.Pair, swapType boltz.SwapType) (*autoswap.PairInfo, error) {
			if swapType == boltz.NormalSwap {
				pair, err := routedServer.getSubmarinePair(pair)
				if err != nil {
					return nil, err
				}
				return &autoswap.PairInfo{
					Limits: autoswap.Limits{
						MinAmount: pair.Limits.Minimal,
						MaxAmount: pair.Limits.Maximal,
					},
					PercentageFee: utils.Percentage(pair.Fees.Percentage),
					OnchainFee:    pair.Fees.MinerFees,
				}, nil
			} else if swapType == boltz.ReverseSwap {
				pair, err := routedServer.getReversePair(pair)
				if err != nil {
					return nil, err
				}
				return &autoswap.PairInfo{
					Limits: autoswap.Limits{
						MinAmount: pair.Limits.Minimal,
						MaxAmount: pair.Limits.Maximal,
					},
					PercentageFee: utils.Percentage(pair.Fees.Percentage),
					OnchainFee:    pair.Fees.MinerFees.Claim + pair.Fees.MinerFees.Lockup,
				}, nil
			}

			return nil, errors.New("invalid swap type")
		},
	}
	if lightning != nil {
		swapper.ExecuteSwap = func(request *boltzrpc.CreateSwapRequest) error {
			ctx, err := routedServer.macaroon.AddEntityToContext(context.Background(), "")
			if err != nil {
				return err
			}
			_, err = routedServer.createSwap(ctx, true, request)
			return err
		}
		swapper.ExecuteReverseSwap = func(request *boltzrpc.CreateReverseSwapRequest) error {
			ctx, err := routedServer.macaroon.AddEntityToContext(context.Background(), "")
			if err != nil {
				return err
			}
			_, err = routedServer.createReverseSwap(ctx, true, request)
			return err
		}
		swapper.ListChannels = routedServer.lightning.ListChannels
	}
	swapper.Init(database, onchain, autoSwapConfigPath)
	routedServer.swapper = swapper

	routedAutoSwapServer := &routedAutoSwapServer{
		database: database,
		swapper:  swapper,
	}

	if server.NoTls {
		// cleanup previous certificates to avoid confusion
		if err := os.Remove(server.TlsCertPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Remove(server.TlsKeyPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		certData, err := loadCertificate(server.TlsCertPath, server.TlsKeyPath, false)

		if err != nil {
			return err
		}
		serverCreds := grpc.Creds(credentials.NewTLS(certData))

		serverOpts = append(serverOpts, serverCreds)
	}
	if !server.NoMacaroons {
		routedServer.macaroon, err = server.generateMacaroons(database)

		if err != nil {
			return err
		}
	} else {
		logger.Warn("Disabled Macaroon authentication")
	}

	unaryInterceptors := []grpc.UnaryServerInterceptor{routedServer.UnaryServerInterceptor()}
	streamInterceptors := []grpc.StreamServerInterceptor{routedServer.StreamServerInterceptor()}

	if routedServer.macaroon != nil {
		unaryInterceptors = append(unaryInterceptors, routedServer.macaroon.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, routedServer.macaroon.StreamServerInterceptor())
	}

	if len(unaryInterceptors) != 0 || len(streamInterceptors) != 0 {
		chainedUnary := grpc.ChainUnaryInterceptor(unaryInterceptors...)
		chainedStream := grpc.ChainStreamInterceptor(streamInterceptors...)

		serverOpts = append(serverOpts, chainedUnary, chainedStream)
	}

	server.Grpc = grpc.NewServer(serverOpts...)
	boltzrpc.RegisterBoltzServer(server.Grpc, routedServer)

	autoswaprpc.RegisterAutoSwapServer(server.Grpc, routedAutoSwapServer)

	if err = routedServer.unlock(""); err != nil {
		if status.Code(err) == codes.InvalidArgument {
			logger.Infof("Server is locked")
		} else {
			return err
		}
	}

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

			creds := insecure.NewCredentials()
			var err error
			if !server.NoTls {
				creds, err = credentials.NewClientTLSFromFile(server.TlsCertPath, "")
				if err != nil {
					errChannel <- err
					return
				}
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
				[]grpc.DialOption{grpc.WithTransportCredentials(creds)},
			)
			if err != nil {
				errChannel <- err
				return
			}

			httpServer = &http.Server{Addr: restUrl, Handler: mux}

			c := cors.AllowAll()
			httpServer.Handler = c.Handler(httpServer.Handler)

			if server.NoTls {
				err = httpServer.ListenAndServe()
			} else {
				err = httpServer.ListenAndServeTLS(server.TlsCertPath, server.TlsKeyPath)
			}
			if err != nil && err.Error() != "http: Server closed" {
				errChannel <- err
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
