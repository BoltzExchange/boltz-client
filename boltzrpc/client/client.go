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

func (boltz *Boltz) GetSubmarinePair(pair *boltzrpc.Pair) (*boltzrpc.SubmarinePair, error) {
	return boltz.Client.GetSubmarinePair(boltz.Ctx, pair)
}

func (boltz *Boltz) GetReversePair(pair *boltzrpc.Pair) (*boltzrpc.ReversePair, error) {
	return boltz.Client.GetReversePair(boltz.Ctx, pair)
}

func (boltz *Boltz) ListSwaps(request *boltzrpc.ListSwapsRequest) (*boltzrpc.ListSwapsResponse, error) {
	return boltz.Client.ListSwaps(boltz.Ctx, request)
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

func (boltz *Boltz) CreateReverseSwap(request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	return boltz.Client.CreateReverseSwap(boltz.Ctx, request)
}
func (boltz *Boltz) GetWallet(name string) (*boltzrpc.Wallet, error) {
	return boltz.Client.GetWallet(boltz.Ctx, &boltzrpc.GetWalletRequest{Name: name})
}

func (boltz *Boltz) GetWallets(currency string, includeReadonly bool) (*boltzrpc.Wallets, error) {
	return boltz.Client.GetWallets(boltz.Ctx, &boltzrpc.GetWalletsRequest{Currency: &currency, IncludeReadonly: &includeReadonly})
}

func (boltz *Boltz) ImportWallet(info *boltzrpc.WalletInfo, credentials *boltzrpc.WalletCredentials, password string) (*boltzrpc.Wallet, error) {
	return boltz.Client.ImportWallet(boltz.Ctx, &boltzrpc.ImportWalletRequest{Info: info, Credentials: credentials, Password: &password})
}

func (boltz *Boltz) SetSubaccount(name string, subaccount *uint64) (*boltzrpc.Subaccount, error) {
	return boltz.Client.SetSubaccount(boltz.Ctx, &boltzrpc.SetSubaccountRequest{Subaccount: subaccount, Name: name})
}

func (boltz *Boltz) GetSubaccounts(info *boltzrpc.WalletInfo) (*boltzrpc.GetSubaccountsResponse, error) {
	return boltz.Client.GetSubaccounts(boltz.Ctx, info)
}

func (boltz *Boltz) CreateWallet(info *boltzrpc.WalletInfo, password string) (*boltzrpc.WalletCredentials, error) {
	return boltz.Client.CreateWallet(boltz.Ctx, &boltzrpc.CreateWalletRequest{
		Info:     info,
		Password: &password,
	})
}

func (boltz *Boltz) GetWalletCredentials(name string, password string) (*boltzrpc.WalletCredentials, error) {
	return boltz.Client.GetWalletCredentials(boltz.Ctx, &boltzrpc.GetWalletCredentialsRequest{Name: name, Password: &password})
}

func (boltz *Boltz) RemoveWallet(name string) (*boltzrpc.RemoveWalletResponse, error) {
	return boltz.Client.RemoveWallet(boltz.Ctx, &boltzrpc.RemoveWalletRequest{Name: name})
}

func (boltz *Boltz) Stop() error {
	_, err := boltz.Client.Stop(boltz.Ctx, &empty.Empty{})
	return err
}

func (boltz *Boltz) Unlock(password string) error {
	_, err := boltz.Client.Unlock(boltz.Ctx, &boltzrpc.UnlockRequest{Password: password})
	return err
}

func (boltz *Boltz) VerifyWalletPassword(password string) (bool, error) {
	response, err := boltz.Client.VerifyWalletPassword(boltz.Ctx, &boltzrpc.VerifyWalletPasswordRequest{Password: password})
	return response.Correct, err
}

func (boltz *Boltz) HasPassword() (bool, error) {
	correct, err := boltz.VerifyWalletPassword("")
	return !correct, err
}

func (boltz *Boltz) ChangeWalletPassword(old string, new string) error {
	_, err := boltz.Client.ChangeWalletPassword(boltz.Ctx, &boltzrpc.ChangeWalletPasswordRequest{Old: old, New: new})
	return err
}
