package rpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/autoswap"
	"github.com/BoltzExchange/boltz-client/v2/internal/build"
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/macaroons"
	"github.com/BoltzExchange/boltz-client/v2/internal/nursery"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/internal/utils"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/serializers"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/fiatjaf/go-lnurl"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/lightningnetwork/lnd/zpay32"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverState string

const (
	stateUnlocked         serverState = "unlocked"
	stateUnavailable      serverState = "unavailable"
	stateSyncing          serverState = "syncing"
	stateLocked           serverState = "locked"
	stateLightningSyncing serverState = "lightningSyncing"
	stateStopping         serverState = "stopping"
)

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	network *boltz.Network

	onchain    *onchain.Onchain
	lightning  lightning.LightningNode
	boltz      *boltz.Api
	nursery    *nursery.Nursery
	database   *database.Database
	swapper    *autoswap.AutoSwap
	macaroon   *macaroons.Service
	referralId string

	walletBackends map[boltz.Currency]onchain.WalletBackend

	stop      chan bool
	state     serverState
	stateLock sync.RWMutex

	newKeyLock sync.Mutex
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

func (server *routedBoltzServer) WalletSendFee(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
	return server.GetWalletSendFee(context.Background(), request)
}

func handleError(err error) error {
	if err != nil && status.Code(err) == codes.Unknown {
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

func (server *routedBoltzServer) queryClaimableSwaps(ctx context.Context) (
	reverseSwaps []*database.ReverseSwap, chainSwaps []*database.ChainSwap, err error,
) {
	tenantId := macaroons.TenantIdFromContext(ctx)
	reverseSwaps, chainSwaps, err = server.nursery.QueryClaimableSwaps(tenantId, boltz.CurrencyBtc)
	if err != nil {
		return
	}

	liquidReverseSwaps, liquidChainSwaps, liquidErr := server.nursery.QueryClaimableSwaps(tenantId, boltz.CurrencyLiquid)
	if liquidErr != nil {
		err = liquidErr
		return
	}
	reverseSwaps = append(reverseSwaps, liquidReverseSwaps...)
	chainSwaps = append(chainSwaps, liquidChainSwaps...)

	return
}

func (server *routedBoltzServer) GetInfo(ctx context.Context, _ *boltzrpc.GetInfoRequest) (*boltzrpc.GetInfoResponse, error) {

	pendingSwaps, err := server.database.QueryPendingSwaps()

	if err != nil {
		return nil, err
	}

	var pendingSwapIds []string

	for _, pendingSwap := range pendingSwaps {
		pendingSwapIds = append(pendingSwapIds, pendingSwap.Id)
	}

	pendingReverseSwaps, err := server.database.QueryPendingReverseSwaps()

	if err != nil {
		return nil, err
	}

	var pendingReverseSwapIds []string

	for _, pendingReverseSwap := range pendingReverseSwaps {
		pendingReverseSwapIds = append(pendingReverseSwapIds, pendingReverseSwap.Id)
	}

	blockHeights, err := server.queryHeights()
	if err != nil {
		return nil, err
	}

	refundableSwaps, refundableChainSwaps, err := server.queryRefundableSwaps(ctx, blockHeights)
	if err != nil {
		return nil, err
	}

	claimableReverseSwaps, claimableChainSwaps, err := server.queryClaimableSwaps(ctx)
	if err != nil {
		return nil, err
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
			return nil, err
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
	switch request.Type {
	case boltzrpc.SwapType_SUBMARINE:
		return server.getSubmarinePair(request.Pair)
	case boltzrpc.SwapType_REVERSE:
		return server.getReversePair(request.Pair)
	case boltzrpc.SwapType_CHAIN:
		return server.getChainPair(request.Pair)
	default:
		return nil, errors.New("unknown swap type")
	}
}

func (server *routedBoltzServer) GetServiceInfo(_ context.Context, request *boltzrpc.GetServiceInfoRequest) (*boltzrpc.GetServiceInfoResponse, error) {
	fees, limits, err := server.getPairs(boltz.PairBtc)

	if err != nil {
		return nil, err
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
		parsed := serializers.ParseCurrency(request.From)
		args.From = &parsed
	}

	if request.To != nil {
		parsed := serializers.ParseCurrency(request.To)
		args.To = &parsed
	}

	if request.GetUnify() {
		args.Offset = request.Offset
		args.Limit = request.Limit
		allSwaps, err := server.database.QueryAllSwaps(args)
		if err != nil {
			return nil, err
		}

		for _, swap := range allSwaps {
			response.AllSwaps = append(response.AllSwaps, serializeAnySwap(swap))
		}
	} else {
		if request.Offset != nil || request.Limit != nil {
			return nil, status.Errorf(codes.InvalidArgument, "offset and limit are only supported with unify")

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
	}

	return response, nil
}

func (server *routedBoltzServer) GetStats(ctx context.Context, request *boltzrpc.GetStatsRequest) (*boltzrpc.GetStatsResponse, error) {
	stats, err := server.database.QueryStats(database.SwapQuery{
		Include:  request.Include,
		TenantId: macaroons.TenantIdFromContext(ctx),
	}, []boltz.SwapType{boltz.NormalSwap, boltz.ReverseSwap, boltz.ChainSwap})
	if err != nil {
		return nil, err
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
		return nil, err
	}

	refundableSwaps, refundableChainSwaps, err := server.queryRefundableSwaps(ctx, heights)
	if err != nil {
		return nil, err
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
		return nil, status.Errorf(codes.NotFound, "no refundable swap with id %s found", request.Id)
	}

	if destination, ok := request.Destination.(*boltzrpc.RefundSwapRequest_Address); ok {
		if err := boltz.ValidateAddress(server.network, destination.Address, currency); err != nil {
			return nil, ErrInvalidAddress
		}
		err = setAddress(destination.Address)
	}

	if destination, ok := request.Destination.(*boltzrpc.RefundSwapRequest_WalletId); ok {
		_, err = server.getWallet(ctx, onchain.WalletChecker{Id: &destination.WalletId, AllowReadonly: true})
		if err != nil {
			return nil, err
		}
		err = setWallet(destination.WalletId)
	}

	if err != nil {
		return nil, err
	}

	if _, err := server.nursery.RefundSwaps(currency, swaps, chainSwaps); err != nil {
		return nil, err
	}

	return server.GetSwapInfo(ctx, &boltzrpc.GetSwapInfoRequest{Id: request.Id})
}

func (server *routedBoltzServer) ClaimSwaps(ctx context.Context, request *boltzrpc.ClaimSwapsRequest) (*boltzrpc.ClaimSwapsResponse, error) {
	var reverseSwaps []*database.ReverseSwap
	var chainSwaps []*database.ChainSwap
	var currency boltz.Currency

	claimableReverseSwaps, claimableChainSwaps, err := server.queryClaimableSwaps(ctx)
	if err != nil {
		return nil, err
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
		return nil, status.Errorf(codes.NotFound, "no claimable swaps with ids %s found", request.SwapIds)
	}

	if destination, ok := request.Destination.(*boltzrpc.ClaimSwapsRequest_Address); ok {
		if err := boltz.ValidateAddress(server.network, destination.Address, currency); err != nil {
			return nil, ErrInvalidAddress
		}
		for _, swap := range reverseSwaps {
			if err := server.database.SetReverseSwapClaimAddress(swap, destination.Address); err != nil {
				return nil, err
			}
		}
		for _, swap := range chainSwaps {
			if err := server.database.SetChainSwapAddress(swap.ToData, destination.Address); err != nil {
				return nil, err
			}
		}
	}

	if destination, ok := request.Destination.(*boltzrpc.ClaimSwapsRequest_WalletId); ok {
		_, err = server.getWallet(ctx, onchain.WalletChecker{Id: &destination.WalletId, AllowReadonly: true})
		if err != nil {
			return nil, err
		}
		for _, swap := range reverseSwaps {
			if err := server.database.SetReverseSwapWalletId(swap, destination.WalletId); err != nil {
				return nil, err
			}
		}
		for _, swap := range chainSwaps {
			if err := server.database.SetChainSwapWallet(swap.ToData, destination.WalletId); err != nil {
				return nil, err
			}
		}
	}

	transactionId, err := server.nursery.ClaimSwaps(currency, reverseSwaps, chainSwaps)
	if err != nil {
		return nil, err
	}

	return &boltzrpc.ClaimSwapsResponse{TransactionId: transactionId}, nil
}

func (server *routedBoltzServer) GetSwapInfo(ctx context.Context, request *boltzrpc.GetSwapInfoRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	// nolint: staticcheck
	if request.Id != "" {
		// nolint: staticcheck
		request.Identifier = &boltzrpc.GetSwapInfoRequest_SwapId{SwapId: request.Id}
	}
	if swapId := request.GetSwapId(); swapId != "" {
		swap, reverseSwap, chainSwap, err := server.database.QueryAnySwap(swapId)
		if err != nil {
			return nil, errors.New("could not find Swap with ID " + swapId)
		}
		return server.serializeAnySwap(ctx, swap, reverseSwap, chainSwap)
	} else if paymentHash := request.GetPaymentHash(); paymentHash != nil {
		swap, err := server.database.QuerySwapByPaymentHash(paymentHash)
		if err != nil {
			return nil, err
		}
		if swap == nil {
			return nil, status.Errorf(codes.NotFound, "could not find Swap with payment hash")
		}
		return server.serializeAnySwap(ctx, swap, nil, nil)
	}
	return nil, status.Errorf(codes.InvalidArgument, "no ID or payment hash provided")
}

func (server *routedBoltzServer) GetSwapInfoStream(request *boltzrpc.GetSwapInfoRequest, stream boltzrpc.Boltz_GetSwapInfoStreamServer) error {
	var updates <-chan nursery.SwapUpdate
	var stop func()

	// nolint: staticcheck
	if request.Id != "" {
		// nolint: staticcheck
		request.Identifier = &boltzrpc.GetSwapInfoRequest_SwapId{SwapId: request.Id}
	}
	if swapId := request.GetSwapId(); swapId == "" || swapId == "*" {
		logger.Info("Starting global Swap info stream")
		updates, stop = server.nursery.GlobalSwapUpdates()
	} else {
		info, err := server.GetSwapInfo(stream.Context(), request)
		if err != nil {
			return err
		}
		swapId := info.Swap.GetId() + info.ReverseSwap.GetId() + info.ChainSwap.GetId()
		logger.Info("Starting Swap info stream for " + swapId)
		updates, stop = server.nursery.SwapUpdates(swapId)
		if updates == nil {
			if err := stream.Send(info); err != nil {
				return err
			}
			return nil
		}
	}

	for update := range updates {
		response, err := server.serializeAnySwap(stream.Context(), update.Swap, update.ReverseSwap, update.ChainSwap)
		if err == nil {
			if err := stream.Send(response); err != nil {
				stop()
				return err
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
		return nil, err
	}

	return &boltzrpc.DepositResponse{
		Id:                 response.Id,
		Address:            response.Address,
		TimeoutBlockHeight: response.TimeoutBlockHeight,
	}, nil
}

func (server *routedBoltzServer) checkMagicRoutingHint(decoded *lightning.DecodedInvoice, invoice string) (*boltzrpc.CreateSwapResponse, error) {
	if pubKey := decoded.MagicRoutingHint; pubKey != nil {
		logger.Info("Found magic routing hint in invoice")
		reverseBip21, err := server.boltz.GetReverseSwapBip21(invoice)
		if err != nil {
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
		if amount > btcutil.Amount(decoded.AmountSat).ToBTC() {
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
	privateKey, publicKey, err := server.newKeys()
	if err != nil {
		return nil, err
	}

	pair := serializers.ParsePair(request.Pair)

	if request.AcceptedPair == nil {
		request.AcceptedPair, err = server.getSubmarinePair(request.Pair)
		if err != nil {
			return nil, err
		}
	}

	createSwap := boltz.CreateSwapRequest{
		From:            pair.From,
		To:              pair.To,
		PairHash:        request.AcceptedPair.Hash,
		RefundPublicKey: publicKey.SerializeCompressed(),
		ReferralId:      server.referralId,
	}
	var swapResponse *boltzrpc.CreateSwapResponse

	var preimage, preimageHash []byte
	if invoice := request.GetInvoice(); invoice != "" {
		if _, lnurlParams, err := lnurl.HandleLNURL(invoice); err == nil {
			if kind := lnurlParams.LNURLKind(); kind != "lnurl-pay" {
				return nil, status.Errorf(codes.InvalidArgument, "lnurl is not pay, but: %s", kind)
			}
			logger.Infof("Fetching invoice for LNURL: %s", invoice)
			lnurlPay := lnurlParams.(lnurl.LNURLPayParams)
			if request.Amount == 0 {
				return nil, status.Errorf(codes.InvalidArgument, "amount has to be specified for lnurl")
			}
			payValues, err := lnurlPay.Call(int64(request.Amount*1000), "", nil)
			if err != nil {
				return nil, err
			}
			invoice = payValues.PR
		} else if offer, err := lightning.DecodeOffer(invoice); err == nil {
			if request.Amount == 0 {
				return nil, status.Errorf(codes.InvalidArgument, "amount has to be specified for offer")
			}
			if request.Amount < offer.MinAmountSat {
				return nil, status.Errorf(codes.InvalidArgument, "amount is below offer minimum: %d < %d", request.Amount, offer.MinAmountSat)
			}
			logger.Infof("Fetching invoice from offer: %s", invoice)
			bolt12, err := server.boltz.FetchBolt12Invoice(invoice, request.Amount)
			if err != nil {
				return nil, fmt.Errorf("could not fetch bolt12 invoice: %w", err)
			}
			logger.Infof("Fetched bolt12 invoice: %s", bolt12)

			if !lightning.CheckInvoiceIsForOffer(bolt12, invoice) {
				return nil, status.Errorf(codes.InvalidArgument, "bolt12 offer does not match offer")
			}
			invoice = bolt12
		}
		logger.Infof("Creating Swap for invoice: %s", invoice)
		decoded, err := lightning.DecodeInvoice(invoice, server.network.Btc)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid invoice or lnurl: %s", err)
		}
		if !request.GetIgnoreMrh() {
			swapResponse, err = server.checkMagicRoutingHint(decoded, invoice)
			if err != nil {
				return nil, err
			}
		}
		preimageHash = decoded.PaymentHash[:]
		createSwap.Invoice = invoice
		// set amount for balance check
		if decoded.AmountSat == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "0 amount invoices are not supported")
		}
		request.Amount = decoded.AmountSat
	} else if !server.lightningAvailable(ctx) {
		return nil, errors.New("invoice is required in standalone mode")
	} else if request.Amount != 0 {
		logger.Infof("Creating Swap for %d sats", request.Amount)

		invoice, err := server.lightning.CreateInvoice(request.Amount, nil, 0, utils.GetSwapMemo(string(pair.From)))
		if err != nil {
			return nil, err
		}
		preimageHash = invoice.PaymentHash
		createSwap.Invoice = invoice.PaymentRequest
	} else {
		if request.SendFromInternal {
			return nil, errors.New("cannot auto send if amount is 0")
		}
		preimage, preimageHash, err = newPreimage()
		if err != nil {
			return nil, err
		}

		logger.Info("Creating Swap with preimage hash: " + hex.EncodeToString(preimageHash))

		createSwap.PreimageHash = preimageHash
	}

	feeRate, err := server.estimateFee(request.GetSatPerVbyte(), pair.From)
	if err != nil {
		return nil, err
	}

	wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{
		Currency:      pair.From,
		Id:            request.WalletId,
		AllowReadonly: !request.SendFromInternal,
	})
	if request.SendFromInternal {
		if err != nil {
			return nil, err
		}
		sendAmount := request.Amount + utils.CalculateFeeEstimate(request.AcceptedPair.Fees, request.Amount)
		if err := server.checkBalance(wallet, sendAmount, feeRate); err != nil {
			return nil, err
		}
		logger.Infof("Using wallet %+v to pay swap", wallet.GetWalletInfo())
	}

	if swapResponse == nil {
		if createSwap.Invoice != "" {
			existing, err := server.database.QuerySwapByInvoice(createSwap.Invoice)
			if err != nil {
				return nil, fmt.Errorf("could not query existing swap: %w", err)
			}
			if existing != nil {
				return nil, status.Errorf(codes.AlreadyExists, "swap %s has the same invoice", existing.Id)
			}
		}
		refundAddress := request.GetRefundAddress()
		if refundAddress != "" {
			if err := boltz.ValidateAddress(server.network, refundAddress, pair.From); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid refund address %s: %s", refundAddress, err)
			}
		}

		response, err := server.boltz.CreateSwap(createSwap)

		if err != nil {
			return nil, errors.New("boltz error: " + err.Error())
		}

		swap := database.Swap{
			Id:                  response.Id,
			Pair:                pair,
			State:               boltzrpc.SwapState_PENDING,
			Error:               "",
			PrivateKey:          privateKey,
			Preimage:            preimage,
			PaymentHash:         preimageHash,
			Invoice:             createSwap.Invoice,
			Address:             response.Address,
			ExpectedAmount:      response.ExpectedAmount,
			TimoutBlockHeight:   response.TimeoutBlockHeight,
			SwapTree:            response.SwapTree.Deserialize(),
			LockupTransactionId: "",
			RefundTransactionId: "",
			RefundAddress:       refundAddress,
			IsAuto:              isAuto,
			ServiceFeePercent:   boltz.Percentage(request.AcceptedPair.Fees.Percentage),
			TenantId:            requireTenantId(ctx),
		}

		logger.Infof("Created new Swap %s (IsAuto: %v, From: %s, Tenant: %s)", swap.Id, swap.IsAuto, pair.From, requireTenant(ctx).Name)

		if request.SendFromInternal {
			id := wallet.GetWalletInfo().Id
			swap.WalletId = &id
		}

		swap.ClaimPubKey, err = btcec.ParsePubKey([]byte(response.ClaimPublicKey))
		if err != nil {
			return nil, err
		}

		// for _, chanId := range request.ChanIds {
		// 	parsed, err := lightning.NewChanIdFromString(chanId)
		// 	if err != nil {
		// 		return nil, (errors.New("invalid channel id: " + err.Error()))
		// 	}
		// 	swap.ChanIds = append(swap.ChanIds, parsed)
		// }

		if pair.From == boltz.CurrencyLiquid {
			swap.BlindingKey, _ = btcec.PrivKeyFromBytes(response.BlindingKey)
		}

		if err := swap.InitTree(); err != nil {
			return nil, err
		}

		if err := swap.SwapTree.Check(boltz.NormalSwap, swap.TimoutBlockHeight, preimageHash); err != nil {
			return nil, err
		}

		if err := swap.SwapTree.CheckAddress(response.Address, server.network, swap.BlindingPubKey()); err != nil {
			return nil, err
		}

		if request.Amount != 0 {
			if err := server.nursery.CheckAmounts(boltz.NormalSwap, pair, response.ExpectedAmount, request.Amount, swap.ServiceFeePercent); err != nil {
				return nil, err
			}
		}

		logger.Debugf("Verified redeem script and address of Swap %s", swap.Id)

		err = server.database.CreateSwap(swap)
		if err != nil {
			return nil, err
		}

		blockHeight, err := server.onchain.GetBlockHeight(pair.From)
		if err != nil {
			return nil, err
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

		if request.SendFromInternal {
			swapResponse.TxId, err = wallet.SendToAddress(
				onchain.WalletSendArgs{
					Address:     swapResponse.Address,
					Amount:      swapResponse.ExpectedAmount,
					SatPerVbyte: feeRate,
				},
			)
			if err != nil {
				if dbErr := server.database.UpdateSwapState(&swap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
					logger.Error(dbErr.Error())
				}
				return nil, err
			}
		}

		if err := server.nursery.RegisterSwap(swap); err != nil {
			return nil, err
		}
	} else if request.SendFromInternal {
		swapResponse.TxId, err = wallet.SendToAddress(
			onchain.WalletSendArgs{
				Address:     swapResponse.Address,
				Amount:      swapResponse.ExpectedAmount,
				SatPerVbyte: feeRate,
			},
		)
		if err != nil {
			return nil, err
		}

		logger.Infof("Sent %d to address %s for MRH in: %s", swapResponse.ExpectedAmount, swapResponse.Address, swapResponse.TxId)
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
	pair := serializers.ParsePair(request.Pair)
	logger.Infof("Creating Reverse Swap for %d sats to %s", request.Amount, pair.To)

	externalPay := request.GetExternalPay()
	if !server.lightningAvailable(ctx) {
		if request.ExternalPay == nil {
			externalPay = true
		} else if !externalPay {
			return nil, errors.New("can not create reverse swap without external pay in standalone mode")
		}
	}

	returnImmediately := request.GetReturnImmediately()
	if externalPay {
		// only error if it was explicitly set to false, implicitly set to true otherwise
		if request.ReturnImmediately != nil && !returnImmediately {
			return nil, errors.New("can not wait for swap transaction when using external pay")
		} else {
			returnImmediately = true
		}
	}

	preimage, preimageHash, err := newPreimage()

	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := server.newKeys()

	if err != nil {
		return nil, err
	}

	if request.AcceptedPair == nil {
		request.AcceptedPair, err = server.getReversePair(request.Pair)
		if err != nil {
			return nil, err
		}
	}

	createRequest := boltz.CreateReverseSwapRequest{
		From:            pair.From,
		To:              pair.To,
		PairHash:        request.AcceptedPair.Hash,
		InvoiceAmount:   request.Amount,
		PreimageHash:    preimageHash,
		ClaimPublicKey:  publicKey.SerializeCompressed(),
		ReferralId:      server.referralId,
		Description:     request.GetDescription(),
		DescriptionHash: request.GetDescriptionHash(),
		InvoiceExpiry:   request.GetInvoiceExpiry(),
	}

	claimAddress := request.Address
	addMrh := request.GetAddMagicRoutingHint()
	if addMrh && (!externalPay || claimAddress != "") {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"magic routing hints can only be used with an internal wallet and the external pay flag",
		)
	}

	var walletId *database.Id
	if claimAddress != "" {
		if request.WalletId != nil {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"claim address and wallet id cannot be used together",
			)
		}
		err := boltz.ValidateAddress(server.network, claimAddress, pair.To)

		if err != nil {
			return nil, fmt.Errorf("invalid claim address %s: %w", claimAddress, err)
		}
		logger.Infof("Using claim address: %s", claimAddress)
	} else {
		wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{
			Currency:      pair.To,
			Id:            request.WalletId,
			AllowReadonly: true,
		})
		if err != nil {
			return nil, err
		}
		info := wallet.GetWalletInfo()
		logger.Infof("Using wallet %+v as reverse swap destination", info)
		walletId = &info.Id

		if addMrh {
			claimAddress, err = wallet.NewAddress()
			if err != nil {
				return nil, fmt.Errorf("could not get claim address from wallet: %w", err)
			}
			addressHash := sha256.Sum256([]byte(claimAddress))
			signature, err := schnorr.Sign(privateKey, addressHash[:])
			if err != nil {
				return nil, err
			}
			createRequest.AddressSignature = signature.Serialize()
			createRequest.Address = claimAddress
		}
	}

	response, err := server.boltz.CreateReverseSwap(createRequest)
	if err != nil {
		return nil, err
	}

	key, err := btcec.ParsePubKey(response.RefundPublicKey)
	if err != nil {
		return nil, err
	}

	if request.RoutingFeeLimitPpm != nil && externalPay {
		return nil, status.Errorf(codes.InvalidArgument, "max routing fee ppm is not supported when using external pay")
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
		ServiceFeePercent:   boltz.Percentage(request.AcceptedPair.Fees.Percentage),
		ExternalPay:         externalPay,
		WalletId:            walletId,
		TenantId:            requireTenantId(ctx),
		RoutingFeeLimitPpm:  request.RoutingFeeLimitPpm,
	}

	logger.Infof(
		"Created new Reverse Swap %s (IsAuto: %v, AcceptZeroConf: %v, Tenant: %s, ExternalPay: %v)",
		reverseSwap.Id, reverseSwap.IsAuto, reverseSwap.AcceptZeroConf, requireTenant(ctx).Name, externalPay,
	)

	for _, chanId := range request.ChanIds {
		parsed, err := lightning.NewChanIdFromString(chanId)
		if err != nil {
			return nil, errors.New("invalid channel id: " + err.Error())
		}
		reverseSwap.ChanIds = append(reverseSwap.ChanIds, parsed)
	}

	var blindingPubKey *btcec.PublicKey
	if reverseSwap.Pair.To == boltz.CurrencyLiquid {
		reverseSwap.BlindingKey, blindingPubKey = btcec.PrivKeyFromBytes(response.BlindingKey)
	}

	if err := reverseSwap.InitTree(); err != nil {
		return nil, err
	}

	if err := reverseSwap.SwapTree.Check(boltz.ReverseSwap, reverseSwap.TimeoutBlockHeight, preimageHash); err != nil {
		return nil, err
	}

	if err := reverseSwap.SwapTree.CheckAddress(response.LockupAddress, server.network, blindingPubKey); err != nil {
		return nil, err
	}

	if err := server.nursery.CheckAmounts(boltz.ReverseSwap, pair, request.Amount, reverseSwap.OnchainAmount, reverseSwap.ServiceFeePercent); err != nil {
		return nil, err
	}

	invoice, err := zpay32.Decode(reverseSwap.Invoice, server.network.Btc)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(preimageHash, invoice.PaymentHash[:]) {
		return nil, errors.New("invalid invoice preimage hash")
	}
	if invoice.MilliSat == nil {
		return nil, errors.New("invoice amount is missing")
	}
	reverseSwap.InvoiceAmount = uint64(invoice.MilliSat.ToSatoshis())

	logger.Debugf("Verified redeem script and invoice of Reverse Swap %s", reverseSwap.Id)

	err = server.database.CreateReverseSwap(reverseSwap)

	if err != nil {
		return nil, err
	}

	if err := server.nursery.RegisterReverseSwap(reverseSwap); err != nil {
		return nil, err
	}

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
				return nil, errors.New("reverse swap failed: " + info.Error)
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

	tenantId := requireTenantId(ctx)

	claimPrivateKey, claimPub, err := server.newKeys()
	if err != nil {
		return nil, err
	}

	refundPrivateKey, refundPub, err := server.newKeys()
	if err != nil {
		return nil, err
	}

	pair := serializers.ParsePair(request.Pair)
	amount := request.GetAmount()
	logger.Infof("Creating Chain Swap for %d sats from %s to %s", amount, pair.From, pair.To)

	if request.AcceptedPair == nil {
		request.AcceptedPair, err = server.getChainPair(request.Pair)
		if err != nil {
			return nil, err
		}
	}

	createChainSwap := boltz.ChainRequest{
		From:            pair.From,
		To:              pair.To,
		UserLockAmount:  amount,
		PairHash:        request.AcceptedPair.Hash,
		ClaimPublicKey:  claimPub.SerializeCompressed(),
		RefundPublicKey: refundPub.SerializeCompressed(),
		ReferralId:      server.referralId,
	}

	preimage, preimageHash, err := newPreimage()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Creating Chain Swap with preimage hash: %x", preimageHash)

	createChainSwap.PreimageHash = preimageHash
	if amount == 0 {
		if !request.GetExternalPay() {
			return nil, errors.New("cannot auto send if amount is 0")
		}
	}

	feeRate, err := server.estimateFee(request.GetSatPerVbyte(), pair.From)
	if err != nil {
		return nil, err
	}

	externalPay := request.GetExternalPay()
	var fromWallet, toWallet onchain.Wallet
	if request.FromWalletId != nil {
		fromWallet, err = server.getWallet(ctx, onchain.WalletChecker{
			Id:       request.FromWalletId,
			Currency: pair.From,
		})
		if err != nil {
			return nil, err
		}
		if err := server.checkBalance(fromWallet, amount, feeRate); err != nil {
			return nil, err
		}
		logger.Infof("Using wallet %+v to pay chain swap", fromWallet.GetWalletInfo())
	} else if !externalPay {
		return nil, errors.New("from wallet required if external pay is not specified")
	}

	if request.ToWalletId != nil {
		toWallet, err = server.getWallet(ctx, onchain.WalletChecker{
			Id:            request.ToWalletId,
			Currency:      pair.To,
			AllowReadonly: true,
		})
		if err != nil {
			return nil, err
		}
		logger.Infof("Using wallet %+v as chain swap destination", toWallet.GetWalletInfo())
	} else if request.ToAddress != nil {
		logger.Infof("Using address %+v as chain swap destination", request.GetToAddress())
	} else {
		return nil, errors.New("to address or to wallet required")
	}

	response, err := server.boltz.CreateChainSwap(createChainSwap)

	if err != nil {
		return nil, errors.New("boltz error: " + err.Error())
	}

	chainSwap := database.ChainSwap{
		Id:                response.Id,
		Pair:              pair,
		State:             boltzrpc.SwapState_PENDING,
		Error:             "",
		Preimage:          preimage,
		IsAuto:            isAuto,
		AcceptZeroConf:    request.GetAcceptZeroConf(),
		ServiceFeePercent: boltz.Percentage(request.AcceptedPair.Fees.Percentage),
		TenantId:          tenantId,
	}

	logger.Infof(
		"Created new Chain Swap %s (IsAuto: %v, AcceptZeroConf: %v, Tenant: %s, ExternalPay: %v)",
		response.Id, chainSwap.IsAuto, chainSwap.AcceptZeroConf, requireTenant(ctx).Name, externalPay,
	)

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
		return nil, err
	}
	if toWallet != nil {
		id := toWallet.GetWalletInfo().Id
		chainSwap.ToData.WalletId = &id
	}

	chainSwap.FromData, err = parseDetails(response.LockupDetails, pair.From)
	if err != nil {
		return nil, err
	}
	if !externalPay {
		id := fromWallet.GetWalletInfo().Id
		chainSwap.FromData.WalletId = &id
	}

	if request.Amount != nil {
		if err := server.nursery.CheckAmounts(boltz.ChainSwap, pair, chainSwap.FromData.Amount, chainSwap.ToData.Amount, chainSwap.ServiceFeePercent); err != nil {
			return nil, err
		}
	}

	logger.Debugf("Verified redeem script and address of Chain Swap %s", chainSwap.Id)

	err = server.database.CreateChainSwap(chainSwap)
	if err != nil {
		return nil, err
	}

	if !externalPay {
		from := chainSwap.FromData
		from.LockupTransactionId, err = fromWallet.SendToAddress(
			onchain.WalletSendArgs{
				Address:     from.LockupAddress,
				Amount:      from.Amount,
				SatPerVbyte: feeRate,
			},
		)
		if err != nil {
			if dbErr := server.database.UpdateChainSwapState(&chainSwap, boltzrpc.SwapState_ERROR, err.Error()); dbErr != nil {
				logger.Error(dbErr.Error())
			}
			return nil, err
		}
	}

	if err := server.nursery.RegisterChainSwap(chainSwap); err != nil {
		return nil, err
	}

	return serializeChainSwap(&chainSwap), nil
}

func (server *routedBoltzServer) importWallet(ctx context.Context, credentials *onchain.WalletCredentials, password string) error {
	decryptedCredentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		return status.Error(codes.InvalidArgument, "wrong password")
	}

	for _, existing := range decryptedCredentials {
		if existing.Name == credentials.Name && existing.TenantId == credentials.TenantId {
			return status.Errorf(codes.InvalidArgument, "wallet %s already exists", existing.Name)
		}
		if existing.Currency == credentials.Currency && existing.Mnemonic == credentials.Mnemonic && existing.Xpub == credentials.Xpub && existing.CoreDescriptor == credentials.CoreDescriptor {
			return status.Errorf(codes.InvalidArgument, "wallet %s has the same credentials", existing.Name)
		}
	}

	var imported onchain.Wallet
	err = server.database.RunTx(func(tx *database.Transaction) error {
		if backend, ok := server.walletBackends[credentials.Currency]; ok {
			if err := onchain.ValidateWalletCredentials(backend, credentials); err != nil {
				return err
			}
		}

		if err := tx.CreateWallet(&database.Wallet{WalletCredentials: credentials}); err != nil {
			return err
		}
		decryptedCredentials = append(decryptedCredentials, credentials)

		logger.Infof("Creating new wallet %s", credentials.WalletInfo)
		imported, err = server.loginWallet(credentials)
		if err != nil {
			return fmt.Errorf("could not login: %w", err)
		}

		if password != "" {
			if err := server.encryptWalletCredentials(tx, password, decryptedCredentials); err != nil {
				return fmt.Errorf("could not encrypt credentials: %w", err)
			}
		}

		server.onchain.AddWallet(imported)

		return nil
	})
	if err != nil {
		return err
	}

	// TODO: maybe allow returning without sync here
	return imported.Sync()
}

func (server *routedBoltzServer) ImportWallet(ctx context.Context, request *boltzrpc.ImportWalletRequest) (*boltzrpc.Wallet, error) {
	if request.Params == nil {
		return nil, errors.New("missing wallet parameters")
	}
	if err := checkName(request.Params.Name); err != nil {
		return nil, err
	}

	currency := serializers.ParseCurrency(&request.Params.Currency)
	credentials := &onchain.WalletCredentials{
		WalletInfo: onchain.WalletInfo{
			Name:     request.Params.Name,
			Currency: currency,
			TenantId: requireTenantId(ctx),
		},
		Mnemonic: request.Credentials.GetMnemonic(),
		//nolint:staticcheck
		Xpub:           request.Credentials.GetXpub(),
		CoreDescriptor: request.Credentials.GetCoreDescriptor(),
		//nolint:staticcheck
		Subaccount: request.Credentials.Subaccount,
	}

	if err := server.importWallet(ctx, credentials, request.Params.GetPassword()); err != nil {
		return nil, err
	}
	return server.GetWallet(ctx, &boltzrpc.GetWalletRequest{Id: &credentials.Id})
}

//nolint:staticcheck
func (server *routedBoltzServer) SetSubaccount(ctx context.Context, request *boltzrpc.SetSubaccountRequest) (*boltzrpc.Subaccount, error) {
	wallet, err := server.getGdkWallet(ctx, onchain.WalletChecker{Id: &request.WalletId})
	if err != nil {
		return nil, err
	}

	subaccountNumber, err := wallet.SetSubaccount(request.Subaccount)
	if err != nil {
		return nil, err
	}

	if err := server.database.SetWalletSubaccount(wallet.GetWalletInfo().Id, *subaccountNumber); err != nil {
		return nil, err
	}

	subaccount, err := wallet.GetSubaccount(*subaccountNumber)
	if err != nil {
		return nil, err
	}
	balance, err := wallet.GetBalance()
	if err != nil {
		return nil, err
	}
	return serializeWalletSubaccount(*subaccount, balance), nil
}

//nolint:staticcheck
func (server *routedBoltzServer) GetSubaccounts(ctx context.Context, request *boltzrpc.GetSubaccountsRequest) (*boltzrpc.GetSubaccountsResponse, error) {
	wallet, err := server.getGdkWallet(ctx, onchain.WalletChecker{Id: &request.WalletId})
	if err != nil {
		return nil, err
	}

	subaccounts, err := wallet.GetSubaccounts(true)
	if err != nil {
		return nil, err
	}

	//nolint:staticcheck
	response := &boltzrpc.GetSubaccountsResponse{}
	for _, subaccount := range subaccounts {
		balance, err := wallet.GetSubaccountBalance(subaccount.Pointer)
		if err != nil {
			logger.Errorf("failed to get balance for subaccount %+v: %v", subaccount, err.Error())
		}
		response.Subaccounts = append(response.Subaccounts, serializeWalletSubaccount(*subaccount, balance))
	}

	if subaccount, err := wallet.CurrentSubaccount(); err == nil {
		response.Current = &subaccount
	}
	return response, nil
}

func (server *routedBoltzServer) CreateWallet(ctx context.Context, request *boltzrpc.CreateWalletRequest) (*boltzrpc.CreateWalletResponse, error) {
	mnemonic, err := wallet.GenerateMnemonic()
	if err != nil {
		return nil, errors.New("could not generate new mnemonic: " + err.Error())
	}

	created, err := server.ImportWallet(ctx, &boltzrpc.ImportWalletRequest{
		Params: request.Params,
		Credentials: &boltzrpc.WalletCredentials{
			Mnemonic: &mnemonic,
		},
	})
	if err != nil {
		return nil, err
	}

	return &boltzrpc.CreateWalletResponse{
		Mnemonic: mnemonic,
		Wallet:   created,
	}, nil
}

func getTransactionType(swap *database.AnySwap, txId string) boltzrpc.TransactionType {
	if swap.RefundTransactionid == txId {
		return boltzrpc.TransactionType_REFUND
	}
	if swap.ClaimTransactionid == txId {
		return boltzrpc.TransactionType_CLAIM
	}
	if swap.LockupTransactionid == txId {
		return boltzrpc.TransactionType_LOCKUP
	}
	return boltzrpc.TransactionType_UNKNOWN
}

func (server *routedBoltzServer) ListWalletTransactions(ctx context.Context, request *boltzrpc.ListWalletTransactionsRequest) (*boltzrpc.ListWalletTransactionsResponse, error) {
	wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{Id: &request.Id, AllowReadonly: true})
	if err != nil {
		return nil, err
	}
	transactions, err := wallet.GetTransactions(request.GetLimit(), request.GetOffset())
	if err != nil {
		return nil, err
	}
	var txIds []string
	for _, tx := range transactions {
		txIds = append(txIds, tx.Id)
	}
	swaps, err := server.database.QuerySwapsByTransactions(database.SwapQuery{TenantId: macaroons.TenantIdFromContext(ctx)}, txIds)
	if err != nil {
		return nil, err
	}
	response := &boltzrpc.ListWalletTransactionsResponse{}
	for _, tx := range transactions {
		result := &boltzrpc.WalletTransaction{
			Id:            tx.Id,
			Timestamp:     tx.Timestamp.Unix(),
			BlockHeight:   tx.BlockHeight,
			BalanceChange: tx.BalanceChange,
		}
		if tx.IsConsolidation {
			result.Infos = append(result.Infos, &boltzrpc.TransactionInfo{Type: boltzrpc.TransactionType_CONSOLIDATION})
		}
		for _, output := range tx.Outputs {
			result.Outputs = append(result.Outputs, &boltzrpc.TransactionOutput{
				Address:      output.Address,
				Amount:       output.Amount,
				IsOurAddress: output.IsOurAddress,
			})
		}
		i := slices.IndexFunc(swaps, func(swap *database.AnySwap) bool {
			return swap.RefundTransactionid == tx.Id || swap.ClaimTransactionid == tx.Id || swap.LockupTransactionid == tx.Id
		})
		if i >= 0 {
			if request.GetExcludeSwapRelated() {
				continue
			}
			swap := swaps[i]
			info := &boltzrpc.TransactionInfo{SwapId: &swap.Id, Type: getTransactionType(swap, tx.Id)}
			result.Infos = append(result.Infos, info)
		}
		response.Transactions = append(response.Transactions, result)
	}
	return response, nil
}

