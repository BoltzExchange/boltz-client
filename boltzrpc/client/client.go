package client

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/golang/protobuf/ptypes/empty"
)

type Boltz struct {
	Connection
	Client boltzrpc.BoltzClient
}

func NewBoltzClient(conn Connection) Boltz {
	return Boltz{
		Connection: conn,
		Client:     boltzrpc.NewBoltzClient(conn.ClientConn),
	}
}

func (boltz *Boltz) GetInfo() (*boltzrpc.GetInfoResponse, error) {
	return boltz.Client.GetInfo(boltz.Ctx, &boltzrpc.GetInfoRequest{})
}

func (boltz *Boltz) GetPairs() (*boltzrpc.GetPairsResponse, error) {
	return boltz.Client.GetPairs(boltz.Ctx, &empty.Empty{})
}

func (boltz *Boltz) GetPairInfo(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	return boltz.Client.GetPairInfo(boltz.Ctx, &boltzrpc.GetPairInfoRequest{Pair: pair, Type: swapType})
}

func (boltz *Boltz) ListSwaps(request *boltzrpc.ListSwapsRequest) (*boltzrpc.ListSwapsResponse, error) {
	return boltz.Client.ListSwaps(boltz.Ctx, request)
}

func (boltz *Boltz) RefundSwap(id string, address string) (*boltzrpc.GetSwapInfoResponse, error) {
	return boltz.Client.RefundSwap(boltz.Ctx, &boltzrpc.RefundSwapRequest{Id: id, Address: address})
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

func (boltz *Boltz) CreateChainSwap(request *boltzrpc.CreateChainSwapRequest) (*boltzrpc.ChainSwapInfo, error) {
	return boltz.Client.CreateChainSwap(boltz.Ctx, request)
}

func (boltz *Boltz) GetWallet(name string) (*boltzrpc.Wallet, error) {
	return boltz.Client.GetWallet(boltz.Ctx, &boltzrpc.GetWalletRequest{Name: &name})
}

func (boltz *Boltz) GetWalletById(id uint64) (*boltzrpc.Wallet, error) {
	return boltz.Client.GetWallet(boltz.Ctx, &boltzrpc.GetWalletRequest{Id: &id})
}

func (boltz *Boltz) GetWallets(currency *boltzrpc.Currency, includeReadonly bool) (*boltzrpc.Wallets, error) {
	return boltz.Client.GetWallets(boltz.Ctx, &boltzrpc.GetWalletsRequest{Currency: currency, IncludeReadonly: &includeReadonly})
}

func (boltz *Boltz) ImportWallet(params *boltzrpc.WalletParams, credentials *boltzrpc.WalletCredentials) (*boltzrpc.Wallet, error) {
	return boltz.Client.ImportWallet(boltz.Ctx, &boltzrpc.ImportWalletRequest{Params: params, Credentials: credentials})
}

func (boltz *Boltz) SetSubaccount(walletId uint64, subaccount *uint64) (*boltzrpc.Subaccount, error) {
	return boltz.Client.SetSubaccount(boltz.Ctx, &boltzrpc.SetSubaccountRequest{Subaccount: subaccount, WalletId: walletId})
}

func (boltz *Boltz) GetSubaccounts(walletId uint64) (*boltzrpc.GetSubaccountsResponse, error) {
	return boltz.Client.GetSubaccounts(boltz.Ctx, &boltzrpc.GetSubaccountsRequest{WalletId: walletId})
}

func (boltz *Boltz) CreateWallet(params *boltzrpc.WalletParams) (*boltzrpc.CreateWalletResponse, error) {
	return boltz.Client.CreateWallet(boltz.Ctx, &boltzrpc.CreateWalletRequest{
		Params: params,
	})
}

func (boltz *Boltz) GetWalletCredentials(id uint64, password *string) (*boltzrpc.WalletCredentials, error) {
	return boltz.Client.GetWalletCredentials(boltz.Ctx, &boltzrpc.GetWalletCredentialsRequest{Id: id, Password: password})
}

func (boltz *Boltz) RemoveWallet(id uint64) (*boltzrpc.RemoveWalletResponse, error) {
	return boltz.Client.RemoveWallet(boltz.Ctx, &boltzrpc.RemoveWalletRequest{Id: id})
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
	if err != nil {
		return false, err
	}
	return response.Correct, nil
}

func (boltz *Boltz) HasPassword() (bool, error) {
	correct, err := boltz.VerifyWalletPassword("")
	return !correct, err
}

func (boltz *Boltz) ChangeWalletPassword(old string, new string) error {
	_, err := boltz.Client.ChangeWalletPassword(boltz.Ctx, &boltzrpc.ChangeWalletPasswordRequest{Old: old, New: new})
	return err
}

func (boltz *Boltz) CreateEntity(name string) (*boltzrpc.Entity, error) {
	return boltz.Client.CreateEntity(boltz.Ctx, &boltzrpc.CreateEntityRequest{Name: name})
}

func (boltz *Boltz) GetEntity(name string) (*boltzrpc.Entity, error) {
	return boltz.Client.GetEntity(boltz.Ctx, &boltzrpc.GetEntityRequest{Name: name})
}

func (boltz *Boltz) ListEntities() (*boltzrpc.ListEntitiesResponse, error) {
	return boltz.Client.ListEntities(boltz.Ctx, &boltzrpc.ListEntitiesRequest{})
}

func (boltz *Boltz) BakeMacaroon(request *boltzrpc.BakeMacaroonRequest) (*boltzrpc.BakeMacaroonResponse, error) {
	return boltz.Client.BakeMacaroon(boltz.Ctx, request)
}
