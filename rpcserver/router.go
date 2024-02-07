package rpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BoltzExchange/boltz-client/build"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/BoltzExchange/boltz-client/onchain/wallet"

	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/nursery"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/zpay32"
)

const referralId = "boltz-client"

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	network *boltz.Network

	onchain   *onchain.Onchain
	lightning lightning.LightningNode
	boltz     *boltz.Boltz
	nursery   *nursery.Nursery
	database  *database.Database
	swapper   *autoswap.AutoSwapper

	stop   chan bool
	locked bool
}

func handleError(err error) error {
	if err != nil {
		logger.Warn("RPC request failed: " + err.Error())
	}

	return err
}

func (server *routedBoltzServer) GetInfo(_ context.Context, _ *boltzrpc.GetInfoRequest) (*boltzrpc.GetInfoResponse, error) {
	lightningInfo, err := server.lightning.GetInfo()

	if err != nil {
		return nil, handleError(err)
	}

	pendingSwaps, err := server.database.QueryPendingSwaps()

	if err != nil {
		return nil, handleError(err)
	}

	var pendingSwapIds []string

	for _, pendingSwap := range pendingSwaps {
		pendingSwapIds = append(pendingSwapIds, pendingSwap.Id)
	}

	pendingReverseSwaps, err := server.database.QueryPendingReverseSwaps()

	if err != nil {
		return nil, handleError(err)
	}

	var pendingReverseSwapIds []string

	for _, pendingReverseSwap := range pendingReverseSwaps {
		pendingReverseSwapIds = append(pendingReverseSwapIds, pendingReverseSwap.Id)
	}

	blockHeights := make(map[string]uint32)

	blockHeights[string(boltz.CurrencyBtc)], err = server.onchain.GetBlockHeight(boltz.CurrencyBtc)
	if err != nil {
		logger.Infof("Failed to get block height for btc: %v", err)
	}
	blockHeights[string(boltz.CurrencyLiquid)], err = server.onchain.GetBlockHeight(boltz.CurrencyLiquid)
	if err != nil {
		logger.Infof("Failed to get block height for liquid: %v", err)
	}

	response := &boltzrpc.GetInfoResponse{
		Version:             build.GetVersion(),
		Node:                server.lightning.Name(),
		Network:             server.network.Name,
		NodePubkey:          lightningInfo.Pubkey,
		BlockHeights:        blockHeights,
		PendingSwaps:        pendingSwapIds,
		PendingReverseSwaps: pendingReverseSwapIds,

		Symbol:      "BTC",
		LndPubkey:   lightningInfo.Pubkey,
		BlockHeight: lightningInfo.BlockHeight,
	}

	if server.swapper.Running() {
		response.AutoSwapStatus = "running"
	} else {
		if server.swapper.Error() != "" {
			response.AutoSwapStatus = "error"
		} else {
			response.AutoSwapStatus = "disabled"
		}
	}

	return response, nil

}

func (server *routedBoltzServer) GetServiceInfo(_ context.Context, request *boltzrpc.GetServiceInfoRequest) (*boltzrpc.GetServiceInfoResponse, error) {
	fees, limits, err := server.getPairs(boltz.PairBtc)

	if err != nil {
		return nil, handleError(err)
	}

	limits.Minimal = calculateDepositLimit(limits.Minimal, fees, true)
	limits.Maximal = calculateDepositLimit(limits.Maximal, fees, false)

	return &boltzrpc.GetServiceInfoResponse{
		Fees:   fees,
		Limits: limits,
	}, nil
}

func ParseCurrency(grpcCurrency boltzrpc.Currency) boltz.Currency {
	if grpcCurrency == boltzrpc.Currency_Btc {
		return boltz.CurrencyBtc
	} else {
		return boltz.CurrencyLiquid
	}
}

func ParsePair(grpcPair *boltzrpc.Pair) (pair boltz.Pair) {
	return boltz.Pair{
		From: ParseCurrency(grpcPair.From),
		To:   ParseCurrency(grpcPair.To),
	}
}

