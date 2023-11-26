package client

import (
	"context"

	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/golang/protobuf/ptypes/empty"
)

type Boltz struct {
	Client boltzrpc.BoltzClient
	Ctx    context.Context
}

func NewBoltzClient(conn Connection) Boltz {
	return Boltz{
		Ctx:    conn.Ctx,
		Client: boltzrpc.NewBoltzClient(conn.ClientConn),
	}
}

func (boltz *Boltz) GetInfo() (*boltzrpc.GetInfoResponse, error) {
	return boltz.Client.GetInfo(boltz.Ctx, &boltzrpc.GetInfoRequest{})
}

func (boltz *Boltz) GetServiceInfo(pair string) (*boltzrpc.GetServiceInfoResponse, error) {
	return boltz.Client.GetServiceInfo(boltz.Ctx, &boltzrpc.GetServiceInfoRequest{
		PairId: pair,
	})
}

func (boltz *Boltz) ListSwaps() (*boltzrpc.ListSwapsResponse, error) {
	return boltz.Client.ListSwaps(boltz.Ctx, &boltzrpc.ListSwapsRequest{})
}

func (boltz *Boltz) GetSwapInfo(id string) (*boltzrpc.GetSwapInfoResponse, error) {
	return boltz.Client.GetSwapInfo(boltz.Ctx, &boltzrpc.GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *Boltz) GetSwapInfoStream(id string) (boltzrpc.Boltz_GetSwapInfoStreamClient, error) {
	return boltz.Client.GetSwapInfoStream(boltz.Ctx, &boltzrpc.GetSwapInfoRequest{
		Id: id,
	})
}

func (boltz *Boltz) CreateSwap(request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	return boltz.Client.CreateSwap(boltz.Ctx, request)
}

func (boltz *Boltz) CreateReverseSwap(amount int64, address string, acceptZeroConf bool, pairId string) (*boltzrpc.CreateReverseSwapResponse, error) {
	return boltz.Client.CreateReverseSwap(boltz.Ctx, &boltzrpc.CreateReverseSwapRequest{
		Address:        address,
		Amount:         amount,
		AcceptZeroConf: acceptZeroConf,
		PairId:         pairId,
	})
}

func (boltz *Boltz) GetLiquidWalletInfo() (*boltzrpc.LiquidWalletInfo, error) {
	return boltz.Client.GetLiquidWalletInfo(boltz.Ctx, &boltzrpc.GetLiquidWalletInfoRequest{})
}

func (boltz *Boltz) ImportLiquidWallet(mnemonic string) (*boltzrpc.ImportLiquidWalletResponse, error) {
	return boltz.Client.ImportLiquidWallet(boltz.Ctx, &boltzrpc.ImportLiquidWalletRequest{Mnemonic: mnemonic})
}

func (boltz *Boltz) SetLiquidSubaccount(subaccount *uint64) (*boltzrpc.LiquidWalletInfo, error) {
	return boltz.Client.SetLiquidSubaccount(boltz.Ctx, &boltzrpc.SetLiquidSubaccountRequest{Subaccount: subaccount})
}

func (boltz *Boltz) GetLiquidSubaccounts() (*boltzrpc.GetLiquidSubaccountsResponse, error) {
	return boltz.Client.GetLiquidSubaccounts(boltz.Ctx, &boltzrpc.GetLiquidSubaccountsRequest{})
}

func (boltz *Boltz) CreateLiquidWallet() (*boltzrpc.LiquidWalletMnemonic, error) {
	return boltz.Client.CreateLiquidWallet(boltz.Ctx, &boltzrpc.CreateLiquidWalletRequest{})
}

func (boltz *Boltz) GetLiquidWalletMnemonic() (*boltzrpc.LiquidWalletMnemonic, error) {
	return boltz.Client.GetLiquidWalletMnemonic(boltz.Ctx, &boltzrpc.GetLiquidWalletMnemonicRequest{})
}

func (boltz *Boltz) RemoveLiquidWallet() (*boltzrpc.RemoveLiquidWalletResponse, error) {
	return boltz.Client.RemoveLiquidWallet(boltz.Ctx, &boltzrpc.RemoveLiquidWalletRequest{})
}

func (boltz *Boltz) Stop() error {
	_, err := boltz.Client.Stop(boltz.Ctx, &empty.Empty{})
	return err
}
