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
	"github.com/BoltzExchange/boltz-client/autoswap"
	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/BoltzExchange/boltz-client/build"
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/macaroons"
	"github.com/BoltzExchange/boltz-client/nursery"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const referralId = "boltz-client"

type serverState string

const (
	stateUnlocked         serverState = "unlocked"
	stateSyncing          serverState = "syncing"
	stateLocked           serverState = "locked"
	stateLightningSyncing serverState = "lightningSyncing"
)

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	network *boltz.Network

	onchain   *onchain.Onchain
	lightning lightning.LightningNode
	boltz     *boltz.Api
	nursery   *nursery.Nursery
	database  *database.Database
	swapper   *autoswap.AutoSwap
	macaroon  *macaroons.Service

	stop  chan bool
	state serverState
}

func (server *routedBoltzServer) GetBlockUpdates(currency boltz.Currency) (<-chan *onchain.BlockEpoch, func()) {
	blocks := server.nursery.BtcBlocks
	if currency == boltz.CurrencyLiquid {
		blocks = server.nursery.LiquidBlocks
	}
	updates := blocks.Get()
	return updates, func() {
		blocks.Remove(updates)
	}
}

func tenantContext(tenant *database.Tenant) context.Context {
	return macaroons.AddTenantToContext(context.Background(), tenant)
}

func (server *routedBoltzServer) CreateAutoSwap(tenant *database.Tenant, request *boltzrpc.CreateSwapRequest) error {
	_, err := server.createSwap(tenantContext(tenant), true, request)
	return err
}

func (server *routedBoltzServer) CreateAutoReverseSwap(tenant *database.Tenant, request *boltzrpc.CreateReverseSwapRequest) error {
	_, err := server.createReverseSwap(tenantContext(tenant), true, request)
	return err
}

func (server *routedBoltzServer) GetLightningChannels() ([]*lightning.LightningChannel, error) {
	if server.lightning != nil {
		return server.lightning.ListChannels()
	}
	return nil, errors.New("lightning channels not available")
}

func (server *routedBoltzServer) GetAutoSwapPairInfo(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	return server.GetPairInfo(context.Background(), &boltzrpc.GetPairInfoRequest{
		Type: swapType,
		Pair: pair,
	})
}

