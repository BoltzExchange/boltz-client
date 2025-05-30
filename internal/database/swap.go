package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
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
	PaymentHash         []byte
	Address             string
	ExpectedAmount      uint64
	TimoutBlockHeight   uint32
	LockupTransactionId string
	RefundTransactionId string
	RefundAddress       string
	BlindingKey         *btcec.PrivateKey
	IsAuto              bool
	ServiceFee          *int64
	ServiceFeePercent   boltz.Percentage
	OnchainFee          *uint64
	WalletId            *Id
	TenantId            Id
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
	PaymentHash         string
	Address             string
	ExpectedAmount      uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	RefundTransactionId string
	RefundAddress       string
	BlindingKey         string
	IsAuto              bool
	ServiceFee          *int64
	ServiceFeePercent   boltz.Percentage
	OnchainFee          *uint64
	WalletId            *Id
	TenantId            Id
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
		PaymentHash:         hex.EncodeToString(swap.PaymentHash),
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
		TenantId:            swap.TenantId,
	}
}

func (swap *Swap) InitTree() error {
	return swap.SwapTree.Init(
		swap.Pair.From,
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
	var paymentHash string
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
			"paymentHash":         &paymentHash,
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
			"tenantId":            &swap.TenantId,
		},
	)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	swap.ServiceFee = parseNullInt(serviceFee)
	swap.OnchainFee = parseNullUint(onchainFee)
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

	if paymentHash != "" {
		swap.PaymentHash, err = hex.DecodeString(paymentHash)

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

	defer closeRows(rows)

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

func (database *Database) QuerySwapByInvoice(invoice string) (swap *Swap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM swaps WHERE invoice = ?", invoice)

	if err != nil {
		return swap, err
	}

	defer closeRows(rows)

	if rows.Next() {
		swap, err = parseSwap(rows)

		if err != nil {
			return swap, err
		}
	}
	return swap, err
}

func (database *Database) QuerySwapByPaymentHash(paymentHash []byte) (swap *Swap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM swaps WHERE paymentHash = ?", hex.EncodeToString(paymentHash))

	if err != nil {
		return swap, err
	}

	defer closeRows(rows)

	if rows.Next() {
		swap, err = parseSwap(rows)

		if err != nil {
			return swap, err
		}
	}
	return swap, err
}

func (database *Database) querySwaps(query string, args ...any) (swaps []*Swap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, args...)

	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	for rows.Next() {
		swap, err := parseSwap(rows)

		if err != nil {
			return nil, err
		}

		swaps = append(swaps, swap)
	}

	return swaps, err
}

func (database *Database) QuerySwaps(args SwapQuery) ([]*Swap, error) {
	where, values := args.ToWhereClause()
	return database.querySwaps("SELECT * FROM swaps"+where, values...)
}

func (database *Database) QueryPendingSwaps() ([]*Swap, error) {
	return database.QuerySwaps(PendingSwapQuery)
}

const refundableSwapsQuery = `
SELECT * FROM swaps
WHERE fromCurrency = ?
  AND swaps.lockupTransactionId != ''
  AND swaps.refundTransactionId == ''
  AND (state IN (?, ?) OR (state != ? AND swaps.timeoutBlockheight < ?))
`

func (database *Database) QueryRefundableSwaps(tenantId *Id, currency boltz.Currency, currentBlockHeight uint32) ([]*Swap, error) {
	query := refundableSwapsQuery
	values := []any{currency, boltzrpc.SwapState_SERVER_ERROR, boltzrpc.SwapState_ERROR, boltzrpc.SwapState_SUCCESSFUL, currentBlockHeight}
	if tenantId != nil {
		query += " AND tenantId = ?"
		values = append(values, tenantId)
	}
	return database.querySwaps(query, values...)
}

const insertSwapStatement = `
INSERT INTO swaps (id, fromCurrency, toCurrency, chanIds, state, error, status, privateKey, preimage, redeemScript, invoice, paymentHash, address,
                   expectedAmount, timeoutBlockheight, lockupTransactionId, refundTransactionId, refundAddress,
                   blindingKey, isAuto, createdAt, serviceFee, serviceFeePercent, onchainFee, walletId, claimPubKey, swapTree, tenantId)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		hex.EncodeToString(swap.PaymentHash[:]),
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
		swap.TenantId,
	)
	return err
}

func (database *Database) UpdateSwapState(swap *Swap, state boltzrpc.SwapState, error string) error {
	swap.State = state
	swap.Error = error

	_, err := database.Exec("UPDATE swaps SET state = ?, error = ? WHERE id = ?", state, error, swap.Id)
	if err != nil {
		return fmt.Errorf("could not update state of Swap %s: %w", swap.Id, err)
	}
	return nil
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

func (database *Database) SetSwapPreimage(swap *Swap, preimage []byte) error {
	swap.Preimage = preimage

	_, err := database.Exec("UPDATE swaps SET preimage = ? WHERE id = ?", hex.EncodeToString(preimage), swap.Id)
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

func (database *Database) SetSwapRefundAddress(swap *Swap, refundAddress string) error {
	swap.RefundAddress = refundAddress

	_, err := database.Exec("UPDATE swaps SET refundAddress = ? WHERE id = ?", refundAddress, swap.Id)
	return err
}

func (database *Database) SetSwapRefundWallet(swap *Swap, walletId Id) error {
	swap.WalletId = &walletId

	_, err := database.Exec("UPDATE swaps SET walletId = ? WHERE id = ?", walletId, swap.Id)
	return err
}

func (database *Database) SetSwapOnchainFee(swap *Swap, onchainFee uint64) error {
	swap.OnchainFee = &onchainFee

	_, err := database.Exec("UPDATE swaps SET onchainFee = ? WHERE id = ?", swap.OnchainFee, swap.Id)
	return err
}

func (database *Database) SetSwapServiceFee(swap *Swap, serviceFee int64, onchainFee uint64) error {
	swap.ServiceFee = &serviceFee
	swap.OnchainFee = addToOptional(swap.OnchainFee, onchainFee)

	_, err := database.Exec("UPDATE swaps SET serviceFee = ?, onchainFee = ? WHERE id = ?", serviceFee, swap.OnchainFee, swap.Id)
	return err
}
