package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/btcsuite/btcd/btcec/v2"
)

type ChainSwap struct {
	Id                string
	Pair              boltz.Pair
	State             boltzrpc.SwapState
	Error             string
	Status            boltz.SwapUpdateEvent
	AcceptZeroConf    bool
	Preimage          []byte
	IsAuto            bool
	ServiceFee        *int64
	ServiceFeePercent boltz.Percentage
	OnchainFee        *uint64
	CreatedAt         time.Time
	TenantId          Id
	FromData          *ChainSwapData
	ToData            *ChainSwapData
}

type ChainSwapData struct {
	Id                  string
	Currency            boltz.Currency
	PrivateKey          *btcec.PrivateKey
	TheirPublicKey      *btcec.PublicKey
	BlindingKey         *btcec.PrivateKey
	Tree                *boltz.SwapTree
	Amount              uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	Transactionid       string
	RefundTransactionId string
	WalletId            *Id
	Address             string
	LockupAddress       string
}

type ChainSwapSerialized struct {
	Id                string
	FromCurrency      string
	ToCurrency        string
	State             int
	Error             string
	Status            string
	AcceptZeroConf    bool
	Preimage          string
	IsAuto            bool
	ServiceFee        *int64
	ServiceFeePercent boltz.Percentage
	OnchainFee        *uint64
	CreatedAt         int64
	TenantId          Id
}

type ChainSwapDataSerialized struct {
	Id                  string
	Currency            string
	PrivateKey          string
	TheirPublicKey      string
	BlindingKey         string
	Tree                string
	Amount              uint64
	TimeoutBlockHeight  uint32
	LockupTransactionId string
	TransactionId       string
	WalletId            *Id
	Address             string
	LockupAddress       string
}

func (swap *ChainSwap) Serialize() ChainSwapSerialized {
	return ChainSwapSerialized{
		Id:                swap.Id,
		FromCurrency:      string(swap.Pair.From),
		ToCurrency:        string(swap.Pair.To),
		State:             int(swap.State),
		Error:             swap.Error,
		Status:            swap.Status.String(),
		AcceptZeroConf:    swap.AcceptZeroConf,
		Preimage:          hex.EncodeToString(swap.Preimage),
		IsAuto:            swap.IsAuto,
		ServiceFee:        swap.ServiceFee,
		ServiceFeePercent: swap.ServiceFeePercent,
		OnchainFee:        swap.OnchainFee,
		CreatedAt:         FormatTime(swap.CreatedAt),
		TenantId:          swap.TenantId,
	}
}

func (swapData *ChainSwapData) Serialize() ChainSwapDataSerialized {
	return ChainSwapDataSerialized{
		Id:                  swapData.Id,
		Currency:            string(swapData.Currency),
		PrivateKey:          formatPrivateKey(swapData.PrivateKey),
		TheirPublicKey:      formatPublicKey(swapData.TheirPublicKey),
		BlindingKey:         formatPrivateKey(swapData.BlindingKey),
		Tree:                formatJson(swapData.Tree.Serialize()),
		Amount:              swapData.Amount,
		TimeoutBlockHeight:  swapData.TimeoutBlockHeight,
		LockupTransactionId: swapData.LockupTransactionId,
		TransactionId:       swapData.Transactionid,
		WalletId:            swapData.WalletId,
		Address:             swapData.Address,
		LockupAddress:       swapData.LockupAddress,
	}
}

const insertChainSwap = `
		INSERT INTO chainSwaps
		(id, fromCurrency, toCurrency, state, error, status, acceptZeroConf, preimage, isAuto, serviceFee, serviceFeePercent, onchainFee, createdAt, tenantId, createdAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) CreateChainSwap(swap ChainSwap) error {
	tx, err := database.BeginTx()
	if err != nil {
		return err
	}
	serialized := swap.Serialize()
	_, err = tx.Exec(
		insertChainSwap,
		serialized.Id,
		serialized.FromCurrency,
		serialized.ToCurrency,
		serialized.State,
		serialized.Error,
		serialized.Status,
		serialized.AcceptZeroConf,
		serialized.Preimage,
		serialized.IsAuto,
		serialized.ServiceFee,
		serialized.ServiceFeePercent,
		serialized.OnchainFee,
		serialized.CreatedAt,
		serialized.TenantId,
		FormatTime(swap.CreatedAt),
	)
	if err != nil {
		return tx.Rollback(err)
	}
	if err := tx.createChainSwapData(*swap.FromData); err != nil {
		return tx.Rollback(err)
	}
	if err := tx.createChainSwapData(*swap.ToData); err != nil {
		return tx.Rollback(err)
	}

	return tx.Commit()
}

const insertChainSwapData = `
		INSERT INTO chainSwapsData
		(id, currency, privateKey, theirPublicKey, blindingKey, tree, amount, timeoutBlockheight, lockupTransactionId, transactionId, walletId, address, lockupAddress)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (database *Database) createChainSwapData(swapData ChainSwapData) error {
	serialized := swapData.Serialize()
	_, err := database.Exec(
		insertChainSwapData,
		serialized.Id,
		serialized.Currency,
		serialized.PrivateKey,
		serialized.TheirPublicKey,
		serialized.BlindingKey,
		serialized.Tree,
		serialized.Amount,
		serialized.TimeoutBlockHeight,
		serialized.LockupTransactionId,
		serialized.TransactionId,
		serialized.WalletId,
		serialized.Address,
		serialized.LockupAddress,
	)

	return err
}