func (server *routedBoltzServer) CreateAutoChainSwap(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error {
	_, err := server.createChainSwap(tenantContext(tenant), true, request)
	return err
}

func handleError(err error) error {
	if err != nil {
		logger.Warn("RPC request failed: " + err.Error())
	}

	return err
}

func (server *routedBoltzServer) queryHeights() (heights *boltzrpc.BlockHeights, err error) {
	heights = &boltzrpc.BlockHeights{}
	heights.Btc, err = server.onchain.GetBlockHeight(boltz.CurrencyBtc)
	if err != nil {
		err = fmt.Errorf("failed to get block height for btc: %w", err)
		return
	}

	liquidHeight, err := server.onchain.GetBlockHeight(boltz.CurrencyLiquid)
	if err != nil {
		logger.Warnf("Failed to get block height for liquid: %v", err)
	} else {
		heights.Liquid = &liquidHeight
	}

	return heights, nil
}

func (server *routedBoltzServer) queryRefundableSwaps(ctx context.Context, heights *boltzrpc.BlockHeights) (
	swaps []*database.Swap, chainSwaps []*database.ChainSwap, err error,
) {
	tenantId := macaroons.TenantIdFromContext(ctx)
	swaps, chainSwaps, err = server.database.QueryAllRefundableSwaps(tenantId, boltz.CurrencyBtc, heights.Btc)
	if err != nil {
		return
	}

	if heights.Liquid != nil {
		liquidSwaps, liquidChainSwaps, liquidErr := server.database.QueryAllRefundableSwaps(tenantId, boltz.CurrencyLiquid, *heights.Liquid)
		if liquidErr != nil {
			err = liquidErr
			return
		}
		swaps = append(swaps, liquidSwaps...)
		chainSwaps = append(chainSwaps, liquidChainSwaps...)
	}

	return
}

func (server *routedBoltzServer) queryClaimableSwaps(ctx context.Context, heights *boltzrpc.BlockHeights) (
	reverseSwaps []*database.ReverseSwap, chainSwaps []*database.ChainSwap, err error,
) {
	tenantId := macaroons.TenantIdFromContext(ctx)
	reverseSwaps, chainSwaps, err = server.database.QueryAllClaimableSwaps(tenantId, boltz.CurrencyBtc, heights.Btc)
	if err != nil {
		return
	}

	if heights.Liquid != nil {
		liquidReverseSwaps, liquidChainSwaps, liquidErr := server.database.QueryAllClaimableSwaps(tenantId, boltz.CurrencyLiquid, *heights.Liquid)
		if liquidErr != nil {
			err = liquidErr
			return
		}
		reverseSwaps = append(reverseSwaps, liquidReverseSwaps...)
		chainSwaps = append(chainSwaps, liquidChainSwaps...)
	}

	return
}

func (server *routedBoltzServer) GetInfo(ctx context.Context, _ *boltzrpc.GetInfoRequest) (*boltzrpc.GetInfoResponse, error) {

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

	blockHeights, err := server.queryHeights()
	if err != nil {
		return nil, handleError(err)
	}

	refundableSwaps, refundableChainSwaps, err := server.queryRefundableSwaps(ctx, blockHeights)
	if err != nil {
		return nil, handleError(err)
	}

	claimableReverseSwaps, claimableChainSwaps, err := server.queryClaimableSwaps(ctx, blockHeights)
	if err != nil {
		return nil, handleError(err)
	}

	var refundableSwapIds, claimableSwapIds []string

	for _, refundableSwap := range refundableSwaps {
		refundableSwapIds = append(pendingReverseSwapIds, refundableSwap.Id)
	}
	for _, refundableChainSwap := range refundableChainSwaps {
		refundableSwapIds = append(refundableSwapIds, refundableChainSwap.Id)
	}

	for _, claimableReverseSwap := range claimableReverseSwaps {
		claimableSwapIds = append(claimableSwapIds, claimableReverseSwap.Id)
	}
	for _, claimableChainSwap := range claimableChainSwaps {
		claimableSwapIds = append(claimableSwapIds, claimableChainSwap.Id)
	}

	response := &boltzrpc.GetInfoResponse{
		Version:             build.GetVersion(),
		Network:             server.network.Name,
		BlockHeights:        blockHeights,
		Tenant:              serializeTenant(macaroons.TenantFromContext(ctx)),
		PendingSwaps:        pendingSwapIds,
		PendingReverseSwaps: pendingReverseSwapIds,
		RefundableSwaps:     refundableSwapIds,
		ClaimableSwaps:      claimableSwapIds,

		Symbol:      "BTC",
		BlockHeight: blockHeights.Btc,
	}

	if server.lightningAvailable(ctx) {
		lightningInfo, err := server.lightning.GetInfo()
		if err != nil {
			return nil, handleError(err)
		}

		response.Node = server.lightning.Name()
		response.NodePubkey = lightningInfo.Pubkey
		//nolint:staticcheck
		response.LndPubkey = lightningInfo.Pubkey
	} else {
		response.Node = "standalone"
	}

	if lnSwapper := server.swapper.GetLnSwapper(); lnSwapper != nil {
		if lnSwapper.Running() {
			response.AutoSwapStatus = "running"
		} else {
			if lnSwapper.Error() != "" {
				response.AutoSwapStatus = "error"
			} else {
				response.AutoSwapStatus = "disabled"
			}
		}
	}

	return response, nil

}

func (server *routedBoltzServer) GetPairInfo(_ context.Context, request *boltzrpc.GetPairInfoRequest) (*boltzrpc.PairInfo, error) {
	pair := utils.ParsePair(request.Pair)
	if request.Type == boltzrpc.SwapType_SUBMARINE {
		submarinePair, err := server.getSubmarinePair(request.Pair)
		if err != nil {
			return nil, handleError(err)
		}
		return serializeSubmarinePair(pair, submarinePair), nil
	} else if request.Type == boltzrpc.SwapType_REVERSE {
		reversePair, err := server.getReversePair(request.Pair)
		if err != nil {
			return nil, handleError(err)
		}
		return serializeReversePair(pair, reversePair), nil
	} else if request.Type == boltzrpc.SwapType_CHAIN {
		chainPair, err := server.getChainPair(request.Pair)
		if err != nil {
			return nil, handleError(err)
		}
		return serializeChainPair(pair, chainPair), nil
	}

	return nil, handleError(errors.New("invalid swap type"))
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

func (server *routedBoltzServer) ListSwaps(ctx context.Context, request *boltzrpc.ListSwapsRequest) (*boltzrpc.ListSwapsResponse, error) {
	response := &boltzrpc.ListSwapsResponse{}

	args := database.SwapQuery{
		Include:  request.Include,
		TenantId: macaroons.TenantIdFromContext(ctx),
	}

	if request.State != nil {
		args.States = []boltzrpc.SwapState{*request.State}
	}

	if request.From != nil {
		parsed := utils.ParseCurrency(request.From)
		args.From = &parsed
	}

	if request.To != nil {
		parsed := utils.ParseCurrency(request.To)
		args.To = &parsed
	}

	swaps, err := server.database.QuerySwaps(args)
	if err != nil {
		return nil, err
	}

	for _, swap := range swaps {
		response.Swaps = append(response.Swaps, serializeSwap(swap))
	}

	// Reverse Swaps
	reverseSwaps, err := server.database.QueryReverseSwaps(args)

	if err != nil {
		return nil, err
	}

	for _, reverseSwap := range reverseSwaps {
		response.ReverseSwaps = append(response.ReverseSwaps, serializeReverseSwap(reverseSwap))
	}

	chainSwaps, err := server.database.QueryChainSwaps(args)
	if err != nil {
		return nil, err
	}

	for _, chainSwap := range chainSwaps {
		response.ChainSwaps = append(response.ChainSwaps, serializeChainSwap(chainSwap))
	}

	return response, nil
}

func (server *routedBoltzServer) GetStats(ctx context.Context, request *boltzrpc.GetStatsRequest) (*boltzrpc.GetStatsResponse, error) {
	stats, err := server.database.QueryStats(database.SwapQuery{
		Include:  request.Include,
		TenantId: macaroons.TenantIdFromContext(ctx),
	}, []boltz.SwapType{boltz.NormalSwap, boltz.ReverseSwap, boltz.ChainSwap})
	if err != nil {
		return nil, handleError(err)
	}
	return &boltzrpc.GetStatsResponse{Stats: stats}, nil
}

var ErrInvalidAddress = status.Errorf(codes.InvalidArgument, "invalid address")

func (server *routedBoltzServer) RefundSwap(ctx context.Context, request *boltzrpc.RefundSwapRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	var swaps []*database.Swap
	var chainSwaps []*database.ChainSwap
	var currency boltz.Currency

	heights, err := server.queryHeights()
	if err != nil {
		return nil, handleError(err)
	}

	refundableSwaps, refundableChainSwaps, err := server.queryRefundableSwaps(ctx, heights)
	if err != nil {
		return nil, handleError(err)
	}

	var setAddress func(address string) error
	var setWallet func(walletId uint64) error

	for _, swap := range refundableSwaps {
		if swap.Id == request.Id {
			currency = swap.Pair.From
			setAddress = func(address string) error {
				return server.database.SetSwapRefundAddress(swap, address)
			}
			setWallet = func(walletId uint64) error {
				return server.database.SetSwapRefundWallet(swap, walletId)
			}
			swaps = append(swaps, swap)
		}
	}

	for _, chainSwap := range refundableChainSwaps {
		if chainSwap.Id == request.Id {
			currency = chainSwap.Pair.From
			setAddress = func(address string) error {
				return server.database.SetChainSwapAddress(chainSwap.FromData, address)
			}
			setWallet = func(walletId uint64) error {
				return server.database.SetChainSwapWallet(chainSwap.FromData, walletId)
			}
			chainSwaps = append(chainSwaps, chainSwap)
		}
	}

	if len(swaps) == 0 && len(chainSwaps) == 0 {
		return nil, handleError(status.Errorf(codes.NotFound, "no refundable swap with id %s found", request.Id))
	}

	if destination, ok := request.Destination.(*boltzrpc.RefundSwapRequest_Address); ok {
		if err := boltz.ValidateAddress(server.network, destination.Address, currency); err != nil {
			return nil, handleError(ErrInvalidAddress)
		}
		err = setAddress(destination.Address)
	}

	if destination, ok := request.Destination.(*boltzrpc.RefundSwapRequest_WalletId); ok {
		_, err = server.getWallet(ctx, onchain.WalletChecker{Id: &destination.WalletId, AllowReadonly: true})
		if err != nil {
			return nil, handleError(err)
		}
		err = setWallet(destination.WalletId)
	}

	if err != nil {
		return nil, handleError(err)
	}

	if _, err := server.nursery.RefundSwaps(currency, swaps, chainSwaps); err != nil {
		return nil, handleError(err)
	}

	return server.GetSwapInfo(ctx, &boltzrpc.GetSwapInfoRequest{Id: request.Id})
}

func (server *routedBoltzServer) ClaimSwaps(ctx context.Context, request *boltzrpc.ClaimSwapsRequest) (*boltzrpc.ClaimSwapsResponse, error) {
	var reverseSwaps []*database.ReverseSwap
	var chainSwaps []*database.ChainSwap
	var currency boltz.Currency

	heights, err := server.queryHeights()
	if err != nil {
		return nil, handleError(err)
	}

	claimableReverseSwaps, claimableChainSwaps, err := server.queryClaimableSwaps(ctx, heights)
	if err != nil {
		return nil, handleError(err)
	}

	for _, swap := range claimableReverseSwaps {
		if slices.Contains(request.SwapIds, swap.Id) {
			currency = swap.Pair.To
			reverseSwaps = append(reverseSwaps, swap)
		}
	}

	for _, chainSwap := range claimableChainSwaps {
		if slices.Contains(request.SwapIds, chainSwap.Id) {
			currency = chainSwap.Pair.To
			chainSwaps = append(chainSwaps, chainSwap)
		}
	}

	if len(reverseSwaps) == 0 && len(chainSwaps) == 0 {
		return nil, handleError(status.Errorf(codes.NotFound, "no claimable swaps with ids %s found", request.SwapIds))
	}

	if destination, ok := request.Destination.(*boltzrpc.ClaimSwapsRequest_Address); ok {
		if err := boltz.ValidateAddress(server.network, destination.Address, currency); err != nil {
			return nil, handleError(ErrInvalidAddress)
		}
		for _, swap := range reverseSwaps {
			if err := server.database.SetReverseSwapClaimAddress(swap, destination.Address); err != nil {
				return nil, handleError(err)
			}
		}
		for _, swap := range chainSwaps {
			if err := server.database.SetChainSwapAddress(swap.ToData, destination.Address); err != nil {
				return nil, handleError(err)
			}
		}
	}

	if destination, ok := request.Destination.(*boltzrpc.ClaimSwapsRequest_WalletId); ok {
		_, err = server.getWallet(ctx, onchain.WalletChecker{Id: &destination.WalletId, AllowReadonly: true})
		if err != nil {
			return nil, handleError(err)
		}
		for _, swap := range reverseSwaps {
			if err := server.database.SetReverseSwapWalletId(swap, destination.WalletId); err != nil {
				return nil, handleError(err)
			}
		}
		for _, swap := range chainSwaps {
			if err := server.database.SetChainSwapWallet(swap.ToData, destination.WalletId); err != nil {
				return nil, handleError(err)
			}
		}
	}

	transactionId, err := server.nursery.ClaimSwaps(currency, reverseSwaps, chainSwaps)
	if err != nil {
		return nil, handleError(err)
	}

	return &boltzrpc.ClaimSwapsResponse{TransactionId: transactionId}, nil
}

func (server *routedBoltzServer) GetSwapInfo(ctx context.Context, request *boltzrpc.GetSwapInfoRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	swap, reverseSwap, chainSwap, err := server.database.QueryAnySwap(request.Id)
	if err != nil {
		return nil, handleError(errors.New("could not find Swap with ID " + request.Id))
	}
	return server.serializeAnySwap(ctx, swap, reverseSwap, chainSwap)
}

func (server *routedBoltzServer) GetSwapInfoStream(request *boltzrpc.GetSwapInfoRequest, stream boltzrpc.Boltz_GetSwapInfoStreamServer) error {
	var updates <-chan nursery.SwapUpdate
	var stop func()

	if request.Id == "" || request.Id == "*" {
		logger.Info("Starting global Swap info stream")
		updates, stop = server.nursery.GlobalSwapUpdates()
	} else {
		info, err := server.GetSwapInfo(stream.Context(), request)
		if err != nil {
			return err
		}
		logger.Info("Starting Swap info stream for " + request.Id)
		updates, stop = server.nursery.SwapUpdates(request.Id)
		if updates == nil {
			if err := stream.Send(info); err != nil {
				return handleError(err)
			}
			return nil
		}
	}

	for update := range updates {
		response, err := server.serializeAnySwap(stream.Context(), update.Swap, update.ReverseSwap, update.ChainSwap)
		if err == nil {
			if err := stream.Send(response); err != nil {
				stop()
				return handleError(err)
			}
		}
	}

	return nil
}

func (server *routedBoltzServer) Deposit(ctx context.Context, request *boltzrpc.DepositRequest) (*boltzrpc.DepositResponse, error) {
	response, err := server.createSwap(ctx, false, &boltzrpc.CreateSwapRequest{
		Pair: &boltzrpc.Pair{
			From: boltzrpc.Currency_BTC,
			To:   boltzrpc.Currency_BTC,
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

func (server *routedBoltzServer) checkMagicRoutingHint(decoded *zpay32.Invoice, invoice string) (*boltzrpc.CreateSwapResponse, error) {
	if pubKey := boltz.FindMagicRoutingHint(decoded); pubKey != nil {
		logger.Infof("Found magic routing hint in invoice %s", invoice)
		reverseBip21, err := server.boltz.GetReverseSwapBip21(invoice)
		var boltzErr boltz.Error
		if err != nil && !errors.As(err, &boltzErr) {
			return nil, fmt.Errorf("could not get reverse swap bip21: %w", err)
		}

		parsed, err := url.Parse(reverseBip21.Bip21)
		if err != nil {
			return nil, err
		}

		signature, err := schnorr.ParseSignature(reverseBip21.Signature)
		if err != nil {
			return nil, err
		}

		address := parsed.Opaque
		addressHash := sha256.Sum256([]byte(address))
		if !signature.Verify(addressHash[:], pubKey) {
			return nil, errors.New("invalid reverse swap bip21 signature")
		}

		amount, err := strconv.ParseFloat(parsed.Query().Get("amount"), 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse bip21 amount: %w", err)
		}
		if amount > decoded.MilliSat.ToBTC() {
			return nil, errors.New("bip21 amount is higher than invoice amount")
		}

		return &boltzrpc.CreateSwapResponse{
			Address:        address,
			ExpectedAmount: uint64(amount * btcutil.SatoshiPerBitcoin),
			Bip21:          reverseBip21.Bip21,
		}, nil
	}
	return nil, nil
}

// TODO: custom refund address
func (server *routedBoltzServer) createSwap(ctx context.Context, isAuto bool, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	logger.Infof("Creating Swap for %d sats", request.Amount)

	privateKey, publicKey, err := newKeys()
	if err != nil {
		return nil, handleError(err)
	}

	pair := utils.ParsePair(request.Pair)

	submarinePair, err := server.getSubmarinePair(request.Pair)
	if err != nil {
		return nil, err
	}

	createSwap := boltz.CreateSwapRequest{
		From:            pair.From,
		To:              pair.To,
		PairHash:        submarinePair.Hash,
		RefundPublicKey: publicKey.SerializeCompressed(),
		ReferralId:      referralId,
	}
	var swapResponse *boltzrpc.CreateSwapResponse

	var preimage, preimageHash []byte
	if invoice := request.GetInvoice(); invoice != "" {
		decoded, err := zpay32.Decode(invoice, server.network.Btc)
		if err != nil {
			return nil, handleError(fmt.Errorf("invalid invoice: %w", err))
		}
		swapResponse, err = server.checkMagicRoutingHint(decoded, invoice)
		if err != nil {
			return nil, handleError(err)
		}
		preimageHash = decoded.PaymentHash[:]
		createSwap.Invoice = invoice
	} else if !server.lightningAvailable(ctx) {
		return nil, handleError(errors.New("invoice is required in standalone mode"))
	} else if request.Amount != 0 {
		invoice, err := server.lightning.CreateInvoice(request.Amount, nil, 0, utils.GetSwapMemo(string(pair.From)))
		if err != nil {
			return nil, handleError(err)
		}
		preimageHash = invoice.PaymentHash
		createSwap.Invoice = invoice.PaymentRequest
	} else {
		if request.SendFromInternal {
			return nil, handleError(errors.New("cannot auto send if amount is 0"))
		}
		preimage, preimageHash, err = newPreimage()
		if err != nil {
			return nil, handleError(err)
		}

		logger.Info("Creating Swap with preimage hash: " + hex.EncodeToString(preimageHash))

		createSwap.PreimageHash = preimageHash
	}

	sendFromInternal := request.GetSendFromInternal()
	wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{
		Currency:      pair.From,
		Id:            request.WalletId,
		AllowReadonly: !sendFromInternal,
	})
	if err != nil && sendFromInternal {
		return nil, handleError(err)
	}

	if swapResponse == nil {

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
			RefundAddress:       request.GetRefundAddress(),
			IsAuto:              isAuto,
			ServiceFeePercent:   utils.Percentage(submarinePair.Fees.Percentage),
			TenantId:            requireTenantId(ctx),
		}

		if request.SendFromInternal {
			id := wallet.GetWalletInfo().Id
			swap.WalletId = &id
		}

		swap.ClaimPubKey, err = btcec.ParsePubKey([]byte(response.ClaimPublicKey))
		if err != nil {
			return nil, handleError(err)
		}

		// for _, chanId := range request.ChanIds {
		// 	parsed, err := lightning.NewChanIdFromString(chanId)
		// 	if err != nil {
		// 		return nil, handleError(errors.New("invalid channel id: " + err.Error()))
		// 	}
		// 	swap.ChanIds = append(swap.ChanIds, parsed)
		// }

		if pair.From == boltz.CurrencyLiquid {
			swap.BlindingKey, _ = btcec.PrivKeyFromBytes(response.BlindingKey)

			if err != nil {
				return nil, handleError(err)
			}
		}

		if err := swap.InitTree(); err != nil {
			return nil, handleError(err)
		}

		if err := swap.SwapTree.Check(boltz.NormalSwap, swap.TimoutBlockHeight, preimageHash); err != nil {
			return nil, handleError(err)
		}

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
		swapResponse = &boltzrpc.CreateSwapResponse{
			Id:                 swap.Id,
			Address:            response.Address,
			ExpectedAmount:     response.ExpectedAmount,
			Bip21:              response.Bip21,
			TimeoutBlockHeight: response.TimeoutBlockHeight,
			TimeoutHours:       float32(timeoutHours),
		}

		logger.Info("Created new Swap " + swap.Id + ": " + marshalJson(swap.Serialize()))

		if request.SendFromInternal {
			swapResponse.TxId, err = server.sendToAddress(wallet, swapResponse.Address, swapResponse.ExpectedAmount)
			if err != nil {
				if dbErr := server.database.UpdateSwapState(&swap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
					logger.Errorf(dbErr.Error())
				}
				return nil, handleError(err)
			}
		}

		if err := server.nursery.RegisterSwap(swap); err != nil {
			return nil, handleError(err)
		}
	} else if request.SendFromInternal {
		swapResponse.TxId, err = server.sendToAddress(wallet, swapResponse.Address, swapResponse.ExpectedAmount)
		if err != nil {
			return nil, handleError(err)
		}
	}

	return swapResponse, nil
}

func (server *routedBoltzServer) CreateSwap(ctx context.Context, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	return server.createSwap(ctx, false, request)
}

func (server *routedBoltzServer) lightningAvailable(ctx context.Context) bool {
	return server.lightning != nil && isAdmin(ctx)
}

func requireTenant(ctx context.Context) database.Tenant {
	tenant := macaroons.TenantFromContext(ctx)
	if tenant == nil {
		return database.DefaultTenant
	}
	return *tenant
}

func requireTenantId(ctx context.Context) database.Id {
	return requireTenant(ctx).Id
}

func (server *routedBoltzServer) createReverseSwap(ctx context.Context, isAuto bool, request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	logger.Infof("Creating Reverse Swap for %d sats", request.Amount)

	externalPay := request.GetExternalPay()
	if !server.lightningAvailable(ctx) {
		if request.ExternalPay == nil {
			externalPay = true
		} else if !externalPay {
			return nil, handleError(errors.New("can not create reverse swap without external pay in standalone mode"))
		}
	}

	returnImmediately := request.GetReturnImmediately()
	if externalPay {
		// only error if it was explicitly set to false, implicitly set to true otherwise
		if request.ReturnImmediately != nil && !returnImmediately {
			return nil, handleError(errors.New("can not wait for swap transaction when using external pay"))
		} else {
			returnImmediately = true
		}
	}

	var walletId *database.Id
	claimAddress := request.Address
	pair := utils.ParsePair(request.Pair)
	if claimAddress != "" {
		err := boltz.ValidateAddress(server.network, claimAddress, pair.To)

		if err != nil {
			return nil, handleError(fmt.Errorf("Invalid claim address %s: %w", claimAddress, err))
		}
	} else {
		wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{
			Currency:      pair.To,
			Id:            request.WalletId,
			AllowReadonly: true,
		})
		if err != nil {
			return nil, handleError(err)
		}
		info := wallet.GetWalletInfo()
		walletId = &info.Id
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

	reversePair, err := server.getReversePair(request.Pair)
	if err != nil {
		return nil, handleError(err)
	}

	createRequest := boltz.CreateReverseSwapRequest{
		From:           pair.From,
		To:             pair.To,
		PairHash:       reversePair.Hash,
		InvoiceAmount:  request.Amount,
		PreimageHash:   preimageHash,
		ClaimPublicKey: publicKey.SerializeCompressed(),
		ReferralId:     referralId,
		Description:    request.GetDescription(),
	}

	if request.Address != "" {
		addressHash := sha256.Sum256([]byte(request.Address))
		signature, err := schnorr.Sign(privateKey, addressHash[:])
		if err != nil {
			return nil, handleError(err)
		}
		createRequest.AddressSignature = signature.Serialize()
		createRequest.Address = request.Address
	}

	response, err := server.boltz.CreateReverseSwap(createRequest)
	if err != nil {
		return nil, handleError(err)
	}

	key, err := btcec.ParsePubKey(response.RefundPublicKey)
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
		ExternalPay:         externalPay,
		WalletId:            walletId,
		TenantId:            requireTenantId(ctx),
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
		reverseSwap.BlindingKey, _ = btcec.PrivKeyFromBytes(response.BlindingKey)
		blindingPubKey = reverseSwap.BlindingKey.PubKey()

		if err != nil {
			return nil, handleError(err)
		}
	}

	if err := reverseSwap.InitTree(); err != nil {
		return nil, handleError(err)
	}

	if err := reverseSwap.SwapTree.Check(boltz.ReverseSwap, reverseSwap.TimeoutBlockHeight, preimageHash); err != nil {
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

	if err := server.nursery.RegisterReverseSwap(reverseSwap); err != nil {
		return nil, handleError(err)
	}

	logger.Info("Created new Reverse Swap " + reverseSwap.Id + ": " + marshalJson(reverseSwap.Serialize()))

	rpcResponse := &boltzrpc.CreateReverseSwapResponse{
		Id:            reverseSwap.Id,
		LockupAddress: response.LockupAddress,
		Invoice:       &reverseSwap.Invoice,
	}

	if !returnImmediately && request.AcceptZeroConf {
		updates, stop := server.nursery.SwapUpdates(reverseSwap.Id)
		defer stop()

		for update := range updates {
			info := update.ReverseSwap
			if info.State == boltzrpc.SwapState_SUCCESSFUL {
				rpcResponse.ClaimTransactionId = &update.ReverseSwap.ClaimTransactionId
				rpcResponse.RoutingFeeMilliSat = update.ReverseSwap.RoutingFeeMsat
			}
			if info.State == boltzrpc.SwapState_ERROR || info.State == boltzrpc.SwapState_SERVER_ERROR {
				return nil, handleError(errors.New("reverse swap failed: " + info.Error))
			}
		}
	}

	return rpcResponse, nil
}

func (server *routedBoltzServer) CreateReverseSwap(ctx context.Context, request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	return server.createReverseSwap(ctx, false, request)
}

func (server *routedBoltzServer) CreateChainSwap(ctx context.Context, request *boltzrpc.CreateChainSwapRequest) (*boltzrpc.ChainSwapInfo, error) {
	return server.createChainSwap(ctx, false, request)
}

func (server *routedBoltzServer) createChainSwap(ctx context.Context, isAuto bool, request *boltzrpc.CreateChainSwapRequest) (*boltzrpc.ChainSwapInfo, error) {
	logger.Infof("Creating new chain swap")

	tenantId := requireTenantId(ctx)

	claimPrivateKey, claimPub, err := newKeys()
	if err != nil {
		return nil, handleError(err)
	}

	refundPrivateKey, refundPub, err := newKeys()
	if err != nil {
		return nil, handleError(err)
	}

	pair := utils.ParsePair(request.Pair)

	chainPair, err := server.getChainPair(request.Pair)
	if err != nil {
		return nil, err
	}

	createChainSwap := boltz.ChainRequest{
		From:            pair.From,
		To:              pair.To,
		UserLockAmount:  request.Amount,
		PairHash:        chainPair.Hash,
		ClaimPublicKey:  claimPub.SerializeCompressed(),
		RefundPublicKey: refundPub.SerializeCompressed(),
		ReferralId:      referralId,
	}

	preimage, preimageHash, err := newPreimage()
	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Creating Chain Swap with preimage hash: " + hex.EncodeToString(preimageHash))

	createChainSwap.PreimageHash = preimageHash
	if request.Amount == 0 {
		if !request.GetExternalPay() {
			return nil, handleError(errors.New("cannot auto send if amount is 0"))
		}
	}

	externalPay := request.GetExternalPay()
	var fromWallet, toWallet onchain.Wallet
	if request.FromWalletId != nil {
		fromWallet, err = server.getWallet(ctx, onchain.WalletChecker{
			Id:       request.FromWalletId,
			Currency: pair.From,
		})
		if err != nil {
			return nil, handleError(err)
		}
	} else if !externalPay {
		return nil, handleError(errors.New("from wallet required if external pay is not specified"))
	}

	if request.ToWalletId != nil {
		toWallet, err = server.getWallet(ctx, onchain.WalletChecker{
			Id:       request.ToWalletId,
			Currency: pair.To,
		})
		if err != nil {
			return nil, handleError(err)
		}
	} else if request.ToAddress == nil {
		return nil, handleError(errors.New("to address or to wallet required"))
	}

	response, err := server.boltz.CreateChainSwap(createChainSwap)

	if err != nil {
		return nil, handleError(errors.New("boltz error: " + err.Error()))
	}

	chainSwap := database.ChainSwap{
		Id:                response.Id,
		Pair:              pair,
		State:             boltzrpc.SwapState_PENDING,
		Error:             "",
		Preimage:          preimage,
		IsAuto:            isAuto,
		AcceptZeroConf:    request.GetAcceptZeroConf(),
		ServiceFeePercent: utils.Percentage(chainPair.Fees.Percentage),
		TenantId:          tenantId,
	}

	parseDetails := func(details *boltz.ChainSwapData, currency boltz.Currency) (*database.ChainSwapData, error) {
		swapData := &database.ChainSwapData{
			Id:                 response.Id,
			Currency:           currency,
			Amount:             details.Amount,
			TimeoutBlockHeight: details.TimeoutBlockHeight,
			Tree:               details.SwapTree.Deserialize(),
			LockupAddress:      details.LockupAddress,
		}
		if currency == pair.From {
			swapData.PrivateKey = refundPrivateKey
			swapData.Address = request.GetRefundAddress()
		} else {
			swapData.PrivateKey = claimPrivateKey
			swapData.Address = request.GetToAddress()
		}

		if swapData.Address != "" {
			if err := boltz.ValidateAddress(server.network, swapData.Address, currency); err != nil {
				return nil, err
			}
		}

		swapData.TheirPublicKey, err = btcec.ParsePubKey(details.ServerPublicKey)
		if err != nil {
			return nil, err
		}

		if currency == boltz.CurrencyLiquid {
			swapData.BlindingKey, _ = btcec.PrivKeyFromBytes(details.BlindingKey)
		}

		if err := swapData.InitTree(currency == pair.To); err != nil {
			return nil, err
		}

		if err := swapData.Tree.Check(boltz.ChainSwap, swapData.TimeoutBlockHeight, preimageHash); err != nil {
			return nil, err
		}

		if err := swapData.Tree.CheckAddress(details.LockupAddress, server.network, swapData.BlindingPubKey()); err != nil {
			return nil, err
		}

		return swapData, nil
	}

	chainSwap.ToData, err = parseDetails(response.ClaimDetails, pair.To)
	if err != nil {
		return nil, handleError(err)
	}
	if toWallet != nil {
		id := toWallet.GetWalletInfo().Id
		chainSwap.ToData.WalletId = &id
	}

	chainSwap.FromData, err = parseDetails(response.LockupDetails, pair.From)
	if err != nil {
		return nil, handleError(err)
	}
	if !externalPay {
		id := fromWallet.GetWalletInfo().Id
		chainSwap.FromData.WalletId = &id
	}

	logger.Info("Verified redeem script and address of chainSwap " + chainSwap.Id)

	err = server.database.CreateChainSwap(chainSwap)
	if err != nil {
		return nil, handleError(err)
	}

	logger.Infof("Created new chain swap %s: %s", chainSwap.Id, marshalJson(chainSwap.Serialize()))

	if !externalPay {
		from := chainSwap.FromData
		from.LockupTransactionId, err = server.sendToAddress(fromWallet, from.LockupAddress, from.Amount)
		if err != nil {
			if dbErr := server.database.UpdateChainSwapState(&chainSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
				logger.Error(dbErr.Error())
			}
			return nil, handleError(err)
		}
	}

	if err := server.nursery.RegisterChainSwap(chainSwap); err != nil {
		return nil, handleError(err)
	}

	return serializeChainSwap(&chainSwap), nil
}

func (server *routedBoltzServer) loginWallet(credentials *wallet.Credentials) (*wallet.Wallet, error) {
	chain, err := server.onchain.GetCurrency(credentials.Currency)
	if err != nil {
		return nil, err
	}
	result, err := wallet.Login(credentials)
	if err != nil {
		return nil, err
	}
	result.SetTxProvider(chain.Tx)
	return result, nil
}

func (server *routedBoltzServer) importWallet(ctx context.Context, credentials *wallet.Credentials, password string) error {
	decryptWalletCredentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		return errors.New("wrong password")
	}

	for _, existing := range decryptWalletCredentials {
		if existing.Mnemonic == credentials.Mnemonic && existing.Xpub == credentials.Xpub && existing.CoreDescriptor == credentials.CoreDescriptor {
			return fmt.Errorf("wallet %s has the same credentials", existing.Name)
		}
	}

	wallet, err := server.loginWallet(credentials)
	if err != nil {
		return errors.New("could not login: " + err.Error())
	}
	if wallet.GetWalletInfo().Readonly {
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
	if err := server.database.CreateWallet(&database.Wallet{Credentials: credentials}); err != nil {
		return err
	}
	if password != "" {
		if err := server.encryptWalletCredentials(password, decryptWalletCredentials); err != nil {
			return fmt.Errorf("could not encrypt credentials: %w", err)
		}
	}
	wallet.Id = credentials.Id

	server.onchain.AddWallet(wallet)
	return nil
}

func (server *routedBoltzServer) ImportWallet(ctx context.Context, request *boltzrpc.ImportWalletRequest) (*boltzrpc.Wallet, error) {
	if request.Params == nil {
		return nil, handleError(errors.New("missing wallet parameters"))
	}
	if err := checkName(request.Params.Name); err != nil {
		return nil, handleError(err)
	}

	currency := utils.ParseCurrency(&request.Params.Currency)
	credentials := &wallet.Credentials{
		WalletInfo: onchain.WalletInfo{
			Name:     request.Params.Name,
			Currency: currency,
			TenantId: requireTenantId(ctx),
		},
		Mnemonic:       request.Credentials.GetMnemonic(),
		Xpub:           request.Credentials.GetXpub(),
		CoreDescriptor: request.Credentials.GetCoreDescriptor(),
		Subaccount:     request.Credentials.Subaccount,
	}

	if err := server.importWallet(ctx, credentials, request.Params.GetPassword()); err != nil {
		return nil, handleError(err)
	}
	return server.GetWallet(ctx, &boltzrpc.GetWalletRequest{Id: &credentials.Id})
}

func (server *routedBoltzServer) SetSubaccount(ctx context.Context, request *boltzrpc.SetSubaccountRequest) (*boltzrpc.Subaccount, error) {
	wallet, err := server.getOwnWallet(ctx, onchain.WalletChecker{Id: &request.WalletId})
	if err != nil {
		return nil, handleError(err)
	}

	subaccountNumber, err := wallet.SetSubaccount(request.Subaccount)
	if err != nil {
		return nil, handleError(err)
	}

	if err := server.database.SetWalletSubaccount(wallet.GetWalletInfo().Id, *subaccountNumber); err != nil {
		return nil, handleError(err)
	}

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

func (server *routedBoltzServer) GetSubaccounts(ctx context.Context, request *boltzrpc.GetSubaccountsRequest) (*boltzrpc.GetSubaccountsResponse, error) {
	wallet, err := server.getOwnWallet(ctx, onchain.WalletChecker{Id: &request.WalletId})
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

func (server *routedBoltzServer) CreateWallet(ctx context.Context, request *boltzrpc.CreateWalletRequest) (*boltzrpc.CreateWalletResponse, error) {
	mnemonic, err := wallet.GenerateMnemonic()
	if err != nil {
		return nil, handleError(errors.New("could not generate new mnemonic: " + err.Error()))
	}

	created, err := server.ImportWallet(ctx, &boltzrpc.ImportWalletRequest{
		Params: request.Params,
		Credentials: &boltzrpc.WalletCredentials{
			Mnemonic: &mnemonic,
		},
	})
	if err != nil {
		return nil, handleError(err)
	}

	_, err = server.SetSubaccount(ctx, &boltzrpc.SetSubaccountRequest{
		WalletId: created.Id,
	})
	if err != nil {
		return nil, err
	}
	return &boltzrpc.CreateWalletResponse{
		Mnemonic: mnemonic,
		Wallet:   created,
	}, nil
}

func (server *routedBoltzServer) serializeWallet(wal onchain.Wallet) (*boltzrpc.Wallet, error) {
	info := wal.GetWalletInfo()
	result := &boltzrpc.Wallet{
		Id:       info.Id,
		Name:     info.Name,
		Currency: serializeCurrency(info.Currency),
		Readonly: info.Readonly,
		TenantId: info.TenantId,
	}
	balance, err := wal.GetBalance()
	if err != nil {
		if !errors.Is(err, wallet.ErrSubAccountNotSet) {
			return nil, handleError(fmt.Errorf("could not get balance for wallet %s: %w", info.Name, err))
		}
	} else {
		result.Balance = serializeWalletBalance(balance)
	}
	return result, nil
}

func (server *routedBoltzServer) GetWallet(ctx context.Context, request *boltzrpc.GetWalletRequest) (*boltzrpc.Wallet, error) {
	wallet, err := server.getWallet(ctx, onchain.WalletChecker{
		Id:            request.Id,
		Name:          request.Name,
		AllowReadonly: true,
	})
	if err != nil {
		return nil, handleError(err)
	}

	return server.serializeWallet(wallet)
}

func (server *routedBoltzServer) GetWallets(ctx context.Context, request *boltzrpc.GetWalletsRequest) (*boltzrpc.Wallets, error) {
	var response boltzrpc.Wallets
	checker := onchain.WalletChecker{
		Currency:      utils.ParseCurrency(request.Currency),
		AllowReadonly: request.GetIncludeReadonly(),
		TenantId:      macaroons.TenantIdFromContext(ctx),
	}
	for _, current := range server.onchain.GetWallets(checker) {
		wallet, err := server.serializeWallet(current)
		if err != nil {
			return nil, err
		}
		response.Wallets = append(response.Wallets, wallet)
	}
	return &response, nil
}

func (server *routedBoltzServer) getWallet(ctx context.Context, checker onchain.WalletChecker) (onchain.Wallet, error) {
	if checker.Id == nil {
		id := requireTenantId(ctx)
		checker.TenantId = &id
		if checker.Name == nil {
			return nil, status.Errorf(codes.InvalidArgument, "id or name required")
		}
	}
	return server.getAnyWallet(ctx, checker)
}

func (server *routedBoltzServer) getAnyWallet(ctx context.Context, checker onchain.WalletChecker) (onchain.Wallet, error) {
	if checker.TenantId == nil {
		checker.TenantId = macaroons.TenantIdFromContext(ctx)
	}
	found, err := server.onchain.GetAnyWallet(checker)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, err.Error())
	}
	return found, nil
}

func (server *routedBoltzServer) GetWalletCredentials(ctx context.Context, request *boltzrpc.GetWalletCredentialsRequest) (*boltzrpc.WalletCredentials, error) {
	wallet, err := server.getWallet(ctx, onchain.WalletChecker{Id: &request.Id})
	if err != nil {
		return nil, handleError(err)
	}
	info := wallet.GetWalletInfo()
	creds, err := server.database.GetWallet(request.Id)
	if err != nil {
		return nil, handleError(fmt.Errorf("could not read credentials for wallet %s: %w", info.Name, err))
	}
	if creds.NodePubkey != nil {
		return nil, handleError(errors.New("cant get credentials for node wallet"))
	}
	if creds.Encrypted() {
		creds.Credentials, err = creds.Decrypt(request.GetPassword())
		if err != nil {
			return nil, handleError(fmt.Errorf("invalid password: %w", err))
		}
	}

	return serializeWalletCredentials(creds.Credentials), err
}

func (server *routedBoltzServer) RemoveWallet(ctx context.Context, request *boltzrpc.RemoveWalletRequest) (*boltzrpc.RemoveWalletResponse, error) {
	wallet, err := server.getOwnWallet(ctx, onchain.WalletChecker{
		Id:            &request.Id,
		AllowReadonly: true,
	})
	if err != nil {
		return nil, handleError(err)
	}
	if server.swapper.WalletUsed(request.Id) {
		return nil, handleError(fmt.Errorf(
			"wallet %s is used in autoswap, configure a different wallet in autoswap before removing this wallet",
			wallet.GetWalletInfo().Name,
		))
	}
	if err := wallet.Disconnect(); err != nil {
		return nil, handleError(err)
	}
	id := wallet.GetWalletInfo().Id
	if err := server.database.DeleteWallet(id); err != nil {
		return nil, handleError(err)
	}
	server.onchain.RemoveWallet(id)

	logger.Debugf("Removed wallet %v", wallet)

	return &boltzrpc.RemoveWalletResponse{}, nil
}

func (server *routedBoltzServer) Stop(context.Context, *empty.Empty) (*empty.Empty, error) {
	if server.nursery != nil {
		server.nursery.Stop()
		logger.Debugf("Stopped nursery")
	}
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
	credentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			if server.state == stateLocked {
				return err
			}
			logger.Infof("Server is locked")
			server.state = stateLocked
			return nil
		} else {
			return err
		}
	}

	if server.lightning != nil {
		info, err := server.lightning.GetInfo()
		if err != nil {
			return fmt.Errorf("could not get info from lightning: %v", err)
		}
		walletInfo := onchain.WalletInfo{
			Name:     server.lightning.Name(),
			Currency: boltz.CurrencyBtc,
			Readonly: false,
			TenantId: database.DefaultTenantId,
		}
		nodeWallet, err := server.database.GetNodeWallet(info.Pubkey)
		if err != nil {
			err = server.database.CreateWallet(&database.Wallet{
				Credentials: &wallet.Credentials{
					WalletInfo: walletInfo,
				},
				NodePubkey: &info.Pubkey,
			})
			if err != nil {
				return fmt.Errorf("could not create wallet for lightning node: %w", err)
			}
			nodeWallet, err = server.database.GetNodeWallet(info.Pubkey)
			if err != nil {
				return fmt.Errorf("could not get node wallet form db: %s", err)
			}
		}
		walletInfo.Id = nodeWallet.Id
		server.lightning.SetupWallet(walletInfo)
		server.onchain.AddWallet(server.lightning)
	}

	server.state = stateSyncing
	go func() {
		defer func() {
			server.state = stateUnlocked
		}()
		var wg sync.WaitGroup
		wg.Add(len(credentials))
		for _, creds := range credentials {
			creds := creds
			go func() {
				defer wg.Done()
				wallet, err := server.loginWallet(creds)
				if err != nil {
					logger.Errorf("could not login to wallet: %v", err)
				} else {
					logger.Infof("logged into wallet: %v", wallet.GetWalletInfo())
					server.onchain.AddWallet(wallet)
				}
			}()
		}
		wg.Wait()

		server.nursery = &nursery.Nursery{}
		err = server.nursery.Init(
			server.network,
			server.lightning,
			server.onchain,
			server.boltz,
			server.database,
		)
		if err != nil {
			logger.Fatalf("could not start nursery: %v", err)
		}

		if err := server.swapper.LoadConfig(); err != nil {
			logger.Warnf("Could not load autoswap config: %v", err)
		}
	}()

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

func (server *routedBoltzServer) requestAllowed(fullMethod string) error {
	if strings.Contains(fullMethod, "Stop") {
		return nil
	}
	if server.state == stateLightningSyncing {
		return status.Errorf(codes.Unavailable, "connected lightning node is syncing, please wait")
	}
	if server.state == stateSyncing {
		return status.Error(codes.Unavailable, "boltzd is syncing its wallets, please wait")
	}
	if strings.Contains(fullMethod, "Unlock") {
		if server.state == stateUnlocked {
			return status.Errorf(codes.FailedPrecondition, "boltzd is already unlocked")
		}
	} else if server.state == stateLocked {
		return status.Error(codes.FailedPrecondition, "boltzd is locked, use \"unlock\" to enable full RPC access")
	}
	return nil
}

func (server *routedBoltzServer) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := server.requestAllowed(info.FullMethod); err != nil {
			return nil, handleError(err)
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
		if err := server.requestAllowed(info.FullMethod); err != nil {
			return handleError(err)
		}

		return handler(srv, ss)
	}
}

func (server *routedBoltzServer) getOwnWallet(ctx context.Context, checker onchain.WalletChecker) (*wallet.Wallet, error) {
	existing, err := server.getWallet(ctx, checker)
	if err != nil {
		return nil, err
	}
	wallet, ok := existing.(*wallet.Wallet)
	if !ok {
		return nil, fmt.Errorf("wallet %s can not be modified", existing.GetWalletInfo().Name)
	}
	return wallet, nil
}

func findPair[T any](pair boltz.Pair, nested map[boltz.Currency]map[boltz.Currency]T) (*T, error) {
	from, hasPair := nested[pair.From]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair from %v", pair)
	}
	result, hasPair := from[pair.To]
	if !hasPair {
		return nil, fmt.Errorf("could not find pair to %v", pair)
	}
	return &result, nil
}

func (server *routedBoltzServer) getSubmarinePair(request *boltzrpc.Pair) (*boltz.SubmarinePair, error) {
	pairsResponse, err := server.boltz.GetSubmarinePairs()
	if err != nil {
		return nil, handleError(err)
	}
	pair := utils.ParsePair(request)
	return findPair(pair, pairsResponse)
}

func (server *routedBoltzServer) getReversePair(request *boltzrpc.Pair) (*boltz.ReversePair, error) {
	pairsResponse, err := server.boltz.GetReversePairs()
	if err != nil {
		return nil, err
	}
	pair := utils.ParsePair(request)
	return findPair(pair, pairsResponse)
}

func (server *routedBoltzServer) getChainPair(request *boltzrpc.Pair) (*boltz.ChainPair, error) {
	pairsResponse, err := server.boltz.GetChainPairs()
	if err != nil {
		return nil, err
	}
	pair := utils.ParsePair(request)
	return findPair(pair, pairsResponse)
}

func (server *routedBoltzServer) GetPairs(context.Context, *empty.Empty) (*boltzrpc.GetPairsResponse, error) {
	response := &boltzrpc.GetPairsResponse{}

	submarinePairs, err := server.boltz.GetSubmarinePairs()
	if err != nil {
		return nil, err
	}

	for from, p := range submarinePairs {
		for to, pair := range p {
			if from != boltz.CurrencyRootstock {
				response.Submarine = append(response.Submarine, serializeSubmarinePair(boltz.Pair{
					From: from,
					To:   to,
				}, &pair))
			}
		}
	}

	reversePairs, err := server.boltz.GetReversePairs()
	if err != nil {
		return nil, err
	}

	for from, p := range reversePairs {
		for to, pair := range p {
			if to != boltz.CurrencyRootstock {
				response.Reverse = append(response.Reverse, serializeReversePair(boltz.Pair{
					From: from,
					To:   to,
				}, &pair))
			}
		}
	}

	chainPairs, err := server.boltz.GetChainPairs()
	if err != nil {
		return nil, err
	}

	for from, p := range chainPairs {
		for to, pair := range p {
			if from != boltz.CurrencyRootstock && to != boltz.CurrencyRootstock {
				response.Chain = append(response.Chain, serializeChainPair(boltz.Pair{
					From: from,
					To:   to,
				}, &pair))
			}
		}
	}

	return response, nil

}

func isAdmin(ctx context.Context) bool {
	id := macaroons.TenantIdFromContext(ctx)
	return id == nil || *id == database.DefaultTenantId
}

func (server *routedBoltzServer) BakeMacaroon(ctx context.Context, request *boltzrpc.BakeMacaroonRequest) (*boltzrpc.BakeMacaroonResponse, error) {

	if !isAdmin(ctx) {
		return nil, handleError(errors.New("only admin can bake macaroons"))
	}

	if request.TenantId != nil {
		_, err := server.database.GetTenant(request.GetTenantId())
		if err != nil {
			return nil, handleError(fmt.Errorf("could not find tenant %d: %w", request.TenantId, err))
		}
	}

	permissions := macaroons.GetPermissions(request.TenantId != nil, request.Permissions)
	mac, err := server.macaroon.NewMacaroon(request.TenantId, permissions...)
	if err != nil {
		return nil, handleError(err)
	}
	macBytes, err := mac.M().MarshalBinary()
	if err != nil {
		return nil, handleError(err)
	}
	return &boltzrpc.BakeMacaroonResponse{
		Macaroon: hex.EncodeToString(macBytes),
	}, nil
}

func (server *routedBoltzServer) CreateTenant(ctx context.Context, request *boltzrpc.CreateTenantRequest) (*boltzrpc.Tenant, error) {
	tenant := &database.Tenant{Name: request.Name}

	if err := server.database.CreateTenant(tenant); err != nil {
		return nil, handleError(err)
	}

	return serializeTenant(tenant), nil
}

func (server *routedBoltzServer) GetTenant(ctx context.Context, request *boltzrpc.GetTenantRequest) (*boltzrpc.Tenant, error) {
	tenant, err := server.database.GetTenantByName(request.Name)
	if err != nil {
		return nil, handleError(err)
	}

	return serializeTenant(tenant), nil
}

func (server *routedBoltzServer) ListTenants(ctx context.Context, request *boltzrpc.ListTenantsRequest) (*boltzrpc.ListTenantsResponse, error) {
	tenants, err := server.database.QueryTenants()
	if err != nil {
		return nil, handleError(err)
	}

	response := &boltzrpc.ListTenantsResponse{}
	for _, tenant := range tenants {
		response.Tenants = append(response.Tenants, serializeTenant(tenant))
	}

	return response, nil
}

func (server *routedBoltzServer) serializeAnySwap(ctx context.Context, swap *database.Swap, reverseSwap *database.ReverseSwap, chainSwap *database.ChainSwap) (*boltzrpc.GetSwapInfoResponse, error) {
	if tenantId := macaroons.TenantIdFromContext(ctx); tenantId != nil {
		err := status.Error(codes.PermissionDenied, "tenant does not have permission to view this swap")
		if swap != nil && swap.TenantId != *tenantId {
			return nil, handleError(err)
		}
		if reverseSwap != nil && reverseSwap.TenantId != *tenantId {
			return nil, handleError(err)
		}
		if chainSwap != nil && chainSwap.TenantId != *tenantId {
			return nil, handleError(err)
		}
	}
	return &boltzrpc.GetSwapInfoResponse{
		Swap:        serializeSwap(swap),
		ReverseSwap: serializeReverseSwap(reverseSwap),
		ChainSwap:   serializeChainSwap(chainSwap),
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

func (server *routedBoltzServer) sendToAddress(wallet onchain.Wallet, address string, amount uint64) (string, error) {
	// TODO: custom block target?
	feeSatPerVbyte, err := server.onchain.EstimateFee(wallet.GetWalletInfo().Currency, 2)
	if err != nil {
		return "", err
	}
	logger.Infof("Using fee of %f sat/vbyte", feeSatPerVbyte)
	return wallet.SendToAddress(address, amount, feeSatPerVbyte)
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
	if name == "" {
		return errors.New("wallet name must not be empty")
	}
	if matched, err := regexp.MatchString("[^a-zA-Z\\d]", name); matched || err != nil {
		return errors.New("wallet name must only contain alphabetic characters and numbers")
	}
	return nil
}