func (server *routedBoltzServer) ListSwaps(_ context.Context, request *boltzrpc.ListSwapsRequest) (*boltzrpc.ListSwapsResponse, error) {
	response := &boltzrpc.ListSwapsResponse{}

	args := database.SwapQuery{
		IsAuto: request.IsAuto,
		State:  request.State,
	}

	if request.From != nil {
		parsed := ParseCurrency(*request.From)
		args.From = &parsed
	}

	if request.To != nil {
		parsed := ParseCurrency(*request.To)
		args.To = &parsed
	}

	swaps, err := server.database.QuerySwaps(args)
	if err != nil {
		return nil, err
	}

	for _, swap := range swaps {
		response.Swaps = append(response.Swaps, serializeSwap(&swap))
	}

	// Reverse Swaps
	reverseSwaps, err := server.database.QueryReverseSwaps(args)

	if err != nil {
		return nil, err
	}

	for _, reverseSwap := range reverseSwaps {
		response.ReverseSwaps = append(response.ReverseSwaps, serializeReverseSwap(&reverseSwap))
	}

	return response, nil
}

func (server *routedBoltzServer) GetSwapInfo(_ context.Context, request *boltzrpc.GetSwapInfoRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	swap, err := server.database.QuerySwap(request.Id)

	if err == nil {
		return &boltzrpc.GetSwapInfoResponse{
			Swap: serializeSwap(swap),
		}, nil
	}

	// Try to find a Reverse Swap with that ID
	reverseSwap, err := server.database.QueryReverseSwap(request.Id)

	if err == nil {
		return &boltzrpc.GetSwapInfoResponse{
			ReverseSwap: serializeReverseSwap(reverseSwap),
		}, nil
	}

	return nil, handleError(errors.New("could not find Swap or Reverse Swap with ID " + request.Id))
}

func (server *routedBoltzServer) GetSwapInfoStream(request *boltzrpc.GetSwapInfoRequest, stream boltzrpc.Boltz_GetSwapInfoStreamServer) error {
	logger.Info("Starting Swap info stream for " + request.Id)
	info, err := server.GetSwapInfo(context.Background(), request)
	if err != nil {
		return handleError(err)
	}

	updates, stop := server.nursery.SwapUpdates(request.Id)
	if updates != nil {
		for update := range updates {
			if err := stream.Send(&boltzrpc.GetSwapInfoResponse{
				Swap:        serializeSwap(update.Swap),
				ReverseSwap: serializeReverseSwap(update.ReverseSwap),
			}); err != nil {
				stop()
				return handleError(err)
			}
		}
	} else {
		if err := stream.Send(info); err != nil {
			return handleError(err)
		}
	}

	return nil
}

func (server *routedBoltzServer) Deposit(_ context.Context, request *boltzrpc.DepositRequest) (*boltzrpc.DepositResponse, error) {
	response, err := server.createSwap(false, &boltzrpc.CreateSwapRequest{
		Pair: &boltzrpc.Pair{
			From: boltzrpc.Currency_Btc,
			To:   boltzrpc.Currency_Btc,
		},
	})
	if err != nil {
		return nil, handleError(err)
	}

	return &boltzrpc.DepositResponse{
		Id:                 response.Id,
		Address:            response.Address,
		TimeoutBlockHeight: response.TimeoutBlockHeight,
	}, nil
}

