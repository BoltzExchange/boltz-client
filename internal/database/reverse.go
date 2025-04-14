package database

import (
	"crypto/sha256"
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

type ReverseSwap struct {
	Id                  string
	Pair                boltz.Pair
	ChanIds             []lightning.ChanId
	State               boltzrpc.SwapState
	Error               string
	CreatedAt           time.Time
	PaidAt              time.Time
	Status              boltz.SwapUpdateEvent
	AcceptZeroConf      bool
	PrivateKey          *btcec.PrivateKey
	RefundPubKey        *btcec.PublicKey
	SwapTree            *boltz.SwapTree
	Preimage            []byte
	RedeemScript        []byte
	Invoice             string
	ClaimAddress        string
	OnchainAmount       uint64
	InvoiceAmount       uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	ClaimTransactionId  string
	BlindingKey         *btcec.PrivateKey
	IsAuto              bool
	RoutingFeeMsat      *uint64
	ServiceFee          *int64
	ServiceFeePercent   boltz.Percentage
	OnchainFee          *uint64
	ExternalPay         bool
	WalletId            *Id
	TenantId            Id
}

type ReverseSwapSerialized struct {
	Id                  string
	From                string
	To                  string
	ChanIds             string
	State               string
	Error               string
	Status              string
	AcceptZeroConf      bool
	PrivateKey          string
	SwapTree            string
	Preimage            string
	RedeemScript        string
	Invoice             string
	ClaimAddress        string
	OnchainAmount       uint64
	InvoiceAmount       uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	ClaimTransactionId  string
	BlindingKey         string
	IsAuto              bool
	RoutingFeeMsat      *uint64
	ServiceFee          *int64
	ServiceFeePercent   boltz.Percentage
	OnchainFee          *uint64
	ExternalPay         bool
	WalletId            *Id
	TenantId            Id
}

func (reverseSwap *ReverseSwap) Serialize() ReverseSwapSerialized {
	return ReverseSwapSerialized{
		Id:                  reverseSwap.Id,
		From:                string(reverseSwap.Pair.From),
		To:                  string(reverseSwap.Pair.To),
		ChanIds:             formatJson(reverseSwap.ChanIds),
		State:               boltzrpc.SwapState_name[int32(reverseSwap.State)],
		Error:               reverseSwap.Error,
		Status:              reverseSwap.Status.String(),
		AcceptZeroConf:      reverseSwap.AcceptZeroConf,
		PrivateKey:          formatPrivateKey(reverseSwap.PrivateKey),
		Preimage:            hex.EncodeToString(reverseSwap.Preimage),
		RedeemScript:        hex.EncodeToString(reverseSwap.RedeemScript),
		Invoice:             reverseSwap.Invoice,
		ClaimAddress:        reverseSwap.ClaimAddress,
		OnchainAmount:       reverseSwap.OnchainAmount,
		InvoiceAmount:       reverseSwap.InvoiceAmount,
		TimeoutBlockHeight:  reverseSwap.TimeoutBlockHeight,
		LockupTransactionId: reverseSwap.LockupTransactionId,
		ClaimTransactionId:  reverseSwap.ClaimTransactionId,
		BlindingKey:         formatPrivateKey(reverseSwap.BlindingKey),
		IsAuto:              reverseSwap.IsAuto,
		RoutingFeeMsat:      reverseSwap.RoutingFeeMsat,
		ServiceFee:          reverseSwap.ServiceFee,
		ServiceFeePercent:   reverseSwap.ServiceFeePercent,
		OnchainFee:          reverseSwap.OnchainFee,
		ExternalPay:         reverseSwap.ExternalPay,
		WalletId:            reverseSwap.WalletId,
		TenantId:            reverseSwap.TenantId,
	}
}

func (reverseSwap *ReverseSwap) InitTree() error {
	return reverseSwap.SwapTree.Init(
		reverseSwap.Pair.To,
		true,
		reverseSwap.PrivateKey,
		reverseSwap.RefundPubKey,
	)
}

func (reverseSwap *ReverseSwap) PreimageHash() []byte {
	hash := sha256.Sum256(reverseSwap.Preimage)
	return hash[:]
}

func parseReverseSwap(rows *sql.Rows) (*ReverseSwap, error) {
	var reverseSwap ReverseSwap

	var status string
	var privateKey PrivateKeyScanner
	var preimage string
	var redeemScript string
	blindingKey := PrivateKeyScanner{Nullable: true}
	var createdAt, paidAt, serviceFee, onchainFee, routingFeeMsat sql.NullInt64
	var externalPay sql.NullBool
	swapTree := JsonScanner[*boltz.SerializedTree]{Nullable: true}
	refundPubKey := PublicKeyScanner{Nullable: true}
	chanIds := JsonScanner[[]lightning.ChanId]{Nullable: true}

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &reverseSwap.Id,
			"fromCurrency":        &reverseSwap.Pair.From,
			"toCurrency":          &reverseSwap.Pair.To,
			"chanIds":             &chanIds,
			"state":               &reverseSwap.State,
			"error":               &reverseSwap.Error,
			"status":              &status,
			"acceptZeroConf":      &reverseSwap.AcceptZeroConf,
			"privateKey":          &privateKey,
			"refundPubKey":        &refundPubKey,
			"swapTree":            &swapTree,
			"preimage":            &preimage,
			"redeemScript":        &redeemScript,
			"invoice":             &reverseSwap.Invoice,
			"claimAddress":        &reverseSwap.ClaimAddress,
			"onchainAmount":       &reverseSwap.OnchainAmount,
			"invoiceAmount":       &reverseSwap.InvoiceAmount,
			"timeoutBlockheight":  &reverseSwap.TimeoutBlockHeight,
			"lockupTransactionId": &reverseSwap.LockupTransactionId,
			"claimTransactionId":  &reverseSwap.ClaimTransactionId,
			"blindingKey":         &blindingKey,
			"isAuto":              &reverseSwap.IsAuto,
			"routingFeeMsat":      &routingFeeMsat,
			"serviceFee":          &serviceFee,
			"serviceFeePercent":   &reverseSwap.ServiceFeePercent,
			"onchainFee":          &onchainFee,
			"createdAt":           &createdAt,
			"paidAt":              &paidAt,
			"externalPay":         &externalPay,
			"tenantId":            &reverseSwap.TenantId,
			"walletId":            &reverseSwap.WalletId,
		},
	)

	if err != nil {
		return nil, err
	}

	reverseSwap.ServiceFee = parseNullInt(serviceFee)
	reverseSwap.OnchainFee = parseNullUint(onchainFee)
	reverseSwap.RoutingFeeMsat = parseNullUint(routingFeeMsat)
	reverseSwap.Status = boltz.ParseEvent(status)
	reverseSwap.ChanIds = chanIds.Value

	reverseSwap.PrivateKey = privateKey.Value
	reverseSwap.BlindingKey = blindingKey.Value
	reverseSwap.RefundPubKey = refundPubKey.Value

	reverseSwap.Preimage, err = hex.DecodeString(preimage)
	if err != nil {
		return nil, err
	}

	reverseSwap.RedeemScript, err = hex.DecodeString(redeemScript)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	reverseSwap.CreatedAt = parseTime(createdAt.Int64)
	reverseSwap.PaidAt = parseTime(paidAt.Int64)

	if swapTree.Value != nil {
		reverseSwap.SwapTree = swapTree.Value.Deserialize()
		if err := reverseSwap.InitTree(); err != nil {
			return nil, fmt.Errorf("could not initialize swap tree: %w", err)
		}
	}
	reverseSwap.ExternalPay = externalPay.Bool

	return &reverseSwap, err
}

