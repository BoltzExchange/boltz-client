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
	"github.com/golang/protobuf/ptypes/empty"
	"math"
	"strconv"

	"github.com/BoltzExchange/boltz-client/onchain/liquid"

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

type routedBoltzServer struct {
	boltzrpc.BoltzServer

	network *boltz.Network

	onchain   *onchain.Onchain
	lightning lightning.LightningNode
	boltz     *boltz.Boltz
	nursery   *nursery.Nursery
	database  *database.Database
	swapper   *autoswap.AutoSwapper

	stop chan bool
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

	blockHeights[string(boltz.PairBtc)], err = server.onchain.GetBlockHeight(boltz.PairBtc)
	if err != nil {
		return nil, handleError(err)
	}
	blockHeights[string(boltz.PairLiquid)], err = server.onchain.GetBlockHeight(boltz.PairLiquid)
	if err != nil {
		return nil, handleError(err)
	}

	return &boltzrpc.GetInfoResponse{
		// TODO: provide info for liquid aswell
		Symbol:              "BTC",
		Network:             server.network.Name,
		NodePubkey:          lightningInfo.Pubkey,
		BlockHeights:        blockHeights,
		PendingSwaps:        pendingSwapIds,
		PendingReverseSwaps: pendingReverseSwapIds,

		LndPubkey:   lightningInfo.Pubkey,
		BlockHeight: lightningInfo.BlockHeight,
	}, nil
}