// TODO: custom refund address
func (server *routedBoltzServer) createSwap(isAuto bool, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	logger.Info("Creating Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	privateKey, publicKey, err := newKeys()
	if err != nil {
		return nil, handleError(err)
	}

	pair := ParsePair(request.Pair)

	submarinePair, err := server.GetSubmarinePair(context.Background(), request.Pair)
	if err != nil {
		return nil, err
	}

	createSwap := boltz.CreateSwapRequest{
		From:            pair.From,
		To:              pair.To,
		PairHash:        submarinePair.Hash,
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
		ReferralId:      referralId,
	}

	var preimage, preimageHash []byte
	if request.Amount != 0 {
		invoice, err := server.lightning.CreateInvoice(request.Amount, nil, 0, utils.GetSwapMemo(string(pair.From)))
		if err != nil {
			return nil, handleError(err)
		}
		preimageHash = invoice.PaymentHash
		createSwap.Invoice = invoice.PaymentRequest
	} else {
		if request.AutoSend {
			return nil, handleError(errors.New("cannot auto send if amount is 0"))
		}
		preimage, preimageHash, err = newPreimage()

		logger.Info("Creating Swap with preimage hash: " + hex.EncodeToString(preimageHash))

		createSwap.PreimageHash = hex.EncodeToString(preimageHash)
		if err != nil {
			return nil, handleError(err)
		}
	}

	wallet, err := server.onchain.GetWallet(request.GetWallet(), pair.From, false)
	if err != nil {
		if request.AutoSend {
			return nil, handleError(err)
		}
		if request.RefundAddress == "" {
			return nil, handleError(fmt.Errorf("refund address is required if wallet is not available: %w", err))
		}
	}

	response, err := server.boltz.CreateSwap(createSwap)

	if err != nil {
		return nil, handleError(errors.New("boltz error: " + err.Error()))
	}

	swap := database.Swap{
		Id:                  response.Id,
		Pair:                pair,
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		PrivateKey:          privateKey,
		Preimage:            preimage,
		Invoice:             createSwap.Invoice,
		Address:             response.Address,
		ExpectedAmount:      response.ExpectedAmount,
		TimoutBlockHeight:   response.TimeoutBlockHeight,
		SwapTree:            response.SwapTree.Deserialize(),
		LockupTransactionId: "",
		RefundTransactionId: "",
		RefundAddress:       request.RefundAddress,
		IsAuto:              isAuto,
		ServiceFeePercent:   utils.Percentage(submarinePair.Fees.Percentage),
		AutoSend:            request.AutoSend,
	}

	swap.ClaimPubKey, err = database.ParsePublicKey(response.ClaimPublicKey)
	if err != nil {
		return nil, handleError(err)
	}

	for _, chanId := range request.ChanIds {
		parsed, err := lightning.NewChanIdFromString(chanId)
		if err != nil {
			return nil, handleError(errors.New("invalid channel id: " + err.Error()))
		}
		swap.ChanIds = append(swap.ChanIds, parsed)
	}

	if pair.From == boltz.CurrencyLiquid {
		swap.BlindingKey, err = database.ParsePrivateKey(response.BlindingKey)

		if err != nil {
			return nil, handleError(err)
		}
	}

	if err := swap.InitTree(); err != nil {
		return nil, handleError(err)
	}

	if err := swap.SwapTree.Check(false, swap.TimoutBlockHeight, preimageHash); err != nil {
		return nil, handleError(err)
	}

	fmt.Println("address" + response.Address)
	if err := swap.SwapTree.CheckAddress(response.Address, server.network, swap.BlindingPubKey()); err != nil {
		return nil, handleError(err)
	}

	logger.Info("Verified redeem script and address of Swap " + swap.Id)

	err = server.database.CreateSwap(swap)
	if err != nil {
		return nil, handleError(err)
	}

	blockHeight, err := server.onchain.GetBlockHeight(pair.From)
	if err != nil {
		return nil, handleError(err)
	}

	timeoutHours := boltz.BlocksToHours(response.TimeoutBlockHeight-blockHeight, pair.From)
	swapResponse := &boltzrpc.CreateSwapResponse{
		Id:                 swap.Id,
		Address:            response.Address,
		ExpectedAmount:     int64(response.ExpectedAmount),
		Bip21:              response.Bip21,
		TimeoutBlockHeight: response.TimeoutBlockHeight,
		TimeoutHours:       float32(timeoutHours),
	}

	if request.AutoSend {
		// TODO: custom block target?
		feeSatPerVbyte, err := server.onchain.EstimateFee(pair.From, 2)
		if err != nil {
			return nil, handleError(err)
		}
		logger.Infof("Paying swap %s with fee of %f sat/vbyte", swap.Id, feeSatPerVbyte)
		txId, err := wallet.SendToAddress(response.Address, response.ExpectedAmount, feeSatPerVbyte)
		if err != nil {
			return nil, handleError(err)
		}
		swapResponse.TxId = txId
	}

	server.nursery.RegisterSwap(swap)

	logger.Info("Created new Swap " + swap.Id + ": " + marshalJson(swap.Serialize()))

	return swapResponse, nil
}

func (server *routedBoltzServer) CreateSwap(_ context.Context, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	return server.createSwap(false, request)
}

func (server *routedBoltzServer) createReverseSwap(isAuto bool, request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	logger.Info("Creating Reverse Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	claimAddress := request.Address

	pair := ParsePair(request.Pair)
	if claimAddress != "" {
		err := boltz.ValidateAddress(server.network, claimAddress, pair.To)

		if err != nil {
			return nil, handleError(fmt.Errorf("Invalid claim address %s: %w", claimAddress, err))
		}
	} else {
		wallet, err := server.onchain.GetWallet(request.GetWallet(), pair.To, true)
		if err != nil {
			return nil, handleError(err)
		}

		claimAddress, err = wallet.NewAddress()
		if err != nil {
			return nil, handleError(err)
		}

		logger.Infof("Got claim address from wallet %v: %v", wallet.Name(), claimAddress)
	}

	preimage, preimageHash, err := newPreimage()

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Generated preimage " + hex.EncodeToString(preimage))

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, handleError(err)
	}

	reversePair, err := server.GetReversePair(context.Background(), request.Pair)
	if err != nil {
		return nil, handleError(err)
	}

	response, err := server.boltz.CreateReverseSwap(boltz.CreateReverseSwapRequest{
		From:           pair.From,
		To:             pair.To,
		PairHash:       reversePair.Hash,
		InvoiceAmount:  uint64(request.Amount),
		PreimageHash:   hex.EncodeToString(preimageHash),
		ClaimPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
		ReferralId:     referralId,
	})
	if err != nil {
		return nil, handleError(err)
	}

	key, err := database.ParsePublicKey(response.RefundPublicKey)
	if err != nil {
		return nil, handleError(err)
	}

	reverseSwap := database.ReverseSwap{
		Id:                  response.Id,
		IsAuto:              isAuto,
		Pair:                pair,
		Status:              boltz.SwapCreated,
		AcceptZeroConf:      request.AcceptZeroConf,
		PrivateKey:          privateKey,
		SwapTree:            response.SwapTree.Deserialize(),
		RefundPubKey:        key,
		Preimage:            preimage,
		Invoice:             response.Invoice,
		ClaimAddress:        claimAddress,
		OnchainAmount:       response.OnchainAmount,
		TimeoutBlockHeight:  response.TimeoutBlockHeight,
		LockupTransactionId: "",
		ClaimTransactionId:  "",
		ServiceFeePercent:   utils.Percentage(reversePair.Fees.Percentage),
	}

	for _, chanId := range request.ChanIds {
		parsed, err := lightning.NewChanIdFromString(chanId)
		if err != nil {
			return nil, handleError(errors.New("invalid channel id: " + err.Error()))
		}
		reverseSwap.ChanIds = append(reverseSwap.ChanIds, parsed)
	}

	var blindingPubKey *btcec.PublicKey
	if reverseSwap.Pair.To == boltz.CurrencyLiquid {
		reverseSwap.BlindingKey, err = database.ParsePrivateKey(response.BlindingKey)
		blindingPubKey = reverseSwap.BlindingKey.PubKey()

		if err != nil {
			return nil, handleError(err)
		}
	}

	if err := reverseSwap.InitTree(); err != nil {
		return nil, handleError(err)
	}

	if err := reverseSwap.SwapTree.Check(true, reverseSwap.TimeoutBlockHeight, preimageHash); err != nil {
		return nil, handleError(err)
	}

	if err := reverseSwap.SwapTree.CheckAddress(response.LockupAddress, server.network, blindingPubKey); err != nil {
		return nil, handleError(err)
	}

	invoice, err := zpay32.Decode(reverseSwap.Invoice, server.network.Btc)

	if err != nil {
		return nil, handleError(err)
	}

	if !bytes.Equal(preimageHash, invoice.PaymentHash[:]) {
		return nil, handleError(errors.New("invalid invoice preimage hash"))
	}

	logger.Info("Verified redeem script and invoice of Reverse Swap " + reverseSwap.Id)

	err = server.database.CreateReverseSwap(reverseSwap)

	if err != nil {
		return nil, handleError(err)
	}

	server.nursery.RegisterReverseSwap(reverseSwap)

	logger.Info("Created new Reverse Swap " + reverseSwap.Id + ": " + marshalJson(reverseSwap.Serialize()))

	if err := server.nursery.PayReverseSwap(&reverseSwap); err != nil {
		if dbErr := server.database.UpdateReverseSwapState(&reverseSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
			return nil, handleError(dbErr)
		}
		return nil, handleError(err)
	}

	return &boltzrpc.CreateReverseSwapResponse{
		Id:            reverseSwap.Id,
		LockupAddress: response.LockupAddress,
	}, nil
}

func (server *routedBoltzServer) CreateReverseSwap(_ context.Context, request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	return server.createReverseSwap(false, request)
}

func (server *routedBoltzServer) onWalletChange() {
	if server.swapper.Enabled() {
		logger.Info("Restarting autoswapper because liquid wallet has changed.")
		if err := server.swapper.Start(); err != nil {
			logger.Errorf("Failed to restart swapper after liquid wallet has changed: %v", err)
		}
	}
}

func (server *routedBoltzServer) importWallet(credentials *wallet.Credentials, password string) error {
	decryptWalletCredentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		return errors.New("wrong password")
	}

	for _, existing := range decryptWalletCredentials {
		if existing.Mnemonic == credentials.Mnemonic && existing.Xpub == credentials.Xpub && existing.CoreDescriptor == credentials.CoreDescriptor {
			return fmt.Errorf("wallet %s has the same credentials", existing.Name)
		}
	}

	wallet, err := wallet.Login(credentials)
	if err != nil {
		return errors.New("could not login: " + err.Error())
	}
	if wallet.Readonly() {
		var subaccount *uint64
		subaccounts, err := wallet.GetSubaccounts(false)
		if err != nil {
			return err
		}
		if len(subaccounts) != 0 {
			subaccount = &subaccounts[0].Pointer
		}
		credentials.Subaccount, err = wallet.SetSubaccount(subaccount)
		if err != nil {
			return err
		}
	}
	decryptWalletCredentials = append(decryptWalletCredentials, credentials)
	if err := server.database.InsertWalletCredentials(credentials); err != nil {
		return err
	}
	if password != "" {
		if err := server.encryptWalletCredentials(password, decryptWalletCredentials); err != nil {
			return fmt.Errorf("could not encrypt credentials: %w", err)
		}
	}
	server.onchain.Wallets = append(server.onchain.Wallets, wallet)
	server.onWalletChange()
	return nil
}

