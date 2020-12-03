package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"strconv"
)

type boltz struct {
	Host string
	Port int

	TlsCertPath string

	NoMacaroons  bool
	MacaroonPath string

	ctx    context.Context
	client boltzrpc.BoltzClient
}

func (boltz *boltz) Connect() error {
	creds, err := credentials.NewClientTLSFromFile(boltz.TlsCertPath, "")

	if err != nil {
		return errors.New(fmt.Sprint("could not read Boltz certificate: ", err))
	}

	con, err := grpc.Dial(boltz.Host+":"+strconv.Itoa(boltz.Port), grpc.WithTransportCredentials(creds))

	if err != nil {
		return err
	}

	boltz.client = boltzrpc.NewBoltzClient(con)

	if boltz.ctx == nil {
		boltz.ctx = context.Background()

		if !boltz.NoMacaroons {
			macaroonFile, err := ioutil.ReadFile(boltz.MacaroonPath)

			if err != nil {
				return errors.New(fmt.Sprint("could not read Boltz macaroon: ", err))
			}

			macaroon := metadata.Pairs("macaroon", hex.EncodeToString(macaroonFile))
			boltz.ctx = metadata.NewOutgoingContext(boltz.ctx, macaroon)
		}
	}

	return nil
}

func (boltz *boltz) GetInfo() (*boltzrpc.GetInfoResponse, error) {
	return boltz.client.GetInfo(boltz.ctx, &boltzrpc.GetInfoRequest{})
}

func (boltz *boltz) GetServiceInfo() (*boltzrpc.GetServiceInfoResponse, error) {
	return boltz.client.GetServiceInfo(boltz.ctx, &boltzrpc.GetServiceInfoRequest{})
}

func (boltz *boltz) ListSwaps() (*boltzrpc.ListSwapsResponse, error) {
	return boltz.client.ListSwaps(boltz.ctx, &boltzrpc.ListSwapsRequest{})
}

func (boltz *boltz) GetSwapInfo(id string) (*boltzrpc.GetSwapInfoResponse, error) {
	return boltz.client.GetSwapInfo(boltz.ctx, &boltzrpc.GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *boltz) Deposit(inboundLiquidity uint) (*boltzrpc.DepositResponse, error) {
	return boltz.client.Deposit(boltz.ctx, &boltzrpc.DepositRequest{
		InboundLiquidity: uint32(inboundLiquidity),
	})
}

func (boltz *boltz) CreateSwap(amount int64) (*boltzrpc.CreateSwapResponse, error) {
	return boltz.client.CreateSwap(boltz.ctx, &boltzrpc.CreateSwapRequest{
		Amount: amount,
	})
}

func (boltz *boltz) CreateChannelCreation(amount int64, inboundLiquidity uint32, private bool) (*boltzrpc.CreateSwapResponse, error) {
	return boltz.client.CreateChannel(boltz.ctx, &boltzrpc.CreateChannelRequest{
		Amount:           amount,
		InboundLiquidity: inboundLiquidity,
		Private:          private,
	})
}

func (boltz *boltz) CreateReverseSwap(amount int64, address string, acceptZeroConf bool) (*boltzrpc.CreateReverseSwapResponse, error) {
	return boltz.client.CreateReverseSwap(boltz.ctx, &boltzrpc.CreateReverseSwapRequest{
		Address:        address,
		Amount:         amount,
		AcceptZeroConf: acceptZeroConf,
	})
}