func (database *Database) QueryReverseSwap(id string) (reverseSwap *ReverseSwap, err error) {
	database.lock.Lock()
	defer database.lock.Unlock()
	// TODO: avoid "SELECT *" to be compatible with migrations (or work with columns in parse functions?)
	rows, err := database.Query("SELECT * FROM reverseSwaps WHERE id = ?", id)

	if err != nil {
		return reverseSwap, err
	}

	defer rows.Close()

	if rows.Next() {
		reverseSwap, err = parseReverseSwap(rows)

		if err != nil {
			return reverseSwap, err
		}
	} else {
		return reverseSwap, errors.New("could not find Reverse Swap " + id)
	}

	return reverseSwap, err
}

func (database *Database) QueryReverseSwapByClaimTransaction(txId string) (reverseSwap *ReverseSwap, err error) {
	database.lock.Lock()
	defer database.lock.Unlock()
	rows, err := database.Query("SELECT * FROM reverseSwaps WHERE claimTransactionId = ?", txId)

	if err != nil {
		return reverseSwap, err
	}
	defer rows.Close()

	if rows.Next() {
		reverseSwap, err = parseReverseSwap(rows)

		if err != nil {
			return reverseSwap, err
		}
	} else {
		return reverseSwap, sql.ErrNoRows
	}

	return reverseSwap, err
}

const claimableSwapsQuery = `
SELECT * FROM reverseSwaps
WHERE fromCurrency = ?
  AND reverseSwaps.lockupTransactionId != ''
  AND reverseSwaps.claimTransactionId == ''
`

func (database *Database) QueryClaimableReverseSwaps(tenantId *Id, currency boltz.Currency) ([]*ReverseSwap, error) {
	query := claimableSwapsQuery
	values := []any{currency}
	if tenantId != nil {
		query += " AND tenantId = ?"
		values = append(values, *tenantId)
	}
	return database.queryReverseSwaps(query, values...)
}

func (database *Database) queryReverseSwaps(query string, values ...any) (swaps []*ReverseSwap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, values...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		swap, err := parseReverseSwap(rows)

		if err != nil {
			return nil, err
		}

		swaps = append(swaps, swap)
	}

	return swaps, err
}

func (database *Database) QueryReverseSwaps(args SwapQuery) ([]*ReverseSwap, error) {
	where, values := args.ToWhereClause()
	return database.queryReverseSwaps("SELECT * FROM reverseSwaps"+where, values...)
}

