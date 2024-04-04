package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
)

type Swap struct {
	Id                  string
	Pair                boltz.Pair
	ChanIds             []lightning.ChanId
	State               boltzrpc.SwapState
	CreatedAt           time.Time
	Error               string
	Status              boltz.SwapUpdateEvent
	PrivateKey          *btcec.PrivateKey
	SwapTree            *boltz.SwapTree
	ClaimPubKey         *btcec.PublicKey
	Preimage            []byte
	RedeemScript        []byte
	Invoice             string
	Address             string
	ExpectedAmount      uint64
	TimoutBlockHeight   uint32
	LockupTransactionId string
	RefundTransactionId string
	RefundAddress       string
	BlindingKey         *btcec.PrivateKey
	IsAuto              bool
	ServiceFee          *uint64
	ServiceFeePercent   utils.Percentage
	OnchainFee          *uint64
	WalletId            *int64
	EntityId            *int64
}

type SwapSerialized struct {
	Id                  string
	From                string
	To                  string
	ChanIds             string
	State               string
	Error               string
	Status              string
	PrivateKey          string
	SwapTree            string
	Preimage            string
	RedeemScript        string
	Invoice             string
	Address             string
	ExpectedAmount      uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	RefundTransactionId string
	RefundAddress       string
	BlindingKey         string
	IsAuto              bool
	ServiceFee          *uint64
	ServiceFeePercent   utils.Percentage
	OnchainFee          *uint64
	WalletId            *int64
	EntityId            *int64
}

func (swap *Swap) BlindingPubKey() *btcec.PublicKey {
	if swap.BlindingKey == nil {
		return nil
	}
	return swap.BlindingKey.PubKey()
}

func (swap *Swap) Serialize() SwapSerialized {
	preimage := ""

	if swap.Preimage != nil {
		preimage = hex.EncodeToString(swap.Preimage)
	}

	return SwapSerialized{
		Id:                  swap.Id,
		From:                string(swap.Pair.From),
		To:                  string(swap.Pair.To),
		ChanIds:             formatJson(swap.ChanIds),
		State:               boltzrpc.SwapState_name[int32(swap.State)],
		Error:               swap.Error,
		Status:              swap.Status.String(),
		PrivateKey:          formatPrivateKey(swap.PrivateKey),
		Preimage:            preimage,
		RedeemScript:        hex.EncodeToString(swap.RedeemScript),
		Invoice:             swap.Invoice,
		Address:             swap.Address,
		ExpectedAmount:      swap.ExpectedAmount,
		TimeoutBlockHeight:  swap.TimoutBlockHeight,
		LockupTransactionId: swap.LockupTransactionId,
		RefundTransactionId: swap.RefundTransactionId,
		RefundAddress:       swap.RefundAddress,
		BlindingKey:         formatPrivateKey(swap.BlindingKey),
		IsAuto:              swap.IsAuto,
		ServiceFee:          swap.ServiceFee,
		ServiceFeePercent:   swap.ServiceFeePercent,
		OnchainFee:          swap.OnchainFee,
		WalletId:            swap.WalletId,
		EntityId:            swap.EntityId,
	}
}

func (swap *Swap) InitTree() error {
	return swap.SwapTree.Init(
		swap.Pair.From == boltz.CurrencyLiquid,
		false,
		swap.PrivateKey,
		swap.ClaimPubKey,
	)
}

