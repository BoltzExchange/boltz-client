package onchain

import (
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type Balance struct {
	Total       uint64
	Confirmed   uint64
	Unconfirmed uint64
}

type TransactionOutput struct {
	Address      string
	Amount       uint64
	IsOurAddress bool
}

type WalletTransaction struct {
	Id              string
	Timestamp       time.Time
	Outputs         []TransactionOutput
	BlockHeight     uint32
	BalanceChange   int64
	IsConsolidation bool
}

type WalletInfo struct {
	Id       Id
	Name     string
	Currency boltz.Currency
	Readonly bool
	TenantId Id
}

type WalletSendArgs struct {
	Address     string
	Amount      uint64
	SatPerVbyte float64
	SendAll     bool
}

type Wallet interface {
	NewAddress() (string, error)
	SendToAddress(args WalletSendArgs) (string, error)
	Ready() bool
	GetBalance() (*Balance, error)
	GetWalletInfo() WalletInfo
	Disconnect() error
	GetTransactions(limit, offset uint64) ([]*WalletTransaction, error)
	BumpTransactionFee(txId string, satPerVbyte float64) (string, error)
	GetSendFee(args WalletSendArgs) (send uint64, fee uint64, err error)
	GetOutputs(address string) ([]*Output, error)
}

func (info WalletInfo) InsufficientBalanceError(amount uint64) error {
	return fmt.Errorf("wallet %s has insufficient balance for sending %d sats", info.Name, amount)
}

func (info WalletInfo) String() string {
	return fmt.Sprintf("Wallet{Id: %d, Name: %s, Currency: %s}", info.Id, info.Name, info.Currency)
}

type ScriptType string

const (
	ScriptTypeWpkh   ScriptType = "wpkh"
	ScriptTypeShWpkh ScriptType = "shwpkh"
)

type SigningData struct {
	Mnemonic   string
	ScriptType ScriptType
	Subaccount *uint64
}

type Readonly struct {
	Xpub           string
	CoreDescriptor string
}

type WalletCredentials struct {
	WalletInfo
	SigningData
	Readonly
	Salt   string
	Legacy bool
}

func (c *WalletCredentials) Encrypted() bool {
	return c.Salt != ""
}

func (c *WalletCredentials) Decrypt(password string) (*WalletCredentials, error) {
	if !c.Encrypted() {
		return nil, errors.New("credentials are not encrypted")
	}
	decrypted := *c
	var err error

	if decrypted.Xpub != "" {
		decrypted.Xpub, err = decrypt(decrypted.Xpub, password, decrypted.Salt)
	} else if decrypted.CoreDescriptor != "" {
		decrypted.CoreDescriptor, err = decrypt(decrypted.CoreDescriptor, password, decrypted.Salt)
	} else if decrypted.Mnemonic != "" {
		decrypted.Mnemonic, err = decrypt(decrypted.Mnemonic, password, decrypted.Salt)
	}
	decrypted.Salt = ""
	return &decrypted, err
}

func (c *WalletCredentials) Encrypt(password string) (*WalletCredentials, error) {
	if c.Encrypted() {
		return nil, errors.New("credentials are already encrypted")
	}
	var err error

	encrypted := *c
	encrypted.Salt, err = generateSalt()
	if err != nil {
		return nil, fmt.Errorf("could not generate new salt: %w", err)
	}

	if encrypted.Xpub != "" {
		encrypted.Xpub, err = encrypt(encrypted.Xpub, password, encrypted.Salt)
	} else if encrypted.CoreDescriptor != "" {
		encrypted.CoreDescriptor, err = encrypt(encrypted.CoreDescriptor, password, encrypted.Salt)
	} else if encrypted.Mnemonic != "" {
		encrypted.Mnemonic, err = encrypt(encrypted.Mnemonic, password, encrypted.Salt)
	}
	return &encrypted, err
}