func (database *Database) QueryPendingReverseSwaps() ([]*ReverseSwap, error) {
	return database.QueryReverseSwaps(PendingSwapQuery)
}

const insertReverseSwapStatement = `
INSERT INTO reverseSwaps (id, fromCurrency, toCurrency, chanIds, state, error, status, acceptZeroConf, privateKey, preimage, redeemScript,
                          invoice, claimAddress, onchainAmount, invoiceAmount, timeoutBlockheight, lockupTransactionId,
                          claimTransactionId, blindingKey, isAuto, createdAt, routingFeeMsat, serviceFee,
                          serviceFeePercent, onchainFee, refundPubKey, swapTree, externalPay, tenantId, walletId)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateReverseSwap(reverseSwap ReverseSwap) error {
	_, err := database.Exec(
		insertReverseSwapStatement,
		reverseSwap.Id,
		reverseSwap.Pair.From,
		reverseSwap.Pair.To,
		formatJson(reverseSwap.ChanIds),
		reverseSwap.State,
		reverseSwap.Error,
		reverseSwap.Status.String(),
		reverseSwap.AcceptZeroConf,
		formatPrivateKey(reverseSwap.PrivateKey),
		hex.EncodeToString(reverseSwap.Preimage),
		hex.EncodeToString(reverseSwap.RedeemScript),
		reverseSwap.Invoice,
		reverseSwap.ClaimAddress,
		reverseSwap.OnchainAmount,
		reverseSwap.InvoiceAmount,
		reverseSwap.TimeoutBlockHeight,
		reverseSwap.LockupTransactionId,
		reverseSwap.ClaimTransactionId,
		formatPrivateKey(reverseSwap.BlindingKey),
		reverseSwap.IsAuto,
		FormatTime(reverseSwap.CreatedAt),
		reverseSwap.RoutingFeeMsat,
		reverseSwap.ServiceFee,
		reverseSwap.ServiceFeePercent,
		reverseSwap.OnchainFee,
		formatPublicKey(reverseSwap.RefundPubKey),
		formatJson(reverseSwap.SwapTree.Serialize()),
		reverseSwap.ExternalPay,
		reverseSwap.TenantId,
		reverseSwap.WalletId,
	)
	return err
}

func (database *Database) UpdateReverseSwapState(reverseSwap *ReverseSwap, state boltzrpc.SwapState, error string) error {
	reverseSwap.State = state
	reverseSwap.Error = error

	_, err := database.Exec("UPDATE reverseSwaps SET state = ?, error = ? WHERE id = ?", state, error, reverseSwap.Id)
	return err
}

func (database *Database) UpdateReverseSwapStatus(reverseSwap *ReverseSwap, status boltz.SwapUpdateEvent) error {
	reverseSwap.Status = status

	_, err := database.Exec("UPDATE reverseSwaps SET status = ? WHERE id = ?", status.String(), reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapLockupTransactionId(reverseSwap *ReverseSwap, lockupTransactionId string) error {
	reverseSwap.LockupTransactionId = lockupTransactionId

	_, err := database.Exec("UPDATE reverseSwaps SET lockupTransactionId = ? WHERE id = ?", lockupTransactionId, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapClaimTransactionId(reverseSwap *ReverseSwap, claimTransactionId string, fee uint64) error {
	reverseSwap.ClaimTransactionId = claimTransactionId
	reverseSwap.OnchainFee = &fee

	_, err := database.Exec("UPDATE reverseSwaps SET claimTransactionId = ?, onchainFee = ? WHERE id = ?", claimTransactionId, fee, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapClaimAddress(reverseSwap *ReverseSwap, claimAddress string) error {
	reverseSwap.ClaimAddress = claimAddress

	_, err := database.Exec("UPDATE reverseSwaps SET claimAddress = ? WHERE id = ?", claimAddress, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapWalletId(reverseSwap *ReverseSwap, walletId Id) error {
	reverseSwap.WalletId = &walletId

	_, err := database.Exec("UPDATE reverseSwaps SET walletId = ? WHERE id = ?", walletId, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapRoutingFee(reverseSwap *ReverseSwap, feeMsat uint64) error {
	reverseSwap.RoutingFeeMsat = &feeMsat

	_, err := database.Exec("UPDATE reverseSwaps SET routingFeeMsat = ? WHERE id = ?", feeMsat, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapServiceFee(reverseSwap *ReverseSwap, serviceFee int64, onchainFee uint64) error {
	reverseSwap.ServiceFee = &serviceFee
	reverseSwap.OnchainFee = addToOptional(reverseSwap.OnchainFee, onchainFee)

	_, err := database.Exec("UPDATE reverseSwaps SET serviceFee = ?, onchainFee = ? WHERE id = ?", serviceFee, reverseSwap.OnchainFee, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapPaidAt(reverseSwap *ReverseSwap, paidAt time.Time) error {
	reverseSwap.PaidAt = paidAt

	_, err := database.Exec("UPDATE reverseSwaps SET paidAt = ? WHERE id = ?", FormatTime(reverseSwap.PaidAt), reverseSwap.Id)
	return err
}
