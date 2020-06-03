package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/btcsuite/btcd/btcec"
	"strings"
)

type ReverseSwap struct {
	Id                 string
	Status             boltz.SwapUpdateEvent
	PrivateKey         *btcec.PrivateKey
	Preimage           []byte
	RedeemScript       []byte
	Invoice            string
	ClaimAddress       string
	OnchainAmount      int
	TimeoutBlockHeight int
}

type ReverseSwapSerialized struct {
	Id                 string
	Status             string
	PrivateKey         string
	Preimage           string
	RedeemScript       string
	Invoice            string
	ClaimAddress       string
	OnchainAmount      int
	TimeoutBlockHeight int
}

func (reverseSwap *ReverseSwap) Serialize() ReverseSwapSerialized {
	return ReverseSwapSerialized{
		Id:                 reverseSwap.Id,
		Status:             reverseSwap.Status.String(),
		PrivateKey:         formatPrivateKey(reverseSwap.PrivateKey),
		Preimage:           hex.EncodeToString(reverseSwap.Preimage),
		RedeemScript:       hex.EncodeToString(reverseSwap.RedeemScript),
		Invoice:            reverseSwap.Invoice,
		ClaimAddress:       reverseSwap.ClaimAddress,
		OnchainAmount:      reverseSwap.OnchainAmount,
		TimeoutBlockHeight: reverseSwap.TimeoutBlockHeight,
	}
}

func parseReverseSwap(rows *sql.Rows) (*ReverseSwap, error) {
	var reverseSwap ReverseSwap

	var status string
	var privateKey string
	var preimage string
	var redeemScript string

	err := rows.Scan(
		&reverseSwap.Id,
		&status,
		&privateKey,
		&preimage,
		&redeemScript,
		&reverseSwap.Invoice,
		&reverseSwap.ClaimAddress,
		&reverseSwap.OnchainAmount,
		&reverseSwap.TimeoutBlockHeight,
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
	rows, err := database.db.Query("SELECT * FROM reverseSwaps WHERE id = '" + id + "'")

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

func (database *Database) QueryPendingReverseSwaps() ([]ReverseSwap, error) {
	return database.queryReverseSwaps("SELECT * FROM reverseSwaps WHERE status NOT IN ('" + strings.Join(boltz.CompletedStatus, "','") + "')")
}

func (database *Database) CreateReverseSwap(reverseSwap ReverseSwap) error {
	insertStatement := "INSERT INTO reverseSwaps (id, status, privateKey, preimage, redeemScript, invoice, claimAddress, expectedAmount, timeoutBlockheight) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	statement, err := database.db.Prepare(insertStatement)

	if err != nil {
		return err
	}

	_, err = statement.Exec(
		reverseSwap.Id,
		reverseSwap.Status.String(),
		formatPrivateKey(reverseSwap.PrivateKey),
		hex.EncodeToString(reverseSwap.Preimage),
		hex.EncodeToString(reverseSwap.RedeemScript),
		reverseSwap.Invoice,
		reverseSwap.ClaimAddress,
		reverseSwap.OnchainAmount,
		reverseSwap.TimeoutBlockHeight,
	)

	if err != nil {
		return err
	}

	return statement.Close()
}

func (database *Database) UpdateReverseSwapStatus(swap *ReverseSwap, status boltz.SwapUpdateEvent) error {
	swap.Status = status

	_, err := database.db.Exec("UPDATE reverseSwaps SET status = '" + status.String() + "' WHERE id = '" + swap.Id + "'")
	return err
}