func parseSwap(rows *sql.Rows) (*Swap, error) {
	var swap Swap

	var status string
	privateKey := PrivateKeyScanner{}
	var preimage string
	var redeemScript string
	blindingKey := PrivateKeyScanner{Nullable: true}
	var createdAt, serviceFee, onchainFee sql.NullInt64
	swapTree := JsonScanner[*boltz.SerializedTree]{Nullable: true}
	claimPubKey := PublicKeyScanner{Nullable: true}
	chanIds := JsonScanner[[]lightning.ChanId]{Nullable: true}

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &swap.Id,
			"fromCurrency":        &swap.Pair.From,
			"toCurrency":          &swap.Pair.To,
			"chanIds":             &chanIds,
			"state":               &swap.State,
			"error":               &swap.Error,
			"status":              &status,
			"privateKey":          &privateKey,
			"claimPubKey":         &claimPubKey,
			"swapTree":            &swapTree,
			"preimage":            &preimage,
			"redeemScript":        &redeemScript,
			"invoice":             &swap.Invoice,
			"address":             &swap.Address,
			"expectedAmount":      &swap.ExpectedAmount,
			"timeoutBlockheight":  &swap.TimoutBlockHeight,
			"lockupTransactionId": &swap.LockupTransactionId,
			"refundTransactionId": &swap.RefundTransactionId,
			"refundAddress":       &swap.RefundAddress,
			"blindingKey":         &blindingKey,
			"isAuto":              &swap.IsAuto,
			"serviceFee":          &serviceFee,
			"serviceFeePercent":   &swap.ServiceFeePercent,
			"onchainFee":          &onchainFee,
			"createdAt":           &createdAt,
			"walletId":            &swap.WalletId,
			"entityId":            &swap.EntityId,
		},
	)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	swap.ServiceFee = parseNullInt(serviceFee)
	swap.OnchainFee = parseNullInt(onchainFee)
	swap.Status = boltz.ParseEvent(status)
	swap.ChanIds = chanIds.Value
	swap.PrivateKey = privateKey.Value
	swap.BlindingKey = blindingKey.Value
	swap.ClaimPubKey = claimPubKey.Value

	if preimage != "" {
		swap.Preimage, err = hex.DecodeString(preimage)

		if err != nil {
			return nil, err
		}
	}

	swap.RedeemScript, err = hex.DecodeString(redeemScript)

	if err != nil {
		return nil, err
	}

	swap.CreatedAt = parseTime(createdAt.Int64)

	if swapTree.Value != nil {
		swap.SwapTree = swapTree.Value.Deserialize()
		if err := swap.InitTree(); err != nil {
			return nil, fmt.Errorf("could not initialize swap tree: %w", err)
		}
	}

	return &swap, err
}

func (database *Database) QuerySwap(id string) (swap *Swap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM swaps WHERE id = '" + id + "'")

	if err != nil {
		return swap, err
	}

	defer rows.Close()

	if rows.Next() {
		swap, err = parseSwap(rows)

		if err != nil {
			return swap, err
		}
	} else {
		return swap, errors.New("could not find Swap " + id)
	}

	return swap, err
}

func (database *Database) querySwaps(query string, args ...any) (swaps []Swap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		swap, err := parseSwap(rows)

		if err != nil {
			return nil, err
		}

		swaps = append(swaps, *swap)
	}

	return swaps, err
}

func (database *Database) QuerySwaps(args SwapQuery) ([]Swap, error) {
	where, values := args.ToWhereClause()
	return database.querySwaps("SELECT * FROM swaps"+where, values...)
}

func (database *Database) QueryPendingSwaps() ([]Swap, error) {
	state := boltzrpc.SwapState_PENDING
	return database.QuerySwaps(SwapQuery{State: &state})
}

func (database *Database) QueryFailedSwaps(since time.Time) ([]Swap, error) {
	state := boltzrpc.SwapState_ERROR
	return database.QuerySwaps(SwapQuery{State: &state, Since: since})
}

func (database *Database) QueryNonSuccessfullSwaps(currency boltz.Currency) ([]Swap, error) {
	return database.querySwaps(
		"SELECT * FROM swaps WHERE (state = ? OR state = ?) AND fromCurrency = ?",
		boltzrpc.SwapState_SERVER_ERROR, boltzrpc.SwapState_ERROR, currency,
	)
}

func (database *Database) QueryRefundableSwapsForBlockHeight(currentBlockHeight uint32, currency boltz.Currency) ([]Swap, error) {
	return database.querySwaps(
		"SELECT * FROM swaps WHERE (state = ? OR state = ? OR state = ?) AND timeoutBlockheight <= ? AND fromCurrency = ?",
		boltzrpc.SwapState_PENDING, boltzrpc.SwapState_SERVER_ERROR, boltzrpc.SwapState_ERROR, currentBlockHeight, currency,
	)
}

func (database *Database) QueryRefundableSwaps() ([]Swap, error) {
	return database.querySwaps(
		"SELECT * FROM swaps WHERE state != ? AND lockupTransactionId != '' AND refundTransactionId = ''",
		boltzrpc.SwapState_SUCCESSFUL,
	)
}

