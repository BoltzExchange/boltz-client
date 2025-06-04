package database

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"golang.org/x/exp/constraints"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/btcsuite/btcd/btcec/v2"
	_ "github.com/mattn/go-sqlite3"
)

// TODO: prepare insert statements only once

const createViews =
// language=sql
`
DROP VIEW IF EXISTS allSwaps;
CREATE VIEW allSwaps AS
SELECT chainSwaps.id     as id,
       fromCurrency,
       toCurrency,
       'chain'     AS type,
       fromData.amount as amount,
       state,
       error,
       status,
       createdAt,
       onchainFee,
       serviceFee,
       isAuto,
       toData.transactionId AS claimTransactionId,
       fromData.lockupTransactionId AS lockupTransactionId,
       fromData.transactionId AS refundTransactionId,
       tenantId
FROM chainSwaps
         JOIN chainSwapsData fromData ON chainSwaps.id = fromData.id AND chainSwaps.fromCurrency = fromData.currency
         JOIN chainSwapsData toData ON chainSwaps.id = toData.id AND chainSwaps.toCurrency = toData.currency
UNION ALL
SELECT id,
       fromCurrency,
       toCurrency,
       'reverse'     AS type,
       invoiceAmount as amount,
       state,
       error,
       status,
       createdAt,
       coalesce(onchainFee, 0) + (coalesce(routingFeeMsat, 0) / 1000)
                     as onchainFee,
       serviceFee,
       isAuto,
       claimTransactionId,
       lockupTransactionId,
       '' AS refundTransactionId,
       tenantId
FROM reverseSwaps
UNION ALL
SELECT id,
       fromCurrency,
       toCurrency,
       'submarine'    AS type,
       expectedAmount as amount,
       state,
       error,
       status,
       createdAt,
       onchainFee,
       serviceFee,
       isAuto,
       '' AS claimTransactionId,
       lockupTransactionId,
       refundTransactionId,
       tenantId
FROM swaps;
`

const createTables = `
CREATE TABLE version
(
    version INT
);
CREATE TABLE macaroons
(
    id      VARCHAR PRIMARY KEY,
    rootKey VARCHAR
);
CREATE TABLE swaps
(
    id                  VARCHAR PRIMARY KEY,
    fromCurrency        VARCHAR,
    toCurrency          VARCHAR,
    chanIds             JSON,
    state               INT,
    error               VARCHAR,
    status              VARCHAR,
    privateKey          VARCHAR,
    swapTree            JSON,
    claimPubKey         VARCHAR,
    preimage            VARCHAR,
    redeemScript        VARCHAR,
    invoice             VARCHAR,
    paymentHash         VARCHAR(64) NOT NULL,
    address             VARCHAR,
    expectedAmount      INT,
    timeoutBlockheight  INTEGER,
    lockupTransactionId VARCHAR,
    refundTransactionId VARCHAR,
    refundAddress       VARCHAR DEFAULT '',
    blindingKey         VARCHAR,
    isAuto              BOOLEAN DEFAULT 0,
    serviceFee          INT,
    serviceFeePercent   REAL,
    onchainFee          INT,
    createdAt           INT,
    walletId            INT REFERENCES wallets (id) ON DELETE SET NULL,
    tenantId            INT REFERENCES tenants (id)
);

CREATE TABLE chainSwaps
(
    id                VARCHAR PRIMARY KEY,
    fromCurrency      VARCHAR,
    toCurrency        VARCHAR,
    state             INT,
    error             VARCHAR,
    status            VARCHAR,
    acceptZeroConf    BOOLEAN,
    preimage          VARCHAR,
    isAuto            BOOLEAN DEFAULT 0,
    serviceFee        INT,
    serviceFeePercent REAL,
    onchainFee        INT,
    createdAt         INT,
    tenantId          INT REFERENCES tenants (id)
);

CREATE TABLE chainSwapsData
(
    id                  VARCHAR,
    currency            VARCHAR,
    privateKey          VARCHAR,
    theirPublicKey      VARCHAR,
    tree                JSON,
    amount              INTEGER,
    timeoutBlockheight  INTEGER,
    lockupTransactionId VARCHAR,
    transactionId       VARCHAR,
    address             VARCHAR,
    lockupAddress       VARCHAR,
    blindingKey         VARCHAR,
    walletId            INT REFERENCES wallets (id) ON DELETE SET NULL,

    PRIMARY KEY (id, currency)
);

CREATE TABLE reverseSwaps
(
    id                  VARCHAR PRIMARY KEY,
    fromCurrency        VARCHAR,
    toCurrency          VARCHAR,
    chanIds             JSON,
    state               INT,
    error               VARCHAR,
    status              VARCHAR,
    acceptZeroConf      BOOLEAN,
    privateKey          VARCHAR,
    swapTree            JSON,
    refundPubKey        VARCHAR,
    preimage            VARCHAR,
    redeemScript        VARCHAR,
    invoice             VARCHAR,
    claimAddress        VARCHAR,
    onchainAmount       INT,
    invoiceAmount       INT,
    timeoutBlockheight  INTEGER,
    lockupTransactionId VARCHAR,
    claimTransactionId  VARCHAR,
    blindingKey         VARCHAR,
    isAuto              BOOLEAN DEFAULT 0,
    routingFeeMsat      INT,
    serviceFee          INT,
    serviceFeePercent   REAL    DEFAULT 0,
    onchainFee          INT,
    createdAt           INT,
    paidAt              INT,
    externalPay         BOOLEAN,
    walletId            INT REFERENCES wallets (id) ON DELETE SET NULL,
    tenantId            INT REFERENCES tenants (id),
    routingFeeLimitPpm  INT
);
CREATE TABLE autobudget
(
    startDate INTEGER NOT NULL,
    endDate   INTEGER NOT NULL,
    name      VARCHAR NOT NULL,
    tenantId  INT REFERENCES tenants (id),

    PRIMARY KEY (startDate, name, tenantId)
);
CREATE TABLE wallets
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           VARCHAR,
    currency       VARCHAR,
    nodePubkey     VARCHAR,
    xpub           VARCHAR,
    coreDescriptor VARCHAR,
    mnemonic       VARCHAR,
    subaccount     INT,
    salt           VARCHAR,
    tenantId       INT NOT NULL REFERENCES tenants (id),
    legacy         BOOLEAN DEFAULT FALSE,
    lastIndex      INT,

    UNIQUE (name, tenantId, nodePubkey),
    UNIQUE (xpub, coreDescriptor, mnemonic, nodePubkey)
);
CREATE TABLE tenants
(
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR UNIQUE
);
` + createViews