func (server *routedBoltzServer) BumpTransaction(ctx context.Context, request *boltzrpc.BumpTransactionRequest) (*boltzrpc.BumpTransactionResponse, error) {
	swapId := request.GetSwapId()
	txId := request.GetTxId()
	var swaps []*database.AnySwap
	var err error
	if swapId != "" {
		swap, err := server.database.GetAnySwap(swapId)
		if err != nil {
			return nil, err
		}
		if swap.RefundTransactionid != "" {
			txId = swap.RefundTransactionid
		} else if swap.ClaimTransactionid != "" {
			txId = swap.ClaimTransactionid
		} else if swap.LockupTransactionid != "" {
			txId = swap.LockupTransactionid
		} else {
			return nil, status.Errorf(codes.NotFound, "swap %s has no transactions to bump", swapId)
		}
		swaps = []*database.AnySwap{swap}
	} else {
		swaps, err = server.database.QuerySwapsByTransactions(database.SwapQuery{}, []string{txId})
		if err != nil {
			return nil, err
		}
	}
	var currency boltz.Currency
	var transaction boltz.Transaction
	for _, currency = range []boltz.Currency{boltz.CurrencyBtc, boltz.CurrencyLiquid} {
		transaction, err = server.onchain.GetTransaction(currency, txId, nil, false)
		if transaction != nil {
			break
		}
	}
	if transaction == nil {
		return nil, status.Errorf(codes.NotFound, "transaction %s not found: %s", txId, err)
	}
	feeRate := request.GetSatPerVbyte()
	confirmed, err := server.onchain.IsTransactionConfirmed(currency, txId, false)
	if err != nil {
		return nil, err
	}
	if confirmed {
		return nil, status.Errorf(
			codes.FailedPrecondition, "transaction %s is already confirmed on %s", txId, currency,
		)
	}
	previousFee, err := server.onchain.GetTransactionFee(transaction)
	if err != nil {
		return nil, err
	}
	previousFeeRate := float64(previousFee) / float64(transaction.VSize())
	if feeRate == 0 {
		feeRate, err = server.onchain.EstimateFee(currency)
		if err != nil {
			return nil, err
		}
		// the new estimation should always be higher than the previous fee rate (why would you have to bump it then?)
		// but we doublecheck here
		feeRate = max(previousFeeRate+1, feeRate)
	} else if feeRate <= previousFeeRate {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"new fee rate has to be higher than the original transactions rate of %f", previousFeeRate,
		)
	}
	if len(swaps) > 0 {
		txType := getTransactionType(swaps[0], txId)
		if txType == boltzrpc.TransactionType_UNKNOWN {
			return nil, status.Errorf(codes.NotFound, "transaction %s is not part of a swap", txId)
		}
		if txType == boltzrpc.TransactionType_CLAIM || txType == boltzrpc.TransactionType_REFUND {
			return nil, status.Errorf(codes.Unimplemented, "claim and refund transactions cannot be bumped")
		}
	}
	checker := onchain.WalletChecker{
		TenantId:      macaroons.TenantIdFromContext(ctx),
		AllowReadonly: false,
		Currency:      currency,
	}
	for _, wallet := range server.onchain.GetWallets(checker) {
		tx, err := wallet.BumpTransactionFee(txId, feeRate)
		if err == nil {
			return &boltzrpc.BumpTransactionResponse{TxId: tx}, nil
		}
		if !errors.Is(err, errors.ErrUnsupported) && !strings.Contains(err.Error(), "not found") {
			return nil, err
		}
	}
	return nil, status.Errorf(codes.NotFound, "transaction %s does not belong to any wallet", txId)
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
			return nil, fmt.Errorf("could not get balance for wallet %s: %w", info.Name, err)
		}
	} else {
		result.Balance = serializers.SerializeWalletBalance(balance)
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
		return nil, err
	}

	return server.serializeWallet(wallet)
}