func (database *Database) UpdateChainSwapStatus(chainSwap *ChainSwap, status boltz.SwapUpdateEvent) error {
	chainSwap.Status = status

	_, err := database.Exec("UPDATE chainSwaps SET status = ? WHERE id = ?", status.String(), chainSwap.Id)
	return err
}

func (database *Database) SetChainSwapLockupTransactionId(swapData *ChainSwapData, lockupTransactionId string) error {
	swapData.LockupTransactionId = lockupTransactionId
	_, err := database.Exec("UPDATE chainSwapsData SET lockupTransactionId = ? WHERE id = ? AND currency = ?", lockupTransactionId, swapData.Id, swapData.Currency)
	return err
}

func (database *Database) SetChainSwapAddress(swapData *ChainSwapData, address string) error {
	swapData.Address = address
	_, err := database.Exec("UPDATE chainSwapsData SET address = ? WHERE id = ? AND currency = ?", address, swapData.Id, swapData.Currency)
	return err
}

func (database *Database) SetChainSwapWallet(swapData *ChainSwapData, walletId Id) error {
	swapData.WalletId = &walletId
	_, err := database.Exec("UPDATE chainSwapsData SET walletId = ? WHERE id = ? AND currency = ?", walletId, swapData.Id, swapData.Currency)
	return err
}

func (database *Database) SetChainSwapAmount(swapData *ChainSwapData, amount uint64) error {
	swapData.Amount = amount
	_, err := database.Exec("UPDATE chainSwapsData SET amount = ? WHERE id = ? AND currency = ?", swapData.Amount, swapData.Id, swapData.Currency)
	return err
}

func (database *Database) SetChainSwapTransactionId(swapData *ChainSwapData, transactionId string) error {
	swapData.Transactionid = transactionId
	_, err := database.Exec("UPDATE chainSwapsData SET transactionId = ? WHERE id = ? AND currency = ?", transactionId, swapData.Id, swapData.Currency)
	return err
}

func (database *Database) AddChainSwapOnchainFee(chainSwap *ChainSwap, onchainFee uint64) error {
	chainSwap.OnchainFee = addToOptional(chainSwap.OnchainFee, onchainFee)
	_, err := database.Exec("UPDATE chainSwaps SET onchainFee = ? WHERE id = ?", chainSwap.OnchainFee, chainSwap.Id)
	return err
}

func (database *Database) SetChainSwapServiceFee(chainSwap *ChainSwap, serviceFee int64) error {
	chainSwap.ServiceFee = &serviceFee
	_, err := database.Exec("UPDATE chainSwaps SET serviceFee = ? WHERE id = ?", chainSwap.ServiceFee, chainSwap.Id)
	return err
}

func (database *Database) UpdateChainSwapState(chainSwap *ChainSwap, state boltzrpc.SwapState, error string) error {
	chainSwap.State = state
	chainSwap.Error = error

	_, err := database.Exec("UPDATE chainSwaps SET state = ?, error = ? WHERE id = ?", state, error, chainSwap.Id)
	if err != nil {
		return fmt.Errorf("could not update Chain swap %s state: %w", chainSwap.Id, err)
	}
	return err
}

func (database *Database) QueryChainSwap(id string) (swap *ChainSwap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM chainSwaps WHERE id = ?", id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		swap, err = database.parseChainSwap(rows)

		if err != nil {
			return swap, err
		}
	} else {
		return swap, errors.New("could not find Swap " + id)
	}

	return swap, err
}

func (database *Database) queryChainSwapData(id string, currency boltz.Currency, isClaim bool) (data *ChainSwapData, err error) {
	rows, err := database.Query("SELECT * FROM chainSwapsData WHERE id = ? AND currency = ?", id, currency)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		data, err = parseChainSwapData(rows, isClaim)

		if err != nil {
			return data, err
		}
	} else {
		return data, errors.New("could not find data " + id)
	}

	return data, nil
}

func (database *Database) QueryChainSwaps(args SwapQuery) ([]*ChainSwap, error) {
	where, values := args.ToWhereClause()
	return database.queryChainSwaps("SELECT * FROM chainSwaps"+where, values...)
}

const refundableChainSwapsQuery = `
SELECT swaps.*
FROM chainSwaps swaps
         JOIN chainSwapsData data ON swaps.id = data.id AND data.currency = swaps.fromCurrency AND data.currency = ?
WHERE data.lockupTransactionId != ''
  AND data.transactionId == ''
  AND (status IN (?, ?) OR (state != ? AND data.timeoutBlockheight < ?))
`

