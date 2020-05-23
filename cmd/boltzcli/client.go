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

func (boltz *boltz) GetSwapInfo(id string) (*boltzrpc.GetSwapInfoResponse, error) {
	return boltz.client.GetSwapInfo(boltz.ctx, &boltzrpc.GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *boltz) CreateSwap(amount string) (*boltzrpc.CreateSwapResponse, error) {
	parsedAmount, err := strconv.ParseInt(amount, 0, 64)

	if err != nil {
		return nil, err
	}

	return boltz.client.CreateSwap(boltz.ctx, &boltzrpc.CreateSwapRequest{
		Amount: parsedAmount,
	})
}