func (server *routedBoltzServer) ImportWallet(context context.Context, request *boltzrpc.ImportWalletRequest) (*boltzrpc.Wallet, error) {
	if err := checkName(request.Info.Name); err != nil {
		return nil, handleError(err)
	}

	currency, err := boltz.ParseCurrency(request.Info.Currency)
	if err != nil {
		return nil, handleError(err)
	}

	credentials := &wallet.Credentials{
		Name:           request.Info.Name,
		Currency:       currency,
		Mnemonic:       request.Credentials.GetMnemonic(),
		Xpub:           request.Credentials.GetXpub(),
		CoreDescriptor: request.Credentials.GetCoreDescriptor(),
		Subaccount:     request.Credentials.Subaccount,
	}

	if err := server.importWallet(credentials, request.GetPassword()); err != nil {
		return nil, handleError(err)
	}
	return server.GetWallet(context, &boltzrpc.GetWalletRequest{Name: request.Info.Name})
}

func (server *routedBoltzServer) SetSubaccount(_ context.Context, request *boltzrpc.SetSubaccountRequest) (*boltzrpc.Subaccount, error) {
	wallet, err := server.getOwnWallet(request.Name, false)
	if err != nil {
		return nil, handleError(err)
	}

	subaccountNumber, err := wallet.SetSubaccount(request.Subaccount)
	if err != nil {
		return nil, handleError(err)
	}

	if err := server.database.SetWalletSubaccount(wallet.Name(), string(wallet.Currency()), *subaccountNumber); err != nil {
		return nil, handleError(err)
	}

	if err := server.swapper.LoadConfig(); err != nil {
		logger.Warnf("Could not load autoswap config: %v", err)
	}

	server.onWalletChange()

	subaccount, err := wallet.GetSubaccount(*subaccountNumber)
	if err != nil {
		return nil, handleError(err)
	}
	balance, err := wallet.GetBalance()
	if err != nil {
		return nil, handleError(err)
	}
	return serializewalletSubaccount(*subaccount, balance), nil
}

