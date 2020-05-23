package rpcserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/nursery"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/logger"
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

	return &boltzrpc.GetInfoResponse{
		Symbol:       server.symbol,
		LndPubkey:    lndInfo.IdentityPubkey,
		PendingSwaps: pendingSwapIds,
	}, nil
}

func (server *routedBoltzServer) GetSwapInfo(_ context.Context, request *boltzrpc.GetSwapInfoRequest) (*boltzrpc.GetSwapInfoResponse, error) {
	swap, err := server.database.QuerySwap(request.Id)

	if err != nil {
		return nil, err
	}

	serializedSwap := swap.Serialize()

	return &boltzrpc.GetSwapInfoResponse{
		Id:                 serializedSwap.Id,
		Status:             serializedSwap.Status,
		PrivateKey:         serializedSwap.PrivateKey,
		Preimage:           serializedSwap.Preimage,
		RedeemScript:       serializedSwap.RedeemScript,
		Invoice:            serializedSwap.Invoice,
		Address:            serializedSwap.Address,
		ExpectedAmount:     int64(serializedSwap.ExpectedAmount),
		TimeoutBlockHeight: uint32(serializedSwap.TimeoutBlockHeight),
	}, nil
}

// TODO: custom refund address
// TODO: automatically sending from LND wallet
func (server *routedBoltzServer) CreateSwap(_ context.Context, request *boltzrpc.CreateSwapRequest) (*boltzrpc.CreateSwapResponse, error) {
	logger.Info("Creating Swap for " + strconv.FormatInt(request.Amount, 10) + " satoshis")

	invoice, err := server.lnd.AddInvoice(request.Amount, "Submarine Swap to "+server.symbol)

	if err != nil {
		return nil, err
	}

	privateKey, publicKey, err := getKeys()

	if err != nil {
		return nil, err
	}

	response, err := server.boltz.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          server.symbol + "/" + server.symbol,
		OrderSide:       "buy",
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
		Invoice:         invoice.PaymentRequest,
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

	err = boltz.CheckSwapAddress(server.chainParams, swap.Address, swap.RedeemScript)

	if err != nil {
		return nil, err
	}

	logger.Info("Verified redeem script and address of Swap " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, err
	}

	server.nursery.RegisterSwap(swap)

	logger.Info("Created new Swap " + swap.Id + ": " + marshalJson(swap.Serialize()))

	return &boltzrpc.CreateSwapResponse{
		Id:             swap.Id,
		Address:        response.Address,
		ExpectedAmount: int64(response.ExpectedAmount),
		Bip21:          response.Bip21,
	}, nil
}

func getKeys() (*btcec.PrivateKey, *btcec.PublicKey, error) {
	privateKey, err := btcec.NewPrivateKey(btcec.S256())

	if err != nil {
		return nil, nil, err
	}

	publicKey := privateKey.PubKey()

	return privateKey, publicKey, err
}

func marshalJson(data interface{}) string {
	marshalled, _ := json.MarshalIndent(data, "", "  ")
	return string(marshalled)
}