type Database struct {
	Path string `long:"database.path" description:"Path to the database file"`

	db *sql.DB
	tx *sql.Tx

	lock sync.RWMutex
}

type Transaction struct {
	Database
}

type row interface {
	Scan(dest ...any) error
}

type JsonScanner[T any] struct {
	Value    T
	Nullable bool
}

func (j *JsonScanner[T]) Scan(src any) error {
	if (src == nil || src == "") && j.Nullable {
		return nil
	}
	if str, ok := src.(string); ok {
		return json.Unmarshal([]byte(str), &j.Value)
	}
	return fmt.Errorf("unsupported type: %T", src)
}

type PrivateKeyScanner struct {
	Value    *btcec.PrivateKey
	Nullable bool
}

func (s *PrivateKeyScanner) Scan(src any) (err error) {
	if src == nil && s.Nullable {
		return nil
	}
	if str, ok := src.(string); ok {
		s.Value, err = ParsePrivateKey(str)
		return err
	}
	return fmt.Errorf("unsupported type: %T", src)
}

type PublicKeyScanner struct {
	Value    *btcec.PublicKey
	Nullable bool
}

func (s *PublicKeyScanner) Scan(src any) (err error) {
	if (src == nil || src == "") && s.Nullable {
		return nil
	}
	if str, ok := src.(string); ok {
		s.Value, err = ParsePublicKey(str)
		return err
	}
	return fmt.Errorf("unsupported type: %T", src)
}

func (database *Database) BeginTx() (*Transaction, error) {
	tx, err := database.db.Begin()
	if err != nil {
		return nil, err
	}
	return &Transaction{
		Database{tx: tx},
	}, nil
}

func (database *Database) RunTx(run func(tx *Transaction) error) error {
	tx, err := database.BeginTx()
	if err != nil {
		return err
	}
	if err := run(tx); err != nil {
		return tx.Rollback(err)
	}
	return tx.Commit()
}

func (transaction *Transaction) Commit() error {
	return transaction.tx.Commit()
}

func (transaction *Transaction) Rollback(cause error) error {
	if err := transaction.tx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback: %w: %w", err, cause)
	}
	return cause
}

type Id = uint64

type SwapQuery struct {
	From     *boltz.Currency
	To       *boltz.Currency
	States   []boltzrpc.SwapState
	Include  boltzrpc.IncludeSwaps
	Since    time.Time
	TenantId *Id
	Limit    *uint64
	Offset   *uint64
}

var PendingSwapQuery = SwapQuery{
	States: []boltzrpc.SwapState{boltzrpc.SwapState_PENDING},
}

var FailedSwapQuery = SwapQuery{
	States: []boltzrpc.SwapState{boltzrpc.SwapState_ERROR, boltzrpc.SwapState_SERVER_ERROR, boltzrpc.SwapState_REFUNDED},
}

