package boltzrpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type Boltz struct {
	Host string
	Port int

	TlsCertPath string

	NoMacaroons  bool
	MacaroonPath string

	ctx    context.Context
	client BoltzClient
}

func (boltz *Boltz) Connect() error {
	creds, err := credentials.NewClientTLSFromFile(boltz.TlsCertPath, "")

	if err != nil {
		return errors.New(fmt.Sprint("could not read Boltz certificate: ", err))
	}

	con, err := grpc.Dial(boltz.Host+":"+strconv.Itoa(boltz.Port), grpc.WithTransportCredentials(creds))

	if err != nil {
		return err
	}

	boltz.client = NewBoltzClient(con)

	if boltz.ctx == nil {
		boltz.ctx = context.Background()

		if !boltz.NoMacaroons {
			macaroonFile, err := os.ReadFile(boltz.MacaroonPath)

			if err != nil {
				return errors.New(fmt.Sprint("could not read Boltz macaroon: ", err))
			}

			macaroon := metadata.Pairs("macaroon", hex.EncodeToString(macaroonFile))
			boltz.ctx = metadata.NewOutgoingContext(boltz.ctx, macaroon)
		}
	}

	return nil
}

func (boltz *Boltz) GetInfo() (*GetInfoResponse, error) {
	return boltz.client.GetInfo(boltz.ctx, &GetInfoRequest{})
}

func (boltz *Boltz) GetServiceInfo() (*GetServiceInfoResponse, error) {
	return boltz.client.GetServiceInfo(boltz.ctx, &GetServiceInfoRequest{})
}

func (boltz *Boltz) ListSwaps() (*ListSwapsResponse, error) {
	return boltz.client.ListSwaps(boltz.ctx, &ListSwapsRequest{})
}

func (boltz *Boltz) GetSwapInfo(id string) (*GetSwapInfoResponse, error) {
	return boltz.client.GetSwapInfo(boltz.ctx, &GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *Boltz) Deposit(inboundLiquidity uint) (*DepositResponse, error) {
	return boltz.client.Deposit(boltz.ctx, &DepositRequest{
		InboundLiquidity: uint32(inboundLiquidity),
	})
}

func (boltz *Boltz) CreateSwap(amount int64) (*CreateSwapResponse, error) {
	return boltz.client.CreateSwap(boltz.ctx, &CreateSwapRequest{
		Amount: amount,
	})
}

func (boltz *Boltz) CreateChannelCreation(amount int64, inboundLiquidity uint32, private bool) (*CreateSwapResponse, error) {
	return boltz.client.CreateChannel(boltz.ctx, &CreateChannelRequest{
		Amount:           amount,
		InboundLiquidity: inboundLiquidity,
		Private:          private,
	})
}

func (boltz *Boltz) CreateReverseSwap(amount int64, address string, acceptZeroConf bool) (*CreateReverseSwapResponse, error) {
	return boltz.client.CreateReverseSwap(boltz.ctx, &CreateReverseSwapRequest{
		Address:        address,
		Amount:         amount,
		AcceptZeroConf: acceptZeroConf,
	})
}