func (server *routedBoltzServer) GetWalletSendFee(ctx context.Context, request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
	wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{Id: &request.Id})
	if err != nil {
		return nil, err
	}
	feeRate := request.GetSatPerVbyte()
	currency := wallet.GetWalletInfo().Currency
	if feeRate == 0 {
		feeRate, err = server.onchain.EstimateFee(currency)
		if err != nil {
			return nil, err
		}
	}
	if request.Address == "" {
		request.Address = server.network.DummyLockupAddress[currency]
	}
	amount, fee, err := wallet.GetSendFee(onchain.WalletSendArgs{
		Address:     request.Address,
		Amount:      request.Amount,
		SatPerVbyte: feeRate,
		SendAll:     request.GetSendAll(),
	})
	if err != nil {
		return nil, err
	}
	return &boltzrpc.WalletSendFee{Amount: amount, Fee: fee, FeeRate: feeRate}, nil
}

func (server *routedBoltzServer) GetWallets(ctx context.Context, request *boltzrpc.GetWalletsRequest) (*boltzrpc.Wallets, error) {
	var response boltzrpc.Wallets
	checker := onchain.WalletChecker{
		Currency:      serializers.ParseCurrency(request.Currency),
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
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return found, nil
}

func (server *routedBoltzServer) GetWalletCredentials(ctx context.Context, request *boltzrpc.GetWalletCredentialsRequest) (*boltzrpc.WalletCredentials, error) {
	wallet, err := server.getWallet(ctx, onchain.WalletChecker{Id: &request.Id})
	if err != nil {
		return nil, err
	}
	info := wallet.GetWalletInfo()
	dbWallet, err := server.database.GetWallet(request.Id)
	if err != nil {
		return nil, fmt.Errorf("could not read credentials for wallet %s: %w", info.Name, err)
	}
	if dbWallet.NodePubkey != nil {
		return nil, errors.New("cant get credentials for node wallet")
	}
	if dbWallet.Encrypted() {
		dbWallet.WalletCredentials, err = dbWallet.Decrypt(request.GetPassword())
		if err != nil {
			return nil, fmt.Errorf("invalid password: %w", err)
		}
	}

	return serializeWalletCredentials(dbWallet.WalletCredentials), err
}

func (server *routedBoltzServer) RemoveWallet(ctx context.Context, request *boltzrpc.RemoveWalletRequest) (*boltzrpc.RemoveWalletResponse, error) {
	wallet, err := server.getAnyWallet(ctx, onchain.WalletChecker{
		Id:            &request.Id,
		AllowReadonly: true,
	})
	if err != nil {
		return nil, err
	}
	if server.swapper.WalletUsed(request.Id) {
		return nil, fmt.Errorf(
			"wallet %s is used in autoswap, configure a different wallet in autoswap before removing this wallet",
			wallet.GetWalletInfo().Name,
		)
	}
	if err := wallet.Disconnect(); err != nil {
		return nil, err
	}
	id := wallet.GetWalletInfo().Id
	if err := server.database.DeleteWallet(id); err != nil {
		return nil, err
	}
	server.onchain.RemoveWallet(id)

	logger.Infof("Removed wallet %s", wallet.GetWalletInfo())

	return &boltzrpc.RemoveWalletResponse{}, nil
}

func (server *routedBoltzServer) WalletSend(ctx context.Context, request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendResponse, error) {
	sendWallet, err := server.getWallet(ctx, onchain.WalletChecker{Id: &request.Id})
	if err != nil {
		return nil, err
	}
	feeRate, err := server.estimateFee(request.GetSatPerVbyte(), sendWallet.GetWalletInfo().Currency)
	if err != nil {
		return nil, err
	}
	if request.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "address required")
	}
	if request.Amount == 0 && !request.GetSendAll() {
		return nil, status.Errorf(codes.InvalidArgument, "amount required")
	}
	txId, err := sendWallet.SendToAddress(onchain.WalletSendArgs{
		Address:     request.Address,
		Amount:      request.Amount,
		SatPerVbyte: feeRate,
		SendAll:     request.GetSendAll(),
	})
	if err != nil {
		return nil, err
	}
	return &boltzrpc.WalletSendResponse{TxId: txId}, nil
}

