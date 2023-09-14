package rpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lightning"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	symbol      string
	chainParams *chaincfg.Params

	lightning lightning.LightningNode
	lnd       *lnd.LND
	boltz     *boltz.Boltz
	nursery   *nursery.Nursery
	database  *database.Database
}

func handleError(err error) error {
	if err != nil {
		logger.Warning("RPC request failed: " + err.Error())
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

	return &boltzrpc.GetInfoResponse{
		Symbol:              server.symbol,
		Network:             server.chainParams.Name,
		LndPubkey:           lightningInfo.Pubkey,
		BlockHeight:         lightningInfo.BlockHeight,
		PendingSwaps:        pendingSwapIds,
		PendingReverseSwaps: pendingReverseSwapIds,
	}, nil
}

func (server *routedBoltzServer) GetServiceInfo(_ context.Context, _ *boltzrpc.GetServiceInfoRequest) (*boltzrpc.GetServiceInfoResponse, error) {
	fees, limits, err := server.getPairs()

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

func (server *routedBoltzServer) ListSwaps(_ context.Context, _ *boltzrpc.ListSwapsRequest) (*boltzrpc.ListSwapsResponse, error) {
	response := &boltzrpc.ListSwapsResponse{}

	swaps, err := server.database.QuerySwaps()

	if err != nil {
		return nil, err
	}

	for _, swap := range swaps {
		// Check for a Channel Creation
		channelCreation, err := server.database.QueryChannelCreation(swap.Id)

		if err == nil {
			response.ChannelCreations = append(response.ChannelCreations, &boltzrpc.CombinedChannelSwapInfo{
				Swap:            serializeSwap(&swap),
				ChannelCreation: serializeChannelCreation(channelCreation),
			})
		} else {
			response.Swaps = append(response.Swaps, serializeSwap(&swap))
		}
	}

	// Reverse Swaps
	reverseSwaps, err := server.database.QueryReverseSwaps()

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
		var grpcChannelCreation *boltzrpc.ChannelCreationInfo

		channelCreation, err := server.database.QueryChannelCreation(swap.Id)

		if err == nil {
			grpcChannelCreation = serializeChannelCreation(channelCreation)
		}

		return &boltzrpc.GetSwapInfoResponse{
			Swap:            serializeSwap(swap),
			ChannelCreation: grpcChannelCreation,
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

func (server *routedBoltzServer) Deposit(_ context.Context, request *boltzrpc.DepositRequest) (*boltzrpc.DepositResponse, error) {
	preimage, preimageHash, err := newPreimage()

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Creating Swap with preimage hash: " + hex.EncodeToString(preimageHash))

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, handleError(err)
	}

	response, err := server.boltz.CreateChannelCreation(boltz.CreateChannelCreationRequest{
		Type:            "submarine",
		PairId:          server.symbol + "/" + server.symbol,
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(preimageHash),
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),

		Channel: boltz.Channel{
			Auto:             true,
			Private:          false,
			InboundLiquidity: getDefaultInboundLiquidity(request.InboundLiquidity),
		},
	})

	if err != nil {
		return nil, handleError(err)
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, handleError(err)
	}

	deposit := database.Swap{
		Id:                  response.Id,
		PairId:              request.PairId,
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		Status:              boltz.SwapCreated,
		PrivateKey:          privateKey,
		Preimage:            preimage,
		RedeemScript:        redeemScript,
		Invoice:             "",
		Address:             response.Address,
		ExpectedAmount:      0,
		TimoutBlockHeight:   response.TimeoutBlockHeight,
		LockupTransactionId: "",
		RefundTransactionId: "",
	}

	err = boltz.CheckSwapScript(deposit.RedeemScript, preimageHash, deposit.PrivateKey, deposit.TimoutBlockHeight)

	if err != nil {
		return nil, handleError(err)
	}

	err = boltz.CheckSwapAddress(server.chainParams, deposit.Address, deposit.RedeemScript, true)

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Verified redeem script and address of Swap " + deposit.Id)

	err = server.database.CreateSwap(deposit)

	if err != nil {
		return nil, handleError(err)
	}

	server.nursery.RegisterSwap(&deposit, nil)

	logger.Info("Created new Swap " + deposit.Id + ": " + marshalJson(deposit.Serialize()))

	return &boltzrpc.DepositResponse{
		Id:                 response.Id,
		Address:            deposit.Address,
		TimeoutBlockHeight: uint32(deposit.TimoutBlockHeight),
	}, nil
}

// TODO: custom refund address
// TODO: automatic sending from LND wallet
func (server *routedBoltzServer) CreateSwap(_ context.Context, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	logger.Info("Creating Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	invoice, err := server.lightning.CreateInvoice(int64(request.Amount), nil, 0, utils.GetSwapMemo(server.symbol))

	if err != nil {
		return nil, handleError(err)
	}

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, handleError(err)
	}

	response, err := server.boltz.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          request.PairId,
		OrderSide:       "buy",
		Invoice:         invoice.PaymentRequest,
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
	})

	if err != nil {
		return nil, handleError(err)
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, handleError(err)
	}

	swap := database.Swap{
		Id:                  response.Id,
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		Status:              boltz.InvoiceSet,
		PrivateKey:          privateKey,
		Preimage:            nil,
		RedeemScript:        redeemScript,
		Invoice:             invoice.PaymentRequest,
		Address:             response.Address,
		ExpectedAmount:      response.ExpectedAmount,
		TimoutBlockHeight:   response.TimeoutBlockHeight,
		LockupTransactionId: "",
		RefundTransactionId: "",
	}

	err = boltz.CheckSwapScript(swap.RedeemScript, invoice.PaymentHash, swap.PrivateKey, swap.TimoutBlockHeight)

	if err != nil {
		return nil, handleError(err)
	}

	err = boltz.CheckSwapAddress(server.chainParams, swap.Address, swap.RedeemScript, true)

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Verified redeem script and address of Swap " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, handleError(err)
	}

	server.nursery.RegisterSwap(&swap, nil)

	logger.Info("Created new Swap " + swap.Id + ": " + marshalJson(swap.Serialize()))

	return &boltzrpc.CreateSwapResponse{
		Id:             swap.Id,
		Address:        response.Address,
		ExpectedAmount: int64(response.ExpectedAmount),
		Bip21:          response.Bip21,
	}, nil
}