func (server *routedBoltzServer) GetServiceInfo(_ context.Context, request *boltzrpc.GetServiceInfoRequest) (*boltzrpc.GetServiceInfoResponse, error) {
	pair, err := boltz.ParsePair(request.PairId)
	if err != nil {
		return nil, handleError(err)
	}
	fees, limits, err := server.getPairs(pair)

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
		response.Swaps = append(response.Swaps, serializeSwap(&swap))
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
	response, err := server.createSwap(false, &boltzrpc.CreateSwapRequest{PairId: request.PairId})
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

	pair, err := boltz.ParsePair(request.PairId)
	if err != nil {
		return nil, handleError(err)
	}

	createSwap := boltz.CreateSwapRequest{
		Type:            boltz.NormalSwap,
		PairId:          string(pair),
		OrderSide:       "sell",
		RefundPublicKey: hex.EncodeToString(publicKey.SerializeCompressed()),
	}

	var preimage, preimageHash []byte
	if request.Amount != 0 {
		invoice, err := server.lightning.CreateInvoice(request.Amount, nil, 0, utils.GetSwapMemo(request.PairId))
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

	wallet, err := server.onchain.GetWallet(pair)
	if err != nil {
		if request.AutoSend {
			return nil, handleError(err)
		}
		if request.RefundAddress == "" {
			return nil, handleError(fmt.Errorf("refund address is required if wallet is not available: %w", err))
		}
	}

	fees, _, err := server.getPairs(pair)
	if err != nil {
		return nil, handleError(err)
	}

	response, err := server.boltz.CreateSwap(createSwap)

	if err != nil {
		return nil, handleError(errors.New("boltz error: " + err.Error()))
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)

	if err != nil {
		return nil, handleError(err)
	}

	swap := database.Swap{
		Id:                  response.Id,
		PairId:              pair,
		State:               boltzrpc.SwapState_PENDING,
		Error:               "",
		PrivateKey:          privateKey,
		Preimage:            preimage,
		RedeemScript:        redeemScript,
		Invoice:             createSwap.Invoice,
		Address:             response.Address,
		ExpectedAmount:      response.ExpectedAmount,
		TimoutBlockHeight:   response.TimeoutBlockHeight,
		LockupTransactionId: "",
		RefundTransactionId: "",
		RefundAddress:       request.RefundAddress,
		IsAuto:              isAuto,
		ServiceFeePercent:   utils.Percentage(fees.Percentage),
		AutoSend:            request.AutoSend,
	}

	if request.ChanId != nil {
		swap.ChanId, err = lightning.NewChanIdFromString(*request.ChanId)
		if err != nil {
			return nil, handleError(errors.New("invalid channel id: " + err.Error()))
		}
	}

	var blindingPubKey *btcec.PublicKey
	if pair == boltz.PairLiquid {
		swap.BlindingKey, err = database.ParsePrivateKey(response.BlindingKey)
		blindingPubKey = swap.BlindingKey.PubKey()

		if err != nil {
			return nil, handleError(err)
		}
	}

	err = boltz.CheckSwapScript(swap.RedeemScript, preimageHash, swap.PrivateKey, swap.TimoutBlockHeight)

	if err != nil {
		return nil, handleError(err)
	}

	err = boltz.CheckSwapAddress(pair, server.network, swap.Address, swap.RedeemScript, true, blindingPubKey)

	if err != nil {
		return nil, handleError(err)
	}

	logger.Info("Verified redeem script and address of Swap " + swap.Id)

	err = server.database.CreateSwap(swap)

	if err != nil {
		return nil, handleError(err)
	}

	swapResponse := &boltzrpc.CreateSwapResponse{
		Id:                 swap.Id,
		Address:            response.Address,
		ExpectedAmount:     int64(response.ExpectedAmount),
		Bip21:              response.Bip21,
		TimeoutBlockHeight: response.TimeoutBlockHeight,
	}

	if request.AutoSend {
		// TODO: custom block target?
		feeSatPerVbyte, err := server.onchain.EstimateFee(pair, 2)
		if err != nil {
			return nil, handleError(err)
		}
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

	pair, err := boltz.ParsePair(request.PairId)
	if err != nil {
		return nil, handleError(err)
	}
	if claimAddress != "" {
		err := boltz.ValidateAddress(server.network, claimAddress, pair)

		if err != nil {
			return nil, handleError(fmt.Errorf("Invalid claim address %s: %w", claimAddress, err))
		}
	} else {
		wallet, err := server.onchain.GetWallet(pair)
		if err != nil {
			return nil, handleError(err)
		}

		claimAddress, err = wallet.NewAddress()
		if err != nil {
			return nil, handleError(err)
		}

		logger.Info("Got claim address from wallet: " + claimAddress)
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
		Type:           boltz.ReverseSwap,
		PairId:         string(pair),
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

	fees, _, err := server.getPairs(pair)
	if err != nil {
		return nil, handleError(err)
	}

	reverseSwap := database.ReverseSwap{
		Id:                  response.Id,
		IsAuto:              isAuto,
		PairId:              pair,
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
		ServiceFeePercent:   utils.Percentage(fees.Percentage),
	}

	if request.ChanId != nil {
		reverseSwap.ChanId, err = lightning.NewChanIdFromString(*request.ChanId)
		if err != nil {
			return nil, handleError(errors.New("invalid channel id: " + err.Error()))
		}
	}

	if pair == boltz.PairLiquid {
		reverseSwap.BlindingKey, err = database.ParsePrivateKey(response.BlindingKey)

		if err != nil {
			return nil, handleError(err)
		}
	}
	err = boltz.CheckReverseSwapScript(reverseSwap.RedeemScript, preimageHash, privateKey, response.TimeoutBlockHeight)

	if err != nil {
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
	logger.Info("Restarting autoswapper because liquid wallet has changed.")
	if err := server.swapper.Start(); err != nil {
		logger.Errorf("Failed to restart swapper after liquid wallet has changed: %v", err)
	}
}

func (server *routedBoltzServer) ImportLiquidWallet(context context.Context, request *boltzrpc.ImportLiquidWalletRequest) (*boltzrpc.ImportLiquidWalletResponse, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err == nil {
		return nil, handleError(errors.New("there is an existing wallet, which has to be removed first"))
	}

	if err := wallet.ImportMnemonic(request.Mnemonic); err != nil {
		return nil, handleError(errors.New("could not login: " + err.Error()))
	}

	server.onWalletChange()
	return &boltzrpc.ImportLiquidWalletResponse{}, nil
}

func (server *routedBoltzServer) SetLiquidSubaccount(context context.Context, request *boltzrpc.SetLiquidSubaccountRequest) (*boltzrpc.LiquidWalletInfo, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err != nil {
		return nil, handleError(err)
	}

	if err := wallet.SetSubaccount(request.Subaccount); err != nil {
		return nil, handleError(err)
	}

	server.onWalletChange()

	return server.GetLiquidWalletInfo(context, &boltzrpc.GetLiquidWalletInfoRequest{})
}

func (server *routedBoltzServer) GetLiquidSubaccounts(context.Context, *boltzrpc.GetLiquidSubaccountsRequest) (*boltzrpc.GetLiquidSubaccountsResponse, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err != nil {
		return nil, handleError(err)
	}

	subaccounts, err := wallet.GetSubaccounts(true)
	if err != nil {
		return nil, handleError(err)
	}

	var grpcSubaccounts []*boltzrpc.LiquidSubaccount
	for _, subaccount := range subaccounts {
		balance, err := wallet.GetSubaccountBalance(subaccount.Pointer)
		if err != nil {
			logger.Errorf("failed to get balance for subaccount %+v: %v", subaccount, err.Error())
		}
		grpcSubaccounts = append(grpcSubaccounts, serializeLiquidSubaccount(*subaccount, balance))
	}
	return &boltzrpc.GetLiquidSubaccountsResponse{Subaccounts: grpcSubaccounts}, nil
}

func (server *routedBoltzServer) CreateLiquidWallet(_ context.Context, request *boltzrpc.CreateLiquidWalletRequest) (*boltzrpc.LiquidWalletMnemonic, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err == nil {
		return nil, handleError(errors.New("there is an existing wallet, which has to be removed first"))
	}

	mnemonic, err := wallet.Register()
	if err != nil {
		return nil, handleError(errors.New("could not create wallet: " + err.Error()))
	}

	server.onWalletChange()

	return &boltzrpc.LiquidWalletMnemonic{Mnemonic: mnemonic}, nil
}

func (server *routedBoltzServer) GetLiquidWalletInfo(_ context.Context, request *boltzrpc.GetLiquidWalletInfoRequest) (*boltzrpc.LiquidWalletInfo, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err != nil {
		return nil, handleError(err)
	}
	current, err := wallet.CurrentSubaccount()
	if err != nil {
		return nil, handleError(err)
	}
	subaccount, err := wallet.GetSubaccount(current)
	if err != nil {
		return nil, handleError(err)
	}
	balance, err := wallet.GetBalance()
	if err != nil {
		return nil, handleError(err)
	}
	return &boltzrpc.LiquidWalletInfo{
		Subaccount: serializeLiquidSubaccount(*subaccount, balance),
	}, nil
}

func (server *routedBoltzServer) GetLiquidWalletMnemonic(_ context.Context, request *boltzrpc.GetLiquidWalletMnemonicRequest) (*boltzrpc.LiquidWalletMnemonic, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err != nil {
		return nil, handleError(err)
	}
	mnemonic, err := wallet.GetMnemonic()
	if err != nil {
		return nil, handleError(errors.New("could not read liquid wallet mnemonic: " + err.Error()))
	}

	return &boltzrpc.LiquidWalletMnemonic{
		Mnemonic: mnemonic,
	}, nil
}

func (server *routedBoltzServer) RemoveLiquidWallet(context.Context, *boltzrpc.RemoveLiquidWalletRequest) (*boltzrpc.RemoveLiquidWalletResponse, error) {
	wallet, err := server.getExistingLiquidWallet()
	if err != nil {
		return nil, handleError(err)
	}
	if err := wallet.Remove(); err != nil {
		return nil, errors.New("could not delete wallet: " + err.Error())
	}
	server.onWalletChange()
	return &boltzrpc.RemoveLiquidWalletResponse{}, nil
}

func (server *routedBoltzServer) Stop(context.Context, *empty.Empty) (*empty.Empty, error) {
	server.nursery.Stop()
	server.stop <- true
	return &empty.Empty{}, nil
}

func (server *routedBoltzServer) getExistingLiquidWallet() (*liquid.Wallet, error) {
	wallet := server.onchain.Liquid.Wallet.(*liquid.Wallet)
	if !wallet.Exists() {
		return wallet, errors.New("got no liquid wallet")
	}
	return wallet, nil
}

func (server *routedBoltzServer) getPairs(pairId boltz.Pair) (*boltzrpc.Fees, *boltzrpc.Limits, error) {
	pairsResponse, err := server.boltz.GetPairs()

	if err != nil {
		return nil, nil, err
	}

	pair, hasPair := pairsResponse.Pairs[string(pairId)]

	if !hasPair {
		return nil, nil, errors.New("could not find pair with id: " + string(pairId))
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
