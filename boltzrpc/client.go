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

	Client BoltzClient
	Ctx    context.Context
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

	boltz.Client = NewBoltzClient(con)

	if boltz.Ctx == nil {
		boltz.Ctx = context.Background()

		if !boltz.NoMacaroons {
			macaroonFile, err := os.ReadFile(boltz.MacaroonPath)

			if err != nil {
				return errors.New(fmt.Sprint("could not read Boltz macaroon: ", err))
			}

			macaroon := metadata.Pairs("macaroon", hex.EncodeToString(macaroonFile))
			boltz.Ctx = metadata.NewOutgoingContext(boltz.Ctx, macaroon)
		}
	}

	return nil
}

func (boltz *Boltz) GetInfo() (*GetInfoResponse, error) {
	return boltz.Client.GetInfo(boltz.Ctx, &GetInfoRequest{})
}

func (boltz *Boltz) GetServiceInfo() (*GetServiceInfoResponse, error) {
	return boltz.Client.GetServiceInfo(boltz.Ctx, &GetServiceInfoRequest{})
}

func (boltz *Boltz) ListSwaps() (*ListSwapsResponse, error) {
	return boltz.Client.ListSwaps(boltz.Ctx, &ListSwapsRequest{})
}

func (boltz *Boltz) GetSwapInfo(id string) (*GetSwapInfoResponse, error) {
	return boltz.Client.GetSwapInfo(boltz.Ctx, &GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *Boltz) Deposit(inboundLiquidity uint) (*DepositResponse, error) {
	return boltz.Client.Deposit(boltz.Ctx, &DepositRequest{
		InboundLiquidity: uint32(inboundLiquidity),
	})
}

func (boltz *Boltz) CreateSwap(amount int64) (*CreateSwapResponse, error) {
	return boltz.Client.CreateSwap(boltz.Ctx, &CreateSwapRequest{
		Amount: amount,
	})
}

func (boltz *Boltz) CreateChannelCreation(amount int64, inboundLiquidity uint32, private bool) (*CreateSwapResponse, error) {
	return boltz.Client.CreateChannel(boltz.Ctx, &CreateChannelRequest{
		Amount:           amount,
		InboundLiquidity: inboundLiquidity,
		Private:          private,
	})
}

func (boltz *Boltz) CreateReverseSwap(amount int64, address string, acceptZeroConf bool) (*CreateReverseSwapResponse, error) {
	return boltz.Client.CreateReverseSwap(boltz.Ctx, &CreateReverseSwapRequest{
		Address:        address,
		Amount:         amount,
		AcceptZeroConf: acceptZeroConf,
	})
}