func (query *SwapQuery) ToWhereClauseWithExisting(conditions []string, values []any) (string, []any) {
	if query.From != nil {
		conditions = append(conditions, "fromCurrency = ?")
		values = append(values, *query.From)
	}
	if query.To != nil {
		conditions = append(conditions, "toCurrency = ?")
		values = append(values, *query.To)
	}
	if query.States != nil {
		states := make([]string, len(query.States))
		for i, state := range query.States {
			states[i] = "?"
			values = append(values, state)
		}
		conditions = append(conditions, "state IN ("+strings.Join(states, ",")+")")
	}
	if query.Include == boltzrpc.IncludeSwaps_MANUAL {
		conditions = append(conditions, "isAuto = ?")
		values = append(values, false)
	}
	if query.Include == boltzrpc.IncludeSwaps_AUTO {
		conditions = append(conditions, "isAuto = ?")
		values = append(values, true)
	}
	if !query.Since.IsZero() {
		conditions = append(conditions, "createdAt >= ?")
		values = append(values, query.Since.Unix())
	}
	if query.TenantId != nil {
		conditions = append(conditions, "tenantId = ?")
		values = append(values, query.TenantId)
	}
	var where string
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	where += " ORDER BY createdAt DESC"
	if query.Limit != nil {
		where += " LIMIT ?"
		values = append(values, *query.Limit)
	}
	if query.Offset != nil {
		where += " OFFSET ?"
		values = append(values, *query.Offset)
	}
	return where, values
}

func (query *SwapQuery) ToWhereClause() (string, []any) {
	return query.ToWhereClauseWithExisting([]string{}, []any{})
}

func (database *Database) Connect() error {
	if database.db == nil {
		logger.Info("Opening database: " + database.Path)
		db, err := sql.Open("sqlite3", database.Path)

		if err != nil {
			return err
		}

		database.db = db

		if err := database.migrate(); err != nil {
			return err
		}

		if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return err
		}

		if err := database.CreateDefaultTenant(); err != nil {
			return err
		}
	}
	return nil
}

func (database *Database) Exec(query string, args ...any) (sql.Result, error) {
	database.lock.Lock()
	defer database.lock.Unlock()
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.Exec(query, args...)
	}
	return database.db.Exec(query, args...)
}

func (database *Database) Query(query string, args ...any) (*sql.Rows, error) {
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.Query(query, args...)
	}
	return database.db.Query(query, args...)
}

func (database *Database) QueryRow(query string, args ...any) *sql.Row {
	logger.Silly("Executing query: " + query)
	if database.tx != nil {
		return database.tx.QueryRow(query, args...)
	}
	return database.db.QueryRow(query, args...)
}

func (database *Database) QueryAnySwap(id string) (*Swap, *ReverseSwap, *ChainSwap, error) {
	swap, err := database.QuerySwap(id)
	if err == nil {
		return swap, nil, nil, nil
	}

	reverseSwap, err := database.QueryReverseSwap(id)
	if err == nil {
		return nil, reverseSwap, nil, nil
	}

	chainSwap, err := database.QueryChainSwap(id)
	if err == nil {
		return nil, nil, chainSwap, nil
	}

	return nil, nil, nil, fmt.Errorf("could not find any type of Swap with ID %s", id)
}

func (database *Database) QueryAllRefundableSwaps(tenantId *Id, currency boltz.Currency, currentHeight uint32) ([]*Swap, []*ChainSwap, error) {
	swaps, err := database.QueryRefundableSwaps(tenantId, currency, currentHeight)
	if err != nil {
		return nil, nil, err
	}
	chainSwaps, err := database.QueryRefundableChainSwaps(tenantId, currency, currentHeight)
	if err != nil {
		return nil, nil, err
	}
	return swaps, chainSwaps, nil
}

func (database *Database) QueryAllClaimableSwaps(tenantId *Id, currency boltz.Currency) ([]*ReverseSwap, []*ChainSwap, error) {
	reverseSwaps, err := database.QueryClaimableReverseSwaps(tenantId, currency)
	if err != nil {
		return nil, nil, err
	}
	chainSwaps, err := database.QueryClaimableChainSwaps(tenantId, currency)
	if err != nil {
		return nil, nil, err
	}
	return reverseSwaps, chainSwaps, nil
}

func (database *Database) createTables() error {
	_, err := database.Exec(createTables)
	return err
}

func ParsePrivateKey(privateKeyHex string) (*btcec.PrivateKey, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, err
	}
	priv, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	return priv, nil
}

func ParsePublicKey(publicKeyHex string) (*btcec.PublicKey, error) {
	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, err
	}
	return btcec.ParsePubKey(pubKeyBytes)
}

func formatPrivateKey(key *btcec.PrivateKey) string {
	if key == nil {
		return ""
	}
	return hex.EncodeToString(key.Serialize())
}

func formatPublicKey(key *btcec.PublicKey) string {
	if key == nil {
		return ""
	}
	return hex.EncodeToString(key.SerializeCompressed())
}

func parseTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

func FormatTime(t time.Time) int64 {
	if t.IsZero() {
		return time.Now().Unix()
	}
	return t.Unix()
}

func parseNullUint(value sql.NullInt64) *uint64 {
	if value.Valid {
		value := uint64(value.Int64)
		return &value
	}
	return nil
}

func parseNullInt(value sql.NullInt64) *int64 {
	if value.Valid {
		return &value.Int64
	}
	return nil
}

func addToOptional[V constraints.Integer](value *V, add V) *V {
	if value != nil {
		*value += add
	} else {
		value = &add
	}
	return value
}

func formatJson(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		logger.Errorf("Could not marshal json value %v: %v", value, err)
	}
	return string(encoded)
}

func closeRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		logger.Errorf("Error closing rows: %v", err)
	}
}