func (server *routedBoltzServer) WalletReceive(ctx context.Context, request *boltzrpc.WalletReceiveRequest) (*boltzrpc.WalletReceiveResponse, error) {
	receiveWallet, err := server.getWallet(ctx, onchain.WalletChecker{Id: &request.Id, AllowReadonly: true})
	if err != nil {
		return nil, err
	}
	address, err := receiveWallet.NewAddress()
	if err != nil {
		return nil, err
	}
	return &boltzrpc.WalletReceiveResponse{Address: address}, nil
}

func (server *routedBoltzServer) Stop(context.Context, *empty.Empty) (*empty.Empty, error) {
	server.stateLock.Lock()
	defer server.stateLock.Unlock()
	if server.state == stateStopping {
		return &empty.Empty{}, nil
	}
	server.state = stateStopping
	if server.nursery != nil {
		server.nursery.Stop()
		logger.Debugf("Stopped nursery")
	}
	close(server.stop)
	return &empty.Empty{}, nil
}

func (server *routedBoltzServer) decryptWalletCredentials(password string) (decrypted []*onchain.WalletCredentials, err error) {
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

func (server *routedBoltzServer) encryptWalletCredentials(tx *database.Transaction, password string, credentials []*onchain.WalletCredentials) (err error) {
	for _, creds := range credentials {
		if password != "" {
			if creds, err = creds.Encrypt(password); err != nil {
				return err
			}
		}
		if err := tx.UpdateWalletCredentials(creds); err != nil {
			return err
		}
	}
	return nil
}

func (server *routedBoltzServer) Unlock(_ context.Context, request *boltzrpc.UnlockRequest) (*empty.Empty, error) {
	return &empty.Empty{}, server.unlock(request.Password)
}

func (server *routedBoltzServer) VerifyWalletPassword(_ context.Context, request *boltzrpc.VerifyWalletPasswordRequest) (*boltzrpc.VerifyWalletPasswordResponse, error) {
	_, err := server.decryptWalletCredentials(request.Password)
	return &boltzrpc.VerifyWalletPasswordResponse{Correct: err == nil}, nil
}

func (server *routedBoltzServer) fullInit() (err error) {
	if err := server.nursery.Init(); err != nil {
		return fmt.Errorf("could not start nursery: %v", err)
	}

	if err := server.swapper.LoadConfig(); err != nil {
		return fmt.Errorf("could not load autoswap config: %v", err)
	}
	return nil
}

func (server *routedBoltzServer) getState() serverState {
	server.stateLock.RLock()
	defer server.stateLock.RUnlock()
	return server.state
}

func (server *routedBoltzServer) setState(state serverState) {
	server.stateLock.Lock()
	defer server.stateLock.Unlock()
	server.state = state
}

func (server *routedBoltzServer) unlock(password string) error {
	credentials, err := server.decryptWalletCredentials(password)
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			server.stateLock.Lock()
			defer server.stateLock.Unlock()
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
				WalletCredentials: &onchain.WalletCredentials{
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

	server.setState(stateSyncing)
	go func() {
		defer func() {
			server.setState(stateUnlocked)
		}()
		var wg sync.WaitGroup
		wg.Add(len(credentials))
		for _, creds := range credentials {
			creds := creds
			go func() {
				defer wg.Done()
				wallet, err := server.loginWallet(creds)
				if err != nil {
					logger.Errorf("Could not login to wallet %s: %v", creds.String(), err)
				} else {
					logger.Debugf("Logged into wallet: %s", wallet.GetWalletInfo().String())
					if err := wallet.FullScan(); err != nil {
						logger.Errorf("Failed to full scan wallet %s: %v", wallet.GetWalletInfo().String(), err)
					}
					server.onchain.AddWallet(wallet)
				}
			}()
		}
		wg.Wait()

		for {
			version, err := server.boltz.GetVersion()
			if err != nil {
				logger.Errorf("Boltz backend is unavailable, retrying in 10 seconds: %v", err)
				server.setState(stateUnavailable)
				time.Sleep(time.Second * 10)
			} else {
				if err := checkBoltzVersion(version); err != nil {
					logger.Fatalf("Unsupported Boltz version: %v", err)
				}
				if err := server.fullInit(); err != nil {
					logger.Errorf("Failed to initialize: %v", err)
				}
				break
			}

		}
	}()

	return nil
}

func (server *routedBoltzServer) ChangeWalletPassword(_ context.Context, request *boltzrpc.ChangeWalletPasswordRequest) (*empty.Empty, error) {
	decrypted, err := server.decryptWalletCredentials(request.Old)
	if err != nil {
		return nil, err
	}

	return &empty.Empty{}, server.database.RunTx(func(tx *database.Transaction) error {
		return server.encryptWalletCredentials(tx, request.New, decrypted)
	})
}

func (server *routedBoltzServer) requestAllowed(fullMethod string) error {
	if strings.Contains(fullMethod, "Stop") {
		return nil
	}
	state := server.getState()
	if state == stateUnavailable {
		return status.Error(codes.Unavailable, "unavailable, please check logs for more information")
	}
	if state == stateLightningSyncing {
		return status.Errorf(codes.Unavailable, "connected lightning node is syncing, please wait")
	}
	if state == stateSyncing {
		return status.Error(codes.Unavailable, "boltzd is syncing its wallets, please wait")
	}
	if strings.Contains(fullMethod, "Unlock") {
		if state == stateUnlocked {
			return status.Errorf(codes.FailedPrecondition, "boltzd is already unlocked")
		}
	} else if state == stateLocked {
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

		response, err := handler(ctx, req)
		return response, handleError(err)
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

		return handleError(handler(srv, ss))
	}
}

func (server *routedBoltzServer) getGdkWallet(ctx context.Context, checker onchain.WalletChecker) (*wallet.Wallet, error) {
	existing, err := server.getWallet(ctx, checker)
	if err != nil {
		return nil, err
	}
	wallet, ok := existing.(*wallet.Wallet)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "operation not supported for wallet %s", existing.GetWalletInfo().Name)
	}
	return wallet, nil
}