func (server *routedBoltzServer) CreateChannel(_ context.Context, request *boltzrpc.CreateChannelRequest) (*boltzrpc.CreateSwapResponse, error) {
	channelCreationType := "public"

	if request.Private {
		channelCreationType = "private"
	}

	logger.Info("Creating a " + channelCreationType + " Channel Creation with " +
		strconv.FormatUint(uint64(request.InboundLiquidity), 10) + "% inbound liquidity for " +
		strconv.FormatInt(request.Amount, 10) + " satoshis")

	preimage, preimageHash, err := newPreimage()

	if err != nil {
		return nil, handleError(err)
	}

	invoice, err := server.lnd.AddHoldInvoice(
		preimageHash,
		int64(request.Amount),
		// TODO: query timeout block delta from API
		utils.CalculateInvoiceExpiry(144, utils.GetBlockTime(server.symbol)),
		"Channel Creation from "+server.symbol,
	)

	if err != nil {
		return nil, handleError(err)
	}

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, handleError(err)
	}

	inboundLiquidity := getDefaultInboundLiquidity(request.InboundLiquidity)

	response, err := server.boltz.CreateChannelCreation(boltz.CreateChannelCreationRequest{
		Type:            "submarine",
		PairId:          server.symbol + "/" + server.symbol,
		OrderSide:       "buy",
		Invoice:         invoice.PaymentRequest,
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
		Channel: boltz.Channel{
			Auto:             false,
			Private:          request.Private,
			InboundLiquidity: inboundLiquidity,
		},
	})

	if err != nil {
		return nil, handleError(err)
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, handleError(err)
	}

	swap := database.Swap{
		Id:                  response.Id,
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		Status:              boltz.InvoiceSet,
		PrivateKey:          privateKey,
		Preimage:            preimage,
		RedeemScript:        redeemScript,
		Invoice:             invoice.PaymentRequest,
		Address:             response.Address,
		ExpectedAmount:      response.ExpectedAmount,
		TimoutBlockHeight:   response.TimeoutBlockHeight,
		LockupTransactionId: "",
		RefundTransactionId: "",
	}

	channelCreation := database.ChannelCreation{
		SwapId:                 response.Id,
		Status:                 boltz.ChannelNone,
		InboundLiquidity:       inboundLiquidity,
		Private:                request.Private,
		FundingTransactionId:   "",
		FundingTransactionVout: 0,
	}

	err = boltz.CheckSwapScript(swap.RedeemScript, preimageHash, swap.PrivateKey, swap.TimoutBlockHeight)

	if err != nil {
		return nil, handleError(err)
	}

	err = boltz.CheckSwapAddress(server.chainParams, swap.Address, swap.RedeemScript, true)

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Verified redeem script and address of Channel Creation " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, handleError(err)
	}

	err = server.database.CreateChannelCreation(channelCreation)

	if err != nil {
		return nil, handleError(err)
	}

	server.nursery.RegisterSwap(&swap, &channelCreation)

	logger.Info("Created new Channel Creation " + swap.Id + ": " + marshalJson(swap.Serialize()) + "\n" + marshalJson(channelCreation.Serialize()))

	return &boltzrpc.CreateSwapResponse{
		Id:             swap.Id,
		Address:        response.Address,
		ExpectedAmount: int64(response.ExpectedAmount),
		Bip21:          response.Bip21,
	}, nil
}

