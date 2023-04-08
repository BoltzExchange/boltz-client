package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
)

type ReverseSwap struct {
	Id                  string
	State               boltzrpc.SwapState
	Error               string
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
	ClaimFeePerVbyte    uint32
	ClaimTransactionId  string
}

type ReverseSwapSerialized struct {
	Id                  string
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
	ClaimFeePerVbyte    uint32
	ClaimTransactionId  string
}

func (reverseSwap *ReverseSwap) Serialize() ReverseSwapSerialized {
	return ReverseSwapSerialized{
		Id:                  reverseSwap.Id,
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
		ClaimFeePerVbyte:    reverseSwap.ClaimFeePerVbyte,
		ClaimTransactionId:  reverseSwap.ClaimTransactionId,
	}
}

func parseReverseSwap(rows *sql.Rows) (*ReverseSwap, error) {
	var reverseSwap ReverseSwap

	var status string
	var privateKey string
	var preimage string
	var redeemScript string

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &reverseSwap.Id,
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
			"claimFeePerVbyte":    &reverseSwap.ClaimFeePerVbyte,
			"claimTransactionId":  &reverseSwap.ClaimTransactionId,
		},
	)

	if err != nil {
		return nil, err
	}

	reverseSwap.Status = boltz.ParseEvent(status)
	privateKeyBytes, err := hex.DecodeString(privateKey)

	if err != nil {
		return nil, err
	}

	reverseSwap.PrivateKey, _ = parsePrivateKey(privateKeyBytes)
	reverseSwap.Preimage, err = hex.DecodeString(preimage)

	if err != nil {
		return nil, err
	}

	reverseSwap.RedeemScript, err = hex.DecodeString(redeemScript)

	return &reverseSwap, err
}

func (database *Database) QueryReverseSwap(id string) (reverseSwap *ReverseSwap, err error) {
	// TODO: avoid "SELECT *" to be compatible with migrations (or work with columns in parse functions?)
	rows, err := database.db.Query("SELECT * FROM reverseSwaps WHERE id = ?", id)

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

func (database *Database) queryReverseSwaps(query string) (swaps []ReverseSwap, err error) {
	rows, err := database.db.Query(query)

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

func (database *Database) QueryReverseSwaps() ([]ReverseSwap, error) {
	return database.queryReverseSwaps("SELECT * FROM reverseSwaps")
}

func (database *Database) QueryPendingReverseSwaps() ([]ReverseSwap, error) {
	return database.queryReverseSwaps("SELECT * FROM reverseSwaps WHERE state = '" + strconv.Itoa(int(boltzrpc.SwapState_PENDING)) + "'")
}

func (database *Database) CreateReverseSwap(reverseSwap ReverseSwap) error {
	insertStatement := `
INSERT INTO reverseSwaps (
	id,
	state,
	error,
	status,
	acceptZeroConf,
	privateKey,
	preimage,
	redeemScript,
	invoice,
	claimAddress,
	expectedAmount,
	timeoutBlockheight,
	lockupTransactionId,
	claimFeePerVbyte,
	claimTransactionId
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	statement, err := database.db.Prepare(insertStatement)

	if err != nil {
		return err
	}

	_, err = statement.Exec(
		reverseSwap.Id,
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
		reverseSwap.ClaimFeePerVbyte,
		reverseSwap.ClaimTransactionId,
	)

	if err != nil {
		return err
	}

	return statement.Close()
}

func (database *Database) UpdateReverseSwapState(reverseSwap *ReverseSwap, state boltzrpc.SwapState, error string) error {
	reverseSwap.State = state
	reverseSwap.Error = error

	_, err := database.db.Exec("UPDATE reverseSwaps SET state = ?, error = ? WHERE id = ?", state, error, reverseSwap.Id)
	return err
}

func (database *Database) UpdateReverseSwapStatus(reverseSwap *ReverseSwap, status boltz.SwapUpdateEvent) error {
	reverseSwap.Status = status

	_, err := database.db.Exec("UPDATE reverseSwaps SET status = ? WHERE id = ?", status.String(), reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapLockupTransactionId(reverseSwap *ReverseSwap, lockupTransactionId string) error {
	reverseSwap.LockupTransactionId = lockupTransactionId

	_, err := database.db.Exec("UPDATE reverseSwaps SET lockupTransactionId = ? WHERE id = ?", lockupTransactionId, reverseSwap.Id)
	return err
}

func (database *Database) SetReverseSwapClaimTransactionId(reverseSwap *ReverseSwap, claimTransactionId string, satPerVbyte int64) error {
	reverseSwap.ClaimTransactionId = claimTransactionId
	reverseSwap.ClaimFeePerVbyte = uint32(satPerVbyte)

	_, err := database.db.Exec(
		"UPDATE reverseSwaps SET claimTransactionId = ?, claimFeePerVbyte = ? WHERE id = ?",
		reverseSwap.ClaimTransactionId,
		reverseSwap.ClaimFeePerVbyte,
		reverseSwap.Id,
	)
	return err
}
