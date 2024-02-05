package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
)

type Swap struct {
	Id                  string
	PairId              boltz.Pair
	ChanIds             []lightning.ChanId
	State               boltzrpc.SwapState
	CreatedAt           time.Time
	Error               string
	Status              boltz.SwapUpdateEvent
	PrivateKey          *btcec.PrivateKey
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
	AutoSend            bool
}

type SwapSerialized struct {
	Id                  string
	PairId              string
	ChanIds             string
	State               string
	Error               string
	Status              string
	PrivateKey          string
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
	AutoSend            bool
}

func (swap *Swap) Serialize() SwapSerialized {
	preimage := ""

	if swap.Preimage != nil {
		preimage = hex.EncodeToString(swap.Preimage)
	}

	return SwapSerialized{
		Id:                  swap.Id,
		PairId:              string(swap.PairId),
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
		AutoSend:            swap.AutoSend,
	}
}

func parseSwap(rows *sql.Rows) (*Swap, error) {
	var swap Swap

	var status string
	var privateKey string
	var preimage string
	var redeemScript string
	var pairId string
	var blindingKey sql.NullString
	var createdAt, serviceFee, onchainFee sql.NullInt64
	chanIds := JsonScanner[[]lightning.ChanId]{Nullable: true}

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &swap.Id,
			"pairId":              &pairId,
			"chanIds":             &chanIds,
			"state":               &swap.State,
			"error":               &swap.Error,
			"status":              &status,
			"privateKey":          &privateKey,
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
			"autoSend":            &swap.AutoSend,
		},
	)

	if err != nil {
		return nil, err
	}

	swap.ServiceFee = parseNullInt(serviceFee)
	swap.OnchainFee = parseNullInt(onchainFee)

	swap.Status = boltz.ParseEvent(status)
	swap.ChanIds = chanIds.Value
	swap.PrivateKey, err = ParsePrivateKey(privateKey)

	if err != nil {
		return nil, err
	}

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

	swap.PairId, err = boltz.ParsePair(pairId)

	if err != nil {
		return nil, err
	}

	if blindingKey.Valid {
		swap.BlindingKey, err = ParsePrivateKey(blindingKey.String)
		if err != nil {
			return nil, err
		}
	}

	if createdAt.Valid {
		swap.CreatedAt = ParseTime(createdAt.Int64)
	}

	return &swap, err
}

func (database *Database) QuerySwap(id string) (swap *Swap, err error) {
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

func (database *Database) QueryRefundableSwaps(currentBlockHeight uint32, pair boltz.Pair) ([]Swap, error) {
	height := strconv.FormatUint(uint64(currentBlockHeight), 10)
	return database.querySwaps("SELECT * FROM swaps WHERE (state = ? OR state = ? OR state = ?) AND timeoutBlockheight <= ? AND pairId = ?", boltzrpc.SwapState_PENDING, boltzrpc.SwapState_SERVER_ERROR, boltzrpc.SwapState_ERROR, height, pair)
}

const insertSwapStatement = `
INSERT INTO swaps (id, pairId, chanIds, state, error, status, privateKey, preimage, redeemScript, invoice, address,
                   expectedAmount, timeoutBlockheight, lockupTransactionId, refundTransactionId, refundAddress,
                   blindingKey, isAuto, createdAt, serviceFee, serviceFeePercent, onchainFee, autoSend)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateSwap(swap Swap) error {
	preimage := ""

	if swap.Preimage != nil {
		preimage = hex.EncodeToString(swap.Preimage)
	}

	_, err := database.Exec(
		insertSwapStatement,
		swap.Id,
		swap.PairId,
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
		swap.AutoSend,
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