func (server *routedBoltzServer) GetSubaccounts(_ context.Context, request *boltzrpc.WalletInfo) (*boltzrpc.GetSubaccountsResponse, error) {
	wallet, err := server.getOwnWallet(request.Name, false)
	if err != nil {
		return nil, handleError(err)
	}

	subaccounts, err := wallet.GetSubaccounts(true)
	if err != nil {
		return nil, handleError(err)
	}

	response := &boltzrpc.GetSubaccountsResponse{}
	for _, subaccount := range subaccounts {
		balance, err := wallet.GetSubaccountBalance(subaccount.Pointer)
		if err != nil {
			logger.Errorf("failed to get balance for subaccount %+v: %v", subaccount, err.Error())
		}
		response.Subaccounts = append(response.Subaccounts, serializewalletSubaccount(*subaccount, balance))
	}

	if subaccount, err := wallet.CurrentSubaccount(); err == nil {
		response.Current = &subaccount
	}
	return response, nil
}

func (server *routedBoltzServer) CreateWallet(ctx context.Context, request *boltzrpc.CreateWalletRequest) (*boltzrpc.WalletCredentials, error) {
	mnemonic, err := wallet.GenerateMnemonic()
	if err != nil {
		return nil, handleError(errors.New("could not generate new mnemonic: " + err.Error()))
	}

	credentials := &boltzrpc.WalletCredentials{
		Mnemonic: &mnemonic,
	}

	if _, err := server.ImportWallet(ctx, &boltzrpc.ImportWalletRequest{
		Info: request.Info,
		Credentials: &boltzrpc.WalletCredentials{
			Mnemonic: &mnemonic,
		},
		Password: request.Password,
	}); err != nil {
		return nil, err
	}

	response, err := server.SetSubaccount(ctx, &boltzrpc.SetSubaccountRequest{
		Name: request.Info.Name,
	})
	if err != nil {
		return nil, err
	}
	credentials.Subaccount = &response.Pointer
	return credentials, nil
}