func (server *routedBoltzServer) CreateReverseSwap(_ context.Context, request *boltzrpc.CreateReverseSwapRequest) (*boltzrpc.CreateReverseSwapResponse, error) {
	logger.Info("Creating Reverse Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	claimAddress := request.Address

	if claimAddress != "" {
		_, err := btcutil.DecodeAddress(claimAddress, server.chainParams)

		if err != nil {
			return nil, handleError(err)
		}
	} else {
		var err error
		claimAddress, err = server.lightning.NewAddress()

		if err != nil {
			return nil, handleError(err)
		}

		logger.Info("Got claim address from LND: " + claimAddress)
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

	response, err := server.boltz.CreateReverseSwap(boltz.CreateReverseSwapRequest{
		Type:           "reverseSubmarine",
		PairId:         request.PairId,
		OrderSide:      "buy",
		InvoiceAmount:  uint64(request.Amount),
		PreimageHash:   hex.EncodeToString(preimageHash),
		ClaimPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
	})

	if err != nil {
		return nil, handleError(err)
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, handleError(err)
	}

	reverseSwap := database.ReverseSwap{
		Id:                  response.Id,
		PairId:              request.PairId,
		Status:              boltz.SwapCreated,
		AcceptZeroConf:      request.AcceptZeroConf,
		PrivateKey:          privateKey,
		Preimage:            preimage,
		RedeemScript:        redeemScript,
		Invoice:             response.Invoice,
		ClaimAddress:        claimAddress,
		OnchainAmount:       response.OnchainAmount,
		TimeoutBlockHeight:  response.TimeoutBlockHeight,
		LockupTransactionId: "",
		ClaimTransactionId:  "",
	}

	err = boltz.CheckReverseSwapScript(reverseSwap.RedeemScript, preimageHash, privateKey, response.TimeoutBlockHeight)

	if err != nil {
		return nil, handleError(err)
	}

	invoice, err := zpay32.Decode(reverseSwap.Invoice, server.chainParams)

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

	// TODO: error handling in case the swap fails
	var claimTransactionIdChan chan string

	if request.AcceptZeroConf {
		claimTransactionIdChan = make(chan string)
	}

	server.nursery.RegisterReverseSwap(reverseSwap, claimTransactionIdChan)

	logger.Info("Created new Reverse Swap " + reverseSwap.Id + ": " + marshalJson(reverseSwap.Serialize()))

	payment, err := server.payInvoice(reverseSwap.Invoice, reverseSwap.Id)

	if err != nil {
		dbErr := server.database.UpdateReverseSwapState(&reverseSwap, boltzrpc.SwapState_ERROR, err.Error())

		if dbErr != nil {
			return nil, handleError(dbErr)
		}

		return nil, handleError(err)
	}

	claimTransactionId := ""

	if claimTransactionIdChan != nil {
		claimTransactionId = <-claimTransactionIdChan
	}

	return &boltzrpc.CreateReverseSwapResponse{
		Id:                 reverseSwap.Id,
		LockupAddress:      response.LockupAddress,
		RoutingFeeMilliSat: uint32(payment),
		ClaimTransactionId: claimTransactionId,
	}, nil
}

func (server *routedBoltzServer) payInvoice(invoice string, id string) (uint, error) {
	feeLimit, err := lightning.GetFeeLimit(invoice, server.chainParams)

	if err != nil {
		return 0, err
	}

	payment, err := server.lightning.PayInvoice(invoice, feeLimit, 30)

	if err != nil {
		return 0, err
	}

	logger.Info("Paid invoice of Reverse Swap " + id + " with fee of " + utils.FormatMilliSat(int64(payment.FeeMsat)) + " satoshis")

	return payment.FeeMsat, nil
}

func (server *routedBoltzServer) getPairs() (*boltzrpc.Fees, *boltzrpc.Limits, error) {
	pairsResponse, err := server.boltz.GetPairs()

	if err != nil {
		return nil, nil, err
	}

	pairSymbol := server.symbol + "/" + server.symbol
	pair, hasPair := pairsResponse.Pairs[pairSymbol]

	if !hasPair {
		return nil, nil, errors.New("could not find pair with symbol " + pairSymbol)
	}

	minerFees := pair.Fees.MinerFees.BaseAsset

	return &boltzrpc.Fees{
			Percentage: pair.Fees.Percentage,
			Miner: &boltzrpc.MinerFees{
				Normal:  uint32(minerFees.Normal),
				Reverse: uint32(minerFees.Reverse.Lockup + minerFees.Reverse.Claim),
			},
		}, &boltzrpc.Limits{
			Minimal: int64(pair.Limits.Minimal),
			Maximal: int64(pair.Limits.Maximal),
		}, nil
}

func getDefaultInboundLiquidity(inboundLiquidity uint32) uint32 {
	if inboundLiquidity == 0 {
		return 25
	}

	return inboundLiquidity
}

func calculateDepositLimit(limit int64, fees *boltzrpc.Fees, isMin bool) int64 {
	effectiveRate := 1 + float64(fees.Percentage)/100
	limitFloat := float64(limit) * effectiveRate

	if isMin {
		// Add two more sats as safety buffer
		limitFloat = math.Ceil(limitFloat) + 2
	} else {
		limitFloat = math.Floor(limitFloat)
	}

	return int64(limitFloat) + int64(fees.Miner.Normal)
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
