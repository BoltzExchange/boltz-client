package rpcserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/zpay32"
	"strconv"
)

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	symbol      string
	chainParams *chaincfg.Params

	lnd      *lnd.LND
	boltz    *boltz.Boltz
	nursery  *nursery.Nursery
	database *database.Database
}

// TODO: use wrappers to handle RPC commands to also print errors in daemon logs

func (server *routedBoltzServer) GetInfo(_ context.Context, _ *boltzrpc.GetInfoRequest) (*boltzrpc.GetInfoResponse, error) {
	lndInfo, err := server.lnd.GetInfo()

	if err != nil {
		return nil, err
	}

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

	return &boltzrpc.GetInfoResponse{
		Symbol:              server.symbol,
		LndPubkey:           lndInfo.IdentityPubkey,
		PendingSwaps:        pendingSwapIds,
		PendingReverseSwaps: pendingReverseSwapIds,
	}, nil
}

// TODO: support reverse swaps
func (server *routedBoltzServer) GetSwapInfo(_ context.Context, request *boltzrpc.GetSwapInfoRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	swap, err := server.database.QuerySwap(request.Id)

	if err == nil {
		serializedSwap := swap.Serialize()

		return &boltzrpc.GetSwapInfoResponse{
			Swap: &boltzrpc.SwapInfo{
				Id:                 serializedSwap.Id,
				Status:             serializedSwap.Status,
				PrivateKey:         serializedSwap.PrivateKey,
				Preimage:           serializedSwap.Preimage,
				RedeemScript:       serializedSwap.RedeemScript,
				Invoice:            serializedSwap.Invoice,
				LockupAddress:      serializedSwap.Address,
				ExpectedAmount:     int64(serializedSwap.ExpectedAmount),
				TimeoutBlockHeight: uint32(serializedSwap.TimeoutBlockHeight),
			},
		}, nil
	}

	// Try to find a Reverse Swap with that ID
	reverseSwap, err := server.database.QueryReverseSwap(request.Id)

	if err == nil {
		serializedReverseSwap := reverseSwap.Serialize()

		return &boltzrpc.GetSwapInfoResponse{
			ReverseSwap: &boltzrpc.ReverseSwapInfo{
				Id:                 serializedReverseSwap.Id,
				Status:             serializedReverseSwap.Status,
				PrivateKey:         serializedReverseSwap.PrivateKey,
				Preimage:           serializedReverseSwap.Preimage,
				RedeemScript:       serializedReverseSwap.RedeemScript,
				Invoice:            serializedReverseSwap.Invoice,
				ClaimAddress:       serializedReverseSwap.ClaimAddress,
				OnchainAmount:      int64(serializedReverseSwap.OnchainAmount),
				TimeoutBlockHeight: uint32(serializedReverseSwap.TimeoutBlockHeight),
			},
		}, nil
	}

	return nil, errors.New("could not find Swap or Reverse Swap with ID " + request.Id)
}