func (server *routedBoltzServer) serializeWallet(wal onchain.Wallet) (*boltzrpc.Wallet, error) {
	result := &boltzrpc.Wallet{
		Name:     wal.Name(),
		Currency: string(wal.Currency()),
		Readonly: wal.Readonly(),
	}
	balance, err := wal.GetBalance()
	if err != nil {
		if !errors.Is(err, wallet.ErrSubAccountNotSet) {
			return nil, handleError(fmt.Errorf("could not get balance for wallet %s: %w", wal.Name(), err))
		}
	} else {
		result.Balance = serializeWalletBalance(balance)
	}
	return result, nil
}

func (server *routedBoltzServer) GetWallet(_ context.Context, request *boltzrpc.GetWalletRequest) (*boltzrpc.Wallet, error) {
	wallet, err := server.onchain.GetWallet(request.Name, "", true)
	if err != nil {
		return nil, handleError(err)
	}

	return server.serializeWallet(wallet)
}

func (server *routedBoltzServer) GetWallets(_ context.Context, request *boltzrpc.GetWalletsRequest) (*boltzrpc.Wallets, error) {
	var response boltzrpc.Wallets
	currency, _ := boltz.ParseCurrency(request.GetCurrency())
	for _, current := range server.onchain.Wallets {
		if (currency == "" || current.Currency() == currency) && (!current.Readonly() || request.GetIncludeReadonly()) {
			wallet, err := server.serializeWallet(current)
			if err != nil {
				return nil, err
			}
			response.Wallets = append(response.Wallets, wallet)
		}
	}
	return &response, nil
}

func (server *routedBoltzServer) GetWalletCredentials(_ context.Context, request *boltzrpc.GetWalletCredentialsRequest) (*boltzrpc.WalletCredentials, error) {
	creds, err := server.database.GetWalletCredentials(request.Name)
	if err != nil {
		return nil, handleError(fmt.Errorf("could not read credentials for wallet %s: %w", request.Name, err))
	}
	if creds.Encrypted() {
		creds, err = creds.Decrypt(request.GetPassword())
		if err != nil {
			return nil, handleError(fmt.Errorf("invalid password: %w", err))
		}
	}

	return serializeWalletCredentials(creds), err
}

func (server *routedBoltzServer) RemoveWallet(_ context.Context, request *boltzrpc.RemoveWalletRequest) (*boltzrpc.RemoveWalletResponse, error) {
	if err := server.database.DeleteWalletCredentials(request.Name); err != nil {
		return nil, handleError(err)
	}
	cfg, err := server.swapper.GetConfig()
	if err == nil {
		if cfg.Wallet == request.Name {
			return nil, handleError(fmt.Errorf(
				"wallet %s is used in autoswap, configure a different wallet in autoswap before removing this wallet.",
				request.Name,
			))
		}
	}
	wallet, err := server.getOwnWallet(request.Name, true)
	if err != nil {
		return nil, handleError(err)
	}
	if err := wallet.Remove(); err != nil {
		return nil, handleError(err)
	}
	server.onchain.Wallets = slices.DeleteFunc(server.onchain.Wallets, func(current onchain.Wallet) bool {
		return current.Name() == request.Name
	})
	server.onWalletChange()

	logger.Debugf("Removed wallet %s", request.Name)

	return &boltzrpc.RemoveWalletResponse{}, nil
}

