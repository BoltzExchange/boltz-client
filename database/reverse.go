package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/utils"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
)

type ReverseSwap struct {
	Id                  string
	PairId              boltz.Pair
	ChanIds             []lightning.ChanId
	State               boltzrpc.SwapState
	Error               string
	CreatedAt           time.Time
	Status              boltz.SwapUpdateEvent
	AcceptZeroConf      bool
	PrivateKey          *btcec.PrivateKey
	Preimage            []byte
	RedeemScript        []byte
	Invoice             string
	ClaimAddress        string
	OnchainAmount       uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	ClaimTransactionId  string
	BlindingKey         *btcec.PrivateKey
	IsAuto              bool
	RoutingFeeMsat      *uint64
	ServiceFee          *uint64
	ServiceFeePercent   utils.Percentage
	OnchainFee          *uint64
}

type ReverseSwapSerialized struct {
	Id                  string
	PairId              string
	ChanIds             string
	State               string
	Error               string
	Status              string
	AcceptZeroConf      bool
	PrivateKey          string
	Preimage            string
	RedeemScript        string
	Invoice             string
	ClaimAddress        string
	OnchainAmount       uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	ClaimTransactionId  string
	BlindingKey         string
	IsAuto              bool
	RoutingFeeMsat      *uint64
	ServiceFee          *uint64
	ServiceFeePercent   utils.Percentage
	OnchainFee          *uint64
}

func (reverseSwap *ReverseSwap) Serialize() ReverseSwapSerialized {
	return ReverseSwapSerialized{
		Id:                  reverseSwap.Id,
		PairId:              string(reverseSwap.PairId),
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
		TimeoutBlockHeight:  reverseSwap.TimeoutBlockHeight,
		LockupTransactionId: reverseSwap.LockupTransactionId,
		ClaimTransactionId:  reverseSwap.ClaimTransactionId,
		BlindingKey:         formatPrivateKey(reverseSwap.BlindingKey),
		IsAuto:              reverseSwap.IsAuto,
		RoutingFeeMsat:      reverseSwap.RoutingFeeMsat,
		ServiceFee:          reverseSwap.ServiceFee,
		ServiceFeePercent:   reverseSwap.ServiceFeePercent,
		OnchainFee:          reverseSwap.OnchainFee,
	}
}

func parseReverseSwap(rows *sql.Rows) (*ReverseSwap, error) {
	var reverseSwap ReverseSwap

	var status string
	var privateKey string
	var preimage string
	var redeemScript string
	var pairId string
	var blindingKey sql.NullString
	var createdAt, serviceFee, onchainFee, routingFeeMsat sql.NullInt64
	chanIds := JsonScanner[[]lightning.ChanId]{Nullable: true}

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &reverseSwap.Id,
			"pairId":              &pairId,
			"chanIds":             &chanIds,
			"state":               &reverseSwap.State,
			"error":               &reverseSwap.Error,
			"status":              &status,
			"acceptZeroConf":      &reverseSwap.AcceptZeroConf,
			"privateKey":          &privateKey,
			"preimage":            &preimage,
			"redeemScript":        &redeemScript,
			"invoice":             &reverseSwap.Invoice,
			"claimAddress":        &reverseSwap.ClaimAddress,
			"expectedAmount":      &reverseSwap.OnchainAmount,
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
		},
	)

	if err != nil {
		return nil, err
	}

	reverseSwap.ServiceFee = parseNullInt(serviceFee)
	reverseSwap.OnchainFee = parseNullInt(onchainFee)
	reverseSwap.RoutingFeeMsat = parseNullInt(routingFeeMsat)

	reverseSwap.Status = boltz.ParseEvent(status)
	reverseSwap.ChanIds = chanIds.Value

	reverseSwap.PrivateKey, err = ParsePrivateKey(privateKey)

	if err != nil {
		return nil, err
	}

	reverseSwap.Preimage, err = hex.DecodeString(preimage)

	if err != nil {
		return nil, err
	}

	reverseSwap.RedeemScript, err = hex.DecodeString(redeemScript)
	if err != nil {
		return nil, err
	}

	reverseSwap.PairId, err = boltz.ParsePair(pairId)

	if err != nil {
		return nil, err
	}

	if blindingKey.Valid {
		reverseSwap.BlindingKey, err = ParsePrivateKey(blindingKey.String)
		if err != nil {
			return nil, err
		}
	}

	if createdAt.Valid {
		reverseSwap.CreatedAt = ParseTime(createdAt.Int64)
	}

	return &reverseSwap, err
}

func (database *Database) QueryReverseSwap(id string) (reverseSwap *ReverseSwap, err error) {
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

func (database *Database) queryReverseSwaps(query string, values ...any) (swaps []ReverseSwap, err error) {
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

		swaps = append(swaps, *swap)
	}

	return swaps, err
}

func (database *Database) QueryReverseSwaps(args SwapQuery) ([]ReverseSwap, error) {
	where, values := args.ToWhereClause()
	return database.queryReverseSwaps("SELECT * FROM reverseSwaps"+where, values...)
}

func (database *Database) QueryPendingReverseSwaps() ([]ReverseSwap, error) {
	state := boltzrpc.SwapState_PENDING
	return database.QueryReverseSwaps(SwapQuery{State: &state})
}

func (database *Database) QueryFailedReverseSwaps(since time.Time) ([]ReverseSwap, error) {
	state := boltzrpc.SwapState_ERROR
	return database.QueryReverseSwaps(SwapQuery{State: &state, Since: since})
}

const insertReverseSwapStatement = `
INSERT INTO reverseSwaps (id, pairId, chanIds, state, error, status, acceptZeroConf, privateKey, preimage, redeemScript,
                          invoice, claimAddress, expectedAmount, timeoutBlockheight, lockupTransactionId,
                          claimTransactionId, blindingKey, isAuto, createdAt, routingFeeMsat, serviceFee,
                          serviceFeePercent, onchainFee)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateReverseSwap(reverseSwap ReverseSwap) error {
	_, err := database.Exec(
		insertReverseSwapStatement,
		reverseSwap.Id,
		reverseSwap.PairId,
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

func (database *Database) SetReverseSwapRoutingFee(reverseSwap *ReverseSwap, feeMsat uint64) error {
	reverseSwap.RoutingFeeMsat = &feeMsat

	_, err := database.Exec("UPDATE reverseSwaps SET routingFeeMsat = ? WHERE id = ?", feeMsat, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapServiceFee(reverseSwap *ReverseSwap, serviceFee uint64, onchainFee uint64) error {
	reverseSwap.ServiceFee = &serviceFee
	reverseSwap.OnchainFee = addToOptional(reverseSwap.OnchainFee, onchainFee)

	_, err := database.Exec("UPDATE reverseSwaps SET serviceFee = ?, onchainFee = ? WHERE id = ?", serviceFee, reverseSwap.OnchainFee, reverseSwap.Id)
	return err
}
