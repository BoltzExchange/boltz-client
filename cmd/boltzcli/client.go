package main

import (
	"context"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"google.golang.org/grpc"
	"strconv"
)

type boltz struct {
	Host string `long:"boltz.host" description:"gRPC host of Boltz"`
	Port int    `long:"boltz.port" description:"gRPC port of Boltz"`

	ctx    context.Context
	client boltzrpc.BoltzClient
}

func (boltz *boltz) Connect() error {
	con, err := grpc.Dial(boltz.Host+":"+strconv.Itoa(boltz.Port), grpc.WithInsecure())

	if err != nil {
		return err
	}

	boltz.client = boltzrpc.NewBoltzClient(con)

	if boltz.ctx == nil {
		boltz.ctx = context.Background()
	}

	return nil
}

func (boltz *boltz) GetInfo() (*boltzrpc.GetInfoResponse, error) {
	return boltz.client.GetInfo(boltz.ctx, &boltzrpc.GetInfoRequest{})
}

func (boltz *boltz) GetServiceInfo() (*boltzrpc.GetServiceInfoResponse, error) {
	return boltz.client.GetServiceInfo(boltz.ctx, &boltzrpc.GetServiceInfoRequest{})
}

func (boltz *boltz) GetSwapInfo(id string) (*boltzrpc.GetSwapInfoResponse, error) {
	return boltz.client.GetSwapInfo(boltz.ctx, &boltzrpc.GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *boltz) Deposit() (*boltzrpc.DepositResponse, error) {
	return boltz.client.Deposit(boltz.ctx, &boltzrpc.DepositRequest{})
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