const insertSwapStatement = `
INSERT INTO swaps (id, fromCurrency, toCurrency, chanIds, state, error, status, privateKey, preimage, redeemScript, invoice, address,
                   expectedAmount, timeoutBlockheight, lockupTransactionId, refundTransactionId, refundAddress,
                   blindingKey, isAuto, createdAt, serviceFee, serviceFeePercent, onchainFee, walletId, claimPubKey, swapTree, entityId)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateSwap(swap Swap) error {
	preimage := ""

	if swap.Preimage != nil {
		preimage = hex.EncodeToString(swap.Preimage)
	}

	_, err := database.Exec(
		insertSwapStatement,
		swap.Id,
		swap.Pair.From,
		swap.Pair.To,
		formatJson(swap.ChanIds),
		swap.State,
		swap.Error,
		swap.Status.String(),
		formatPrivateKey(swap.PrivateKey),
		preimage,
		hex.EncodeToString(swap.RedeemScript),
		swap.Invoice,
		swap.Address,
		swap.ExpectedAmount,
		swap.TimoutBlockHeight,
		swap.LockupTransactionId,
		swap.RefundTransactionId,
		swap.RefundAddress,
		formatPrivateKey(swap.BlindingKey),
		swap.IsAuto,
		FormatTime(swap.CreatedAt),
		swap.ServiceFee,
		swap.ServiceFeePercent,
		swap.OnchainFee,
		swap.WalletId,
		formatPublicKey(swap.ClaimPubKey),
		formatJson(swap.SwapTree.Serialize()),
		swap.EntityId,
	)
	return err
}

func (database *Database) UpdateSwapState(swap *Swap, state boltzrpc.SwapState, error string) error {
	swap.State = state
	swap.Error = error

	_, err := database.Exec("UPDATE swaps SET state = ?, error = ? WHERE id = ?", state, error, swap.Id)
	return err
}

func (database *Database) UpdateSwapStatus(swap *Swap, status boltz.SwapUpdateEvent) error {
	swap.Status = status

	_, err := database.Exec("UPDATE swaps SET status = ? WHERE id = ?", status.String(), swap.Id)
	return err
}

func (database *Database) SetSwapInvoice(swap *Swap, invoice string) error {
	swap.Invoice = invoice

	_, err := database.Exec("UPDATE swaps SET invoice = ? WHERE id = ?", invoice, swap.Id)
	return err
}

func (database *Database) SetSwapLockupTransactionId(swap *Swap, lockupTransactionId string) error {
	swap.LockupTransactionId = lockupTransactionId

	_, err := database.Exec("UPDATE swaps SET lockupTransactionId = ? WHERE id = ?", lockupTransactionId, swap.Id)
	return err
}

func (database *Database) SetSwapExpectedAmount(swap *Swap, expectedAmount uint64) error {
	swap.ExpectedAmount = expectedAmount

	_, err := database.Exec("UPDATE swaps SET expectedAmount = ? WHERE id = ?", expectedAmount, swap.Id)
	return err
}

func (database *Database) SetSwapRefundTransactionId(swap *Swap, refundTransactionId string, fee uint64) error {
	swap.State = boltzrpc.SwapState_REFUNDED
	swap.RefundTransactionId = refundTransactionId
	swap.OnchainFee = addToOptional(swap.OnchainFee, fee)

	_, err := database.Exec("UPDATE swaps SET state = ?, refundTransactionId = ?, onchainFee = ? WHERE id = ?", swap.State, refundTransactionId, swap.OnchainFee, swap.Id)
	return err
}

func (database *Database) SetSwapRefundRefundAddress(swap *Swap, refundAddress string) error {
	swap.RefundAddress = refundAddress

	_, err := database.Exec("UPDATE swaps SET refundAddress = ?  WHERE id = ?", refundAddress, swap.Id)
	return err
}

func (database *Database) SetSwapOnchainFee(swap *Swap, onchainFee uint64) error {
	swap.OnchainFee = &onchainFee

	_, err := database.Exec("UPDATE swaps SET onchainFee = ? WHERE id = ?", swap.OnchainFee, swap.Id)
	return err
}

func (database *Database) SetSwapServiceFee(swap *Swap, serviceFee uint64, onchainFee uint64) error {
	swap.ServiceFee = &serviceFee
	swap.OnchainFee = addToOptional(swap.OnchainFee, onchainFee)

	_, err := database.Exec("UPDATE swaps SET serviceFee = ?, onchainFee = ? WHERE id = ?", serviceFee, swap.OnchainFee, swap.Id)
	return err
}