func (server *routedBoltzServer) Stop(context.Context, *empty.Empty) (*empty.Empty, error) {
	server.nursery.Stop()
	logger.Debugf("Stopped nursery")
	server.stop <- true
	return &empty.Empty{}, nil
}

func (server *routedBoltzServer) decryptWalletCredentials(password string) (decrypted []*wallet.Credentials, err error) {
	credentials, err := server.database.QueryWalletCredentials()
	if err != nil {
		return nil, err
	}
	for _, creds := range credentials {
		if creds.Encrypted() {
			if creds, err = creds.Decrypt(password); err != nil {
				logger.Debugf("failed to decrypted wallet credentials: %s", err)
				return nil, status.Errorf(codes.InvalidArgument, "wrong password")
			}
		}
		decrypted = append(decrypted, creds)
	}
	return decrypted, nil
}

func (server *routedBoltzServer) encryptWalletCredentials(password string, credentials []*wallet.Credentials) (err error) {
	tx, err := server.database.BeginTx()
	if err != nil {
		return err
	}
	for _, creds := range credentials {
		if password != "" {
			if creds, err = creds.Encrypt(password); err != nil {
				return err
			}
		}
		if err := tx.UpdateWalletCredentials(creds); err != nil {
			return tx.Rollback(err)
		}
	}
	return tx.Commit()
}

func (server *routedBoltzServer) Unlock(_ context.Context, request *boltzrpc.UnlockRequest) (*empty.Empty, error) {
	return &empty.Empty{}, handleError(server.unlock(request.Password))
}

func (server *routedBoltzServer) VerifyWalletPassword(_ context.Context, request *boltzrpc.VerifyWalletPasswordRequest) (*boltzrpc.VerifyWalletPasswordResponse, error) {
	_, err := server.decryptWalletCredentials(request.Password)
	return &boltzrpc.VerifyWalletPasswordResponse{Correct: err == nil}, nil
}

func (server *routedBoltzServer) unlock(password string) error {
	if !server.locked {
		return errors.New("boltzd already unlocked!")
	}

	credentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		return err
	}
	for _, creds := range credentials {
		wallet, err := wallet.Login(creds)
		if err != nil {
			return fmt.Errorf("could not login to wallet: %v", err)
		} else {
			server.onchain.Wallets = append(server.onchain.Wallets, wallet)
		}
	}

	if err := server.swapper.LoadConfig(); err != nil {
		logger.Warnf("Could not load autoswap config: %v", err)
	}
	server.onWalletChange()

	server.nursery = &nursery.Nursery{}
	err = server.nursery.Init(
		server.network,
		server.lightning,
		server.onchain,
		server.boltz,
		server.database,
	)
	if err != nil {
		return err
	}
	server.locked = false

	return nil
}

func (server *routedBoltzServer) ChangeWalletPassword(_ context.Context, request *boltzrpc.ChangeWalletPasswordRequest) (*empty.Empty, error) {
	decrypted, err := server.decryptWalletCredentials(request.Old)
	if err != nil {
		return nil, handleError(err)
	}

	if err := server.encryptWalletCredentials(request.New, decrypted); err != nil {
		return nil, handleError(err)
	}
	return &empty.Empty{}, nil
}

var errLocked = errors.New("boltzd is locked, use \"unlock\" to enable full RPC access")

func (server *routedBoltzServer) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if server.locked && !strings.Contains(info.FullMethod, "Unlock") {
			return nil, handleError(errLocked)
		}

		return handler(ctx, req)
	}
}

func (server *routedBoltzServer) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if server.locked && !strings.Contains(info.FullMethod, "Unlock") {
			return handleError(errLocked)
		}

		return handler(srv, ss)
	}
}

func (server *routedBoltzServer) getOwnWallet(name string, readonly bool) (*wallet.Wallet, error) {
	existing, err := server.onchain.GetWallet(name, "", readonly)
	if err != nil {
		return nil, err
	}
	wallet, ok := existing.(*wallet.Wallet)
	if !ok {
		return nil, fmt.Errorf("wallet %s can not be modified", name)
	}
	return wallet, nil
}

func findPair[T any](pair boltz.Pair, nested map[string]map[string]T) (*T, error) {
	from, hasPair := nested[string(pair.From)]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair from %v", pair)
	}
	result, hasPair := from[string(pair.To)]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair to %v", pair)
	}
	return &result, nil
}