func (server *routedBoltzServer) getSubmarinePair(request *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	pairsResponse, err := server.boltz.GetSubmarinePairs()
	if err != nil {
		return nil, err
	}
	pair := serializers.ParsePair(request)
	found, err := boltz.FindPair(pair, pairsResponse)
	return serializeSubmarinePair(pair, found), err
}

func (server *routedBoltzServer) getReversePair(request *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	pairsResponse, err := server.boltz.GetReversePairs()
	if err != nil {
		return nil, err
	}
	pair := serializers.ParsePair(request)
	found, err := boltz.FindPair(pair, pairsResponse)
	return serializeReversePair(pair, found), err
}

func (server *routedBoltzServer) getChainPair(request *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	pairsResponse, err := server.boltz.GetChainPairs()
	if err != nil {
		return nil, err
	}
	pair := serializers.ParsePair(request)
	found, err := boltz.FindPair(pair, pairsResponse)
	return serializeChainPair(pair, found), err
}

func (server *routedBoltzServer) GetPairs(context.Context, *empty.Empty) (*boltzrpc.GetPairsResponse, error) {
	response := &boltzrpc.GetPairsResponse{}

	eg := errgroup.Group{}
	eg.Go(func() error {
		submarinePairs, err := server.boltz.GetSubmarinePairs()
		if err != nil {
			return err
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
		return nil
	})

	eg.Go(func() error {
		reversePairs, err := server.boltz.GetReversePairs()
		if err != nil {
			return err
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
		return nil
	})

	eg.Go(func() error {
		chainPairs, err := server.boltz.GetChainPairs()
		if err != nil {
			return err
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
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return response, nil

}

func isAdmin(ctx context.Context) bool {
	id := macaroons.TenantIdFromContext(ctx)
	return id == nil || *id == database.DefaultTenantId
}

func (server *routedBoltzServer) BakeMacaroon(ctx context.Context, request *boltzrpc.BakeMacaroonRequest) (*boltzrpc.BakeMacaroonResponse, error) {

	if !isAdmin(ctx) {
		return nil, errors.New("only admin can bake macaroons")
	}

	if request.TenantId != nil {
		_, err := server.database.GetTenant(request.GetTenantId())
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "could not find tenant %d: %s", request.TenantId, err)
			}
			return nil, err
		}
	}

	permissions := macaroons.GetPermissions(request.TenantId != nil, request.Permissions)
	mac, err := server.macaroon.NewMacaroon(request.TenantId, permissions...)
	if err != nil {
		return nil, err
	}
	macBytes, err := mac.M().MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &boltzrpc.BakeMacaroonResponse{
		Macaroon: hex.EncodeToString(macBytes),
	}, nil
}

func (server *routedBoltzServer) CreateTenant(ctx context.Context, request *boltzrpc.CreateTenantRequest) (*boltzrpc.Tenant, error) {
	if request.Name == macaroons.TenantAll {
		return nil, status.Errorf(codes.InvalidArgument, "name is reserved")
	}
	tenant := &database.Tenant{Name: request.Name}

	if err := server.database.CreateTenant(tenant); err != nil {
		return nil, err
	}

	return serializeTenant(tenant), nil
}

func (server *routedBoltzServer) GetTenant(ctx context.Context, request *boltzrpc.GetTenantRequest) (*boltzrpc.Tenant, error) {
	tenant, err := server.database.GetTenantByName(request.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "tenant %s does not exist", request.Name)
		}
		return nil, err
	}

	return serializeTenant(tenant), nil
}