func (database *Database) QueryRefundableChainSwaps(tenantId *Id, currency boltz.Currency, currentBlockHeight uint32) ([]*ChainSwap, error) {
	query := refundableChainSwapsQuery
	values := []any{currency, boltz.TransactionLockupFailed.String(), boltz.TransactionFailed.String(), boltzrpc.SwapState_SUCCESSFUL, currentBlockHeight}
	if tenantId != nil {
		query += " AND tenantId = ?"
		values = append(values, tenantId)
	}
	return database.queryChainSwaps(
		query, values...,
	)
}

const claimableChainSwapsQuery = `
SELECT swaps.*
FROM chainSwaps swaps
         JOIN chainSwapsData data ON swaps.id = data.id AND data.currency = swaps.toCurrency AND data.currency = ?
WHERE data.lockupTransactionId != ''
  AND data.transactionId == ''
  AND state != ? 
`

func (database *Database) QueryClaimableChainSwaps(tenantId *Id, currency boltz.Currency) ([]*ChainSwap, error) {
	query := claimableChainSwapsQuery
	values := []any{currency, boltzrpc.SwapState_REFUNDED}
	if tenantId != nil {
		query += " AND tenantId = ?"
		values = append(values, tenantId)
	}
	return database.queryChainSwaps(query, values...)
}

func (database *Database) parseChainSwap(rows *sql.Rows) (*ChainSwap, error) {
	var status string
	var swap ChainSwap
	var preimage string
	var createdAt sql.NullInt64
	var serviceFee, onchainFee sql.NullInt64

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                &swap.Id,
			"fromCurrency":      &swap.Pair.From,
			"toCurrency":        &swap.Pair.To,
			"state":             &swap.State,
			"error":             &swap.Error,
			"status":            &status,
			"acceptZeroConf":    &swap.AcceptZeroConf,
			"preimage":          &preimage,
			"isAuto":            &swap.IsAuto,
			"serviceFee":        &serviceFee,
			"serviceFeePercent": &swap.ServiceFeePercent,
			"onchainFee":        &onchainFee,
			"createdAt":         &createdAt,
			"tenantId":          &swap.TenantId,
		},
	)

	if err != nil {
		return nil, err
	}

	swap.ServiceFee = parseNullInt(serviceFee)
	swap.OnchainFee = parseNullUint(onchainFee)
	swap.CreatedAt = parseTime(createdAt.Int64)
	swap.Status = boltz.ParseEvent(status)

	if preimage != "" {
		swap.Preimage, err = hex.DecodeString(preimage)
		if err != nil {
			return nil, err
		}
	}

	swap.FromData, err = database.queryChainSwapData(swap.Id, swap.Pair.From, false)
	if err != nil {
		return nil, err
	}
	swap.ToData, err = database.queryChainSwapData(swap.Id, swap.Pair.To, true)
	if err != nil {
		return nil, err
	}

	return &swap, nil
}

func (swapData *ChainSwapData) InitTree(isClaim bool) error {
	return swapData.Tree.Init(
		swapData.Currency,
		isClaim,
		swapData.PrivateKey,
		swapData.TheirPublicKey,
	)
}

func (swapData *ChainSwapData) BlindingPubKey() *btcec.PublicKey {
	if swapData.BlindingKey == nil {
		return nil
	}
	return swapData.BlindingKey.PubKey()
}

func (database *Database) queryChainSwaps(query string, args ...any) (swaps []*ChainSwap, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query(query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		swap, err := database.parseChainSwap(rows)

		if err != nil {
			return nil, err
		}

		swaps = append(swaps, swap)
	}

	return swaps, err
}

func parseChainSwapData(rows *sql.Rows, isClaim bool) (*ChainSwapData, error) {
	var swapData ChainSwapData
	privateKey := PrivateKeyScanner{}
	theirPublicKey := PublicKeyScanner{Nullable: true}
	swapTree := JsonScanner[*boltz.SerializedTree]{Nullable: true}
	blindingKey := PrivateKeyScanner{Nullable: true}

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":                  &swapData.Id,
			"currency":            &swapData.Currency,
			"privateKey":          &privateKey,
			"theirPublicKey":      &theirPublicKey,
			"blindingKey":         &blindingKey,
			"tree":                &swapTree,
			"amount":              &swapData.Amount,
			"timeoutBlockheight":  &swapData.TimeoutBlockHeight,
			"lockupTransactionId": &swapData.LockupTransactionId,
			"transactionId":       &swapData.Transactionid,
			"walletId":            &swapData.WalletId,
			"address":             &swapData.Address,
			"lockupAddress":       &swapData.LockupAddress,
		},
	)

	if err != nil {
		return nil, err
	}

	swapData.PrivateKey = privateKey.Value
	swapData.TheirPublicKey = theirPublicKey.Value
	swapData.BlindingKey = blindingKey.Value

	if swapTree.Value != nil {
		swapData.Tree = swapTree.Value.Deserialize()
		if err := swapData.InitTree(isClaim); err != nil {
			return nil, fmt.Errorf("could not initialize swap tree: %w", err)
		}
	}

	return &swapData, nil
}