func (server *routedBoltzServer) GetSubmarinePair(ctx context.Context, request *boltzrpc.Pair) (*boltzrpc.SubmarinePair, error) {
	pairsResponse, err := server.boltz.GetSubmarinePairs()
	if err != nil {
		return nil, handleError(err)
	}
	pair, err := findPair(ParsePair(request), pairsResponse)
	if err != nil {
		return nil, handleError(err)
	}

	return &boltzrpc.SubmarinePair{
		Hash: pair.Hash,
		Rate: float32(pair.Rate),
		Fees: &boltzrpc.SubmarinePair_Fees{
			Percentage: float32(pair.Fees.Percentage),
			MinerFees:  pair.Fees.MinerFees,
		},
		Limits: &boltzrpc.Limits{
			Minimal:               pair.Limits.Minimal,
			Maximal:               pair.Limits.Maximal,
			MaximalZeroConfAmount: pair.Limits.MaximalZeroConfAmount,
		},
	}, nil
}

func (server *routedBoltzServer) GetReversePair(ctx context.Context, request *boltzrpc.Pair) (*boltzrpc.ReversePair, error) {
	pairsResponse, err := server.boltz.GetReversePairs()
	if err != nil {
		return nil, err
	}
	pair, err := findPair(ParsePair(request), pairsResponse)
	if err != nil {
		return nil, err
	}

	return &boltzrpc.ReversePair{
		Hash: pair.Hash,
		Rate: float32(pair.Rate),
		Fees: &boltzrpc.ReversePair_Fees{
			Percentage: float32(pair.Fees.Percentage),
			MinerFees: &boltzrpc.ReversePair_Fees_MinerFees{
				Lockup: pair.Fees.MinerFees.Lockup,
				Claim:  pair.Fees.MinerFees.Claim,
			},
		},
		Limits: &boltzrpc.Limits{
			Minimal: pair.Limits.Minimal,
			Maximal: pair.Limits.Maximal,
		},
	}, nil
}

func (server *routedBoltzServer) getPairs(pairId boltz.Pair) (*boltzrpc.Fees, *boltzrpc.Limits, error) {
	pairsResponse, err := server.boltz.GetPairs()

	if err != nil {
		return nil, nil, err
	}

	pair, hasPair := pairsResponse.Pairs[pairId.String()]

	if !hasPair {
		return nil, nil, fmt.Errorf("could not find pair with id %s", pairId)
	}

	minerFees := pair.Fees.MinerFees.BaseAsset

	return &boltzrpc.Fees{
			Percentage: pair.Fees.Percentage,
			Miner: &boltzrpc.MinerFees{
				Normal:  uint32(minerFees.Normal),
				Reverse: uint32(minerFees.Reverse.Lockup + minerFees.Reverse.Claim),
			},
		}, &boltzrpc.Limits{
			Minimal: pair.Limits.Minimal,
			Maximal: pair.Limits.Maximal,
		}, nil
}

func calculateDepositLimit(limit uint64, fees *boltzrpc.Fees, isMin bool) uint64 {
	effectiveRate := 1 + float64(fees.Percentage)/100
	limitFloat := float64(limit) * effectiveRate

	if isMin {
		// Add two more sats as safety buffer
		limitFloat = math.Ceil(limitFloat) + 2
	} else {
		limitFloat = math.Floor(limitFloat)
	}

	return uint64(limitFloat) + uint64(fees.Miner.Normal)
}

func newKeys() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privateKey, err := btcec.NewPrivateKey()

	if err != nil {
		return nil, nil, err
	}

	publicKey := privateKey.PubKey()

	return privateKey, publicKey, err
}

func newPreimage() ([]byte, []byte, error) {
	preimage := make([]byte, 32)
	_, err := rand.Read(preimage)

	if err != nil {
		return nil, nil, err
	}

	preimageHash := sha256.Sum256(preimage)

	return preimage, preimageHash[:], nil
}

func marshalJson(data interface{}) string {
	marshalled, _ := json.MarshalIndent(data, "", "  ")
	return string(marshalled)
}

func checkName(name string) error {
	if matched, err := regexp.MatchString("[^a-zA-Z\\d]", name); matched || err != nil {
		return errors.New("wallet name must only contain alphabetic characters and numbers")
	}
	return nil
}