func (server *routedBoltzServer) ListTenants(ctx context.Context, request *boltzrpc.ListTenantsRequest) (*boltzrpc.ListTenantsResponse, error) {
	tenants, err := server.database.QueryTenants()
	if err != nil {
		return nil, err
	}

	response := &boltzrpc.ListTenantsResponse{}
	for _, tenant := range tenants {
		response.Tenants = append(response.Tenants, serializeTenant(tenant))
	}

	return response, nil
}

func (server *routedBoltzServer) GetSwapMnemonic(ctx context.Context, request *boltzrpc.GetSwapMnemonicRequest) (*boltzrpc.GetSwapMnemonicResponse, error) {
	swapMnemonic, err := server.database.GetSwapMnemonic()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "no swap mnemonic created yet")
		}
		return nil, err
	}

	return &boltzrpc.GetSwapMnemonicResponse{Mnemonic: swapMnemonic.Mnemonic}, nil
}

func (server *routedBoltzServer) SetSwapMnemonic(ctx context.Context, request *boltzrpc.SetSwapMnemonicRequest) (*boltzrpc.SetSwapMnemonicResponse, error) {
	server.newKeyLock.Lock()
	defer server.newKeyLock.Unlock()

	mnemonic := request.GetExisting()
	if mnemonic == "" {
		if !request.GetGenerate() {
			return nil, status.Errorf(codes.InvalidArgument, "existing mnemonic or generate must be set")
		}
		var err error
		mnemonic, err = wallet.GenerateMnemonic()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not generate mnemonic: %s", err.Error())
		}
	}

	// Validate the mnemonic by trying to derive a key from it
	_, err := boltz.DeriveKey(mnemonic, 0, server.network.Btc)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid mnemonic: %s", err)
	}

	err = server.database.RunTx(func(tx *database.Transaction) error {
		return tx.SetSwapMnemonic(mnemonic)
	})
	if err != nil {
		return nil, err
	}

	logger.Info("Updated swap mnemonic")
	return &boltzrpc.SetSwapMnemonicResponse{Mnemonic: mnemonic}, nil
}