// TODO: custom refund address
// TODO: automatically sending from LND wallet
func (server *routedBoltzServer) CreateSwap(_ context.Context, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	logger.Info("Creating Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	invoice, err := server.lnd.AddInvoice(request.Amount, "Submarine Swap from "+server.symbol)

	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, err
	}

	response, err := server.boltz.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          server.symbol + "/" + server.symbol,
		OrderSide:       "buy",
		Invoice:         invoice.PaymentRequest,
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
	})

	if err != nil {
		return nil, err
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, err
	}

	swap := database.Swap{
		Id:                response.Id,
		Status:            boltz.InvoiceSet,
		PrivateKey:        privateKey,
		Preimage:          nil,
		RedeemScript:      redeemScript,
		Invoice:           invoice.PaymentRequest,
		Address:           response.Address,
		ExpectedAmount:    response.ExpectedAmount,
		TimoutBlockHeight: response.TimeoutBlockHeight,
	}

	err = boltz.CheckSwapScript(swap.RedeemScript, invoice.RHash, swap.PrivateKey, swap.TimoutBlockHeight)

	if err != nil {
		return nil, err
	}

	err = boltz.CheckSwapAddress(server.chainParams, swap.Address, swap.RedeemScript, true)

	if err != nil {
		return nil, err
	}

	logger.Info("Verified redeem script and address of Swap " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, err
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
		return nil, err
	}

	invoice, err := server.lnd.AddHoldInvoice(preimageHash, request.Amount, "Channel Creation from "+server.symbol)

	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, err
	}

	response, err := server.boltz.CreateChannelCreation(boltz.CreateChannelCreationRequest{
		Type:            "submarine",
		PairId:          server.symbol + "/" + server.symbol,
		OrderSide:       "buy",
		Invoice:         invoice.PaymentRequest,
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
		Channel: boltz.Channel{
			Auto:             false,
			Private:          request.Private,
			InboundLiquidity: request.InboundLiquidity,
		},
	})

	if err != nil {
		return nil, err
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, err
	}

	swap := database.Swap{
		Id:                response.Id,
		Status:            boltz.InvoiceSet,
		PrivateKey:        privateKey,
		Preimage:          preimage,
		RedeemScript:      redeemScript,
		Invoice:           invoice.PaymentRequest,
		Address:           response.Address,
		ExpectedAmount:    response.ExpectedAmount,
		TimoutBlockHeight: response.TimeoutBlockHeight,
	}

	channelCreation := database.ChannelCreation{
		SwapId:                 response.Id,
		Status:                 boltz.ChannelNone,
		InboundLiquidity:       int(request.InboundLiquidity),
		Private:                request.Private,
		FundingTransactionId:   "",
		FundingTransactionVout: 0,
	}

	err = boltz.CheckSwapScript(swap.RedeemScript, preimageHash, swap.PrivateKey, swap.TimoutBlockHeight)

	if err != nil {
		return nil, err
	}

	err = boltz.CheckSwapAddress(server.chainParams, swap.Address, swap.RedeemScript, true)

	if err != nil {
		return nil, err
	}

	logger.Info("Verified redeem script and address of Channel Creation " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, err
	}

	err = server.database.CreateChannelCreation(channelCreation)

	if err != nil {
		return nil, err
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
			return nil, err
		}
	} else {
		var err error
		claimAddress, err = server.lnd.NewAddress()

		if err != nil {
			return nil, err
		}

		logger.Info("Got claim address from LND: " + claimAddress)
	}

	preimage, preimageHash, err := newPreimage()

	if err != nil {
		return nil, err
	}

	logger.Info("Generated preimage " + hex.EncodeToString(preimage))

	privateKey, publicKey, err := newKeys()

	if err != nil {
		return nil, err
	}

	response, err := server.boltz.CreateReverseSwap(boltz.CreateReverseSwapRequest{
		Type:           "reverseSubmarine",
		PairId:         server.symbol + "/" + server.symbol,
		OrderSide:      "buy",
		InvoiceAmount:  int(request.Amount),
		PreimageHash:   hex.EncodeToString(preimageHash),
		ClaimPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
	})

	if err != nil {
		return nil, err
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, err
	}

	reverseSwap := database.ReverseSwap{
		Id:                 response.Id,
		Status:             boltz.SwapCreated,
		PrivateKey:         privateKey,
		Preimage:           preimage,
		ClaimAddress:       claimAddress,
		RedeemScript:       redeemScript,
		Invoice:            response.Invoice,
		OnchainAmount:      response.OnchainAmount,
		TimeoutBlockHeight: response.TimeoutBlockHeight,
	}

	err = boltz.CheckReverseSwapScript(reverseSwap.RedeemScript, preimageHash, privateKey, response.TimeoutBlockHeight)

	if err != nil {
		return nil, err
	}

	invoice, err := zpay32.Decode(reverseSwap.Invoice, server.chainParams)

	if err != nil {
		return nil, err
	}

	if !bytes.Equal(preimageHash, invoice.PaymentHash[:]) {
		return nil, errors.New("invalid invoice preimage hash")
	}

	logger.Info("Verified redeem script and invoice of Reverse Swap " + reverseSwap.Id)

	err = server.database.CreateReverseSwap(reverseSwap)

	if err != nil {
		return nil, err
	}

	server.nursery.RegisterReverseSwap(reverseSwap)

	logger.Info("Created new Reverse Swap " + reverseSwap.Id + ": " + marshalJson(reverseSwap.Serialize()))

	payment, err := server.lnd.PayInvoice(reverseSwap.Invoice, 3, 30)

	if err != nil {
		return nil, err
	}

	// TODO: test this on testnet
	satsFee := strconv.FormatFloat(float64(payment.FeeMsat)*1000, 'f', 3, 64)
	logger.Info("Paid invoice of Reverse Swap " + reverseSwap.Id + " with fee of " + satsFee + " satoshis")

	return &boltzrpc.CreateReverseSwapResponse{
		Id: reverseSwap.Id,
	}, nil
}

func newKeys() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privateKey, err := btcec.NewPrivateKey(btcec.S256())

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