func (server *routedBoltzServer) serializeAnySwap(ctx context.Context, swap *database.Swap, reverseSwap *database.ReverseSwap, chainSwap *database.ChainSwap) (*boltzrpc.GetSwapInfoResponse, error) {
	if tenantId := macaroons.TenantIdFromContext(ctx); tenantId != nil {
		err := status.Error(codes.PermissionDenied, "tenant does not have permission to view this swap")
		if swap != nil && swap.TenantId != *tenantId {
			return nil, err
		}
		if reverseSwap != nil && reverseSwap.TenantId != *tenantId {
			return nil, err
		}
		if chainSwap != nil && chainSwap.TenantId != *tenantId {
			return nil, err
		}
	}
	return &boltzrpc.GetSwapInfoResponse{
		Swap:        serializeSwap(swap),
		ReverseSwap: serializeReverseSwap(reverseSwap),
		ChainSwap:   serializeChainSwap(chainSwap),
	}, nil
}

func (server *routedBoltzServer) getPairs(pairId boltz.Pair) (*boltzrpc.Fees, *boltzrpc.Limits, error) {
	//nolint:staticcheck
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

func (server *routedBoltzServer) estimateFee(requested float64, currency boltz.Currency) (float64, error) {
	if requested == 0 {
		feeSatPerVbyte, err := server.onchain.EstimateFee(currency)
		if err != nil {
			return 0, err
		}
		logger.Infof("Using fee of %f sat/vbyte", feeSatPerVbyte)
		return feeSatPerVbyte, nil
	}
	return requested, nil
}

func (server *routedBoltzServer) checkBalance(check onchain.Wallet, sendAmount uint64, feeRate float64) error {
	balance, err := check.GetBalance()
	if err != nil {
		return err
	}
	info := check.GetWalletInfo()
	dummyAddress := server.network.DummyLockupAddress[info.Currency]
	_, _, err = check.GetSendFee(onchain.WalletSendArgs{
		Address:     dummyAddress,
		Amount:      sendAmount,
		SatPerVbyte: feeRate,
	})
	if errors.Is(err, errors.ErrUnsupported) {
		if balance.Confirmed < sendAmount {
			return info.InsufficientBalanceError(sendAmount)
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (server *routedBoltzServer) loginWallet(credentials *onchain.WalletCredentials) (onchain.Wallet, error) {
	if !credentials.Legacy {
		if backend, ok := server.walletBackends[credentials.Currency]; ok {
			return backend.NewWallet(credentials)
		}
	}
	return wallet.Login(credentials)
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

func (server *routedBoltzServer) newKeys() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	server.newKeyLock.Lock()
	defer server.newKeyLock.Unlock()

	var privateKey *btcec.PrivateKey
	mnemonic, err := server.database.GetSwapMnemonic()
	if err != nil {
		return nil, nil, status.Errorf(codes.FailedPrecondition, "swap mnemonic not set")
	}

	privateKey, err = boltz.DeriveKey(mnemonic.Mnemonic, mnemonic.LastKeyIndex, server.network.Btc)
	if err != nil {
		return nil, nil, err
	}

	if err := server.database.IncrementSwapMnemonicKey(mnemonic.Mnemonic); err != nil {
		return nil, nil, err
	}
	return privateKey, privateKey.PubKey(), nil
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

func checkName(name string) error {
	if name == "" {
		return errors.New("wallet name must not be empty")
	}
	if matched, err := regexp.MatchString("[^a-zA-Z\\d_-]", name); matched || err != nil {
		return errors.New("wallet name must only contain alphabetic characters, numbers, hyphens, and underscores")
	}
	return nil
}
