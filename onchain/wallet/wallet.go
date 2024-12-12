package wallet

/*
#cgo CFLAGS: -I${SRCDIR}/include/
#include "gdk.h"
#include <stdlib.h>
#include <stdio.h>

extern void go_notification_handler(GA_json* details);

void notification_handler(void* context, GA_json* details) {
	go_notification_handler(details);
}
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/btcsuite/btcd/wire"
	"github.com/mitchellh/mapstructure"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
)

const MinFeeRate = 0.01
const MaxInputs = uint64(255) // TODO: change back to 256 when gdk is fixed
const DefaultAutoConsolidateThreshold = uint64(200)
const GapLimit = 100

type TransactionNotification struct {
	TxId     string
	Currency boltz.Currency
}

var ErrSubAccountNotSet = errors.New("subaccount not set")
var TransactionNotifier = utils.ForwardChannel(make(chan TransactionNotification), 0, false)

type AuthHandler = *C.struct_GA_auth_handler
type Json = *C.GA_json
type Session = *C.struct_GA_session

type Credentials struct {
	onchain.WalletInfo
	Mnemonic       string  `json:"mnemonic"`
	Subaccount     *uint64 `json:"subaccount"`
	Xpub           string  `json:"xpub"`
	CoreDescriptor string  `json:"core_descriptor"`
	Salt           string
}

func (c *Credentials) Encrypted() bool {
	return c.Salt != ""
}

func (c *Credentials) Decrypt(password string) (*Credentials, error) {
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

func (c *Credentials) Encrypt(password string) (*Credentials, error) {
	if c.Encrypted() {
		return nil, errors.New("credentails are already encrypted")
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

type Subaccount struct {
	Pointer         uint64   `json:"pointer"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Used            bool     `json:"bip44_discovered"`
	CoreDescriptors []string `json:"core_descriptors"`
}

type Wallet struct {
	onchain.WalletInfo
	subaccount       *uint64
	session          Session
	connected        bool
	syncedAccounts   []uint64
	txProvider       onchain.TxProvider
	spentOutputs     map[string]bool
	spentOutputsLock sync.RWMutex
	txNotifier       <-chan TransactionNotification
}

type Config struct {
	DataDir                  string
	Network                  *boltz.Network
	Debug                    bool
	Electrum                 onchain.ElectrumConfig
	AutoConsolidateThreshold uint64
	MaxInputs                uint64
}

func (c *Config) Validate() error {
	if c.MaxInputs == 0 {
		c.MaxInputs = MaxInputs
	} else if c.MaxInputs > MaxInputs {
		return fmt.Errorf("max inputs must be less than %d", MaxInputs)
	}
	if c.AutoConsolidateThreshold != 0 {
		if c.AutoConsolidateThreshold < 10 {
			return errors.New("auto consolidate threshold must be at least 10")
		}
		if c.AutoConsolidateThreshold > c.MaxInputs {
			return fmt.Errorf("auto consolidate threshold must be less than %d", c.MaxInputs)
		}
	}
	return nil
}

var config *Config

type syncListener struct {
	accounts []uint64
	done     chan bool
}

var syncListeners []*syncListener
var syncListenersLock = sync.Mutex{}

func toErr(ret C.int) error {
	if ret == C.GA_OK {
		return nil
	}
	var output Json
	if C.GA_get_thread_error_details(&output) != C.GA_OK {
		return errors.New("failed to get error details")
	}
	var result struct {
		Details string `json:"details"`
	}
	if err := parseJson(output, &result); err != nil {
		return err
	}
	return fmt.Errorf("failed with code %v: %v", ret, result.Details)
}

func toJson(data any) (result Json, free func()) {
	bytes, err := json.Marshal(data)
	if err != nil {
		logger.Error("failed to convert json: " + err.Error())
		return nil, nil
	}
	cStr := C.CString(string(bytes))
	defer C.free(unsafe.Pointer(cStr))
	ret := C.GA_convert_string_to_json(cStr, &result)
	if ret != C.GA_OK {
		logger.Error("failed to convert json: " + string(bytes))
	}
	return result, func() { freeJson(result) }
}

func freeJson(json Json) {
	if err := toErr(C.GA_destroy_json(json)); err != nil {
		logger.Error("failed to free json: " + err.Error())
	}
}

func parseJson[V any](output Json, value *V) error {
	cStr := C.CString("")
	defer freeJson(output)
	if C.GA_convert_json_to_string(output, &cStr) != C.GA_OK {
		return errors.New("failed to convert json to string")
	}
	goStr := []byte(C.GoString(cStr))
	C.free(unsafe.Pointer(cStr))
	return json.Unmarshal(goStr, value)
}

func withOutput[V any](ret C.int, output Json, value *V) error {
	err := toErr(ret)
	if err != nil {
		return err
	}
	return parseJson(output, value)
}

func withAuthHandler[R any](ret C.int, handler *AuthHandler, result *R) (err error) {
	if handler == nil {
		return errors.New("auth handler is nil")
	}
	defer C.GA_destroy_auth_handler(*handler)
	var output Json

	if err := toErr(ret); err != nil {
		return err
	}
	var handlerStatus struct {
		Status       string         `json:"status"`
		Result       R              `json:"result"`
		Action       string         `json:"action"`
		Error        string         `json:"error"`
		RequiredData map[string]any `json:"required_data"`
	}
	if err := withOutput(C.GA_auth_handler_get_status(*handler, &output), output, &handlerStatus); err != nil {
		return err
	}

	if handlerStatus.Status == "error" {
		return errors.New(handlerStatus.Error)
	} else if handlerStatus.Status == "resolve_code" {
		return errors.New("resolve code not implemented")
	}
	if result != nil {
		*result = handlerStatus.Result
	}
	return nil
}

func Initialized() bool {
	return config != nil
}

func Init(walletConfig Config) error {
	if config != nil {
		return errors.New("already initialized")
	}
	if err := walletConfig.Validate(); err != nil {
		return err
	}
	walletConfig.DataDir += "/wallet"
	params := map[string]any{
		"datadir":   walletConfig.DataDir,
		"log_level": "error",
	}
	if walletConfig.Debug {
		params["log_level"] = "debug"
	}
	paramsJson, free := toJson(params)
	defer free()
	if err := toErr(C.GA_init(paramsJson)); err != nil {
		return err
	}
	config = &walletConfig

	registerHandler(subaccountNotification, func(data map[string]any) {
		var parsed struct {
			Pointer   uint64
			EventType string `mapstructure:"event_type"`
		}
		if err := mapstructure.Decode(data, &parsed); err != nil {
			logger.Errorf("Could not parse subaccount notification: %v", data)
			return
		}
		if parsed.EventType == "synced" {
			syncListenersLock.Lock()
			for i, listener := range syncListeners {
				if slices.Contains(listener.accounts, parsed.Pointer) {
					listener.accounts = slices.DeleteFunc(listener.accounts, func(u uint64) bool {
						return u == parsed.Pointer
					})
					if len(listener.accounts) == 0 {
						listener.done <- true
						syncListeners = slices.Delete(syncListeners, i, i)
					}
				}
			}
			syncListenersLock.Unlock()
		}
	})

	registerHandler(transactionNotification, func(data map[string]any) {
		var parsed struct {
			Transaction string `mapstructure:"txhash"`
			Type        string `mapstructure:"type"`
		}
		if err := mapstructure.Decode(data, &parsed); err != nil {
			logger.Errorf("Could not parse subaccount notification: %v", data)
			return
		}
		currency := boltz.CurrencyBtc
		if parsed.Type == "" {
			currency = boltz.CurrencyLiquid
		}
		TransactionNotifier.Send(TransactionNotification{
			TxId:     parsed.Transaction,
			Currency: currency,
		})
	})

	return nil
}

func UpdateConfig(walletConfig Config) {
	if config != nil {
		config.AutoConsolidateThreshold = walletConfig.AutoConsolidateThreshold
		config.MaxInputs = walletConfig.MaxInputs
	}
}

func (wallet *Wallet) Connect() error {
	if wallet.connected {
		return nil
	}

	if err := toErr(C.GA_create_session(&wallet.session)); err != nil {
		return err
	}

	if err := toErr(C.GA_set_notification_handler(wallet.session, C.GA_notification_handler(C.notification_handler), nil)); err != nil {
		return err
	}

	params := map[string]any{
		// gdk uses sat/kVB
		"min_fee_rate": MinFeeRate * 1000,
		"gap_limit":    GapLimit,
	}
	var electrum onchain.ElectrumOptions
	if wallet.Currency == boltz.CurrencyBtc {
		electrum = config.Electrum.Btc
		if config.Network == boltz.MainNet {
			params["name"] = "electrum-mainnet"
		} else if config.Network == boltz.TestNet {
			params["name"] = "electrum-testnet"
		} else if config.Network == boltz.Regtest {
			params["name"] = "electrum-localtest"
		} else {
			return errors.New("unknown network")
		}
	} else if wallet.Currency == boltz.CurrencyLiquid {
		electrum = config.Electrum.Liquid
		if config.Network == boltz.MainNet {
			params["name"] = "electrum-liquid"
		} else if config.Network == boltz.TestNet {
			params["name"] = "electrum-testnet-liquid"
		} else if config.Network == boltz.Regtest {
			params["name"] = "electrum-localtest-liquid"
		} else {
			return errors.New("unknown network")
		}
	} else {
		return errors.New("unknown currency")
	}
	if electrum.Url != "" {
		params["electrum_url"] = electrum.Url
		params["electrum_tls"] = electrum.SSL
	}
	paramsJson, free := toJson(params)
	defer free()

	if err := toErr(C.GA_connect(wallet.session, paramsJson)); err != nil {
		return err
	}

	wallet.connected = true

	if config.AutoConsolidateThreshold > 0 && !wallet.Readonly {
		go func() {
			if err := wallet.autoConsolidate(); err != nil {
				logger.Errorf("Auto consolidation failed: %v", err)
			}
			wallet.txNotifier = TransactionNotifier.Get()
			for range wallet.txNotifier {
				if err := wallet.autoConsolidate(); err != nil {
					logger.Errorf("Auto consolidation failed: %v", err)
				}
			}
		}()
	}

	return nil
}

func (wallet *Wallet) GetSubaccounts(refresh bool) ([]*Subaccount, error) {
	details, free := toJson(map[string]any{"refresh": refresh})
	defer free()
	handler := new(AuthHandler)
	var result struct {
		Subaccounts []*Subaccount `json:"subaccounts"`
	}

	err := withAuthHandler(C.GA_get_subaccounts(wallet.session, details, handler), handler, &result)
	if err != nil {
		return nil, err
	}

	var newAccounts []uint64
	for _, account := range result.Subaccounts {
		if !slices.Contains(wallet.syncedAccounts, account.Pointer) {
			newAccounts = append(newAccounts, account.Pointer)
			wallet.syncedAccounts = append(wallet.syncedAccounts, account.Pointer)
		}
	}
	if len(newAccounts) > 0 {
		done := make(chan bool)
		syncListenersLock.Lock()
		syncListeners = append(syncListeners, &syncListener{
			done:     done,
			accounts: newAccounts,
		})
		syncListenersLock.Unlock()
		<-done
	}
	return result.Subaccounts, nil
}

func (wallet *Wallet) SetSubaccount(subaccount *uint64) (*uint64, error) {
	accounts, err := wallet.GetSubaccounts(false)
	if err != nil {
		return nil, err
	}
	handler := new(AuthHandler)
	if subaccount == nil {
		accType := "p2wpkh"
		for _, account := range accounts {
			if account.Type == accType && !account.Used {
				subaccount = &account.Pointer
			}
		}
		if subaccount == nil {
			details, free := toJson(map[string]any{"name": "boltzd", "type": accType})
			defer free()
			var result struct {
				Pointer uint64 `json:"pointer"`
			}
			err := withAuthHandler(C.GA_create_subaccount(wallet.session, details, handler), handler, &result)
			if err != nil {
				return nil, err
			}

			subaccount = &result.Pointer
		}
	} else {
		var result any
		err := withAuthHandler(C.GA_get_subaccount(wallet.session, C.uint32_t(*subaccount), handler), handler, &result)
		if err != nil {
			return nil, err
		}
	}
	logger.Debugf("Setting subaccount to %v", *subaccount)
	wallet.subaccount = subaccount
	return subaccount, err
}

func (wallet *Wallet) GetSubaccount(pointer uint64) (*Subaccount, error) {
	var result Subaccount
	handler := new(AuthHandler)
	err := withAuthHandler(C.GA_get_subaccount(wallet.session, C.uint32_t(pointer), handler), handler, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (wallet *Wallet) CurrentSubaccount() (uint64, error) {
	if wallet.subaccount == nil {
		return 0, ErrSubAccountNotSet
	}
	return *wallet.subaccount, nil
}

func Login(credentials *Credentials) (*Wallet, error) {
	if credentials.Encrypted() {
		return nil, errors.New("credentials are encrypted")
	}
	wallet := &Wallet{WalletInfo: credentials.WalletInfo, spentOutputs: make(map[string]bool)}
	login := make(map[string]any)

	if credentials.Mnemonic != "" {
		login["mnemonic"] = credentials.Mnemonic
		wallet.Readonly = false
	} else if credentials.Xpub != "" {
		if credentials.Currency == boltz.CurrencyLiquid {
			return nil, errors.New("xpub not supported for liquid")
		}
		login["slip132_extended_pubkeys"] = []string{credentials.Xpub}
		wallet.Readonly = true
	} else if credentials.CoreDescriptor != "" {
		login["core_descriptors"] = strings.Split(credentials.CoreDescriptor, "\n")
		wallet.Readonly = true
	} else {
		return nil, errors.New("no login found in credentials")
	}

	if err := wallet.Connect(); err != nil {
		return nil, err
	}

	loginJson, freeLogin := toJson(login)
	defer freeLogin()
	hwDevice, freeDevice := toJson(nil)
	defer freeDevice()

	handler := new(AuthHandler)
	var result any
	if err := withAuthHandler(C.GA_login_user(wallet.session, hwDevice, loginJson, handler), handler, &result); err != nil {
		if strings.Contains(err.Error(), "checksum") {
			return nil, errors.New("invalid xpub")
		}
		return nil, err
	}
	logger.Debugf("Logged in: %v", result)

	_, err := wallet.GetSubaccounts(false)
	if err != nil {
		return nil, err
	}

	if credentials.Subaccount != nil {
		if _, err := wallet.SetSubaccount(credentials.Subaccount); err != nil {
			logger.Warnf("Failed to set subaccount for wallet %s, Resyncing subaccounts", credentials.Name)
			if _, err := wallet.GetSubaccounts(true); err != nil {
				return nil, err
			}
			if _, err := wallet.SetSubaccount(credentials.Subaccount); err != nil {
				subaccount, err := wallet.SetSubaccount(nil)
				if err != nil {
					logger.Errorf("Failed to set existing and new subaccount for wallet %s: %v", credentials.Name, err)
					return nil, err
				}
				if *subaccount != *credentials.Subaccount {
					logger.Infof("Subaccount %d was not found, using new subaccount %d", *credentials.Subaccount, *subaccount)
				}
			}
		}
	}

	return wallet, nil
}

func GenerateMnemonic() (string, error) {
	mnemonic := C.CString("")
	defer C.free(unsafe.Pointer(mnemonic))
	if err := toErr(C.GA_generate_mnemonic_12(&mnemonic)); err != nil {
		return "", errors.New("failed to generate mnemonic: " + err.Error())
	}
	return C.GoString(mnemonic), nil
}

func (wallet *Wallet) SetTxProvider(txProvider onchain.TxProvider) {
	wallet.txProvider = txProvider
}

func (wallet *Wallet) Disconnect() error {
	if !wallet.connected {
		return nil
	}
	if wallet.txNotifier != nil {
		TransactionNotifier.Remove(wallet.txNotifier)
	}
	if err := toErr(C.GA_destroy_session(wallet.session)); err != nil {
		return err
	}
	wallet.connected = false
	wallet.session = nil

	return nil
}

func (wallet *Wallet) NewAddress() (string, error) {
	if wallet.subaccount == nil {
		return "", ErrSubAccountNotSet
	}
	params, free := toJson(map[string]any{
		"subaccount": *wallet.subaccount,
	})
	defer free()
	handler := new(AuthHandler)
	var details struct {
		Address string `json:"address"`
	}
	err := withAuthHandler(C.GA_get_receive_address(wallet.session, params, handler), handler, &details)
	if err != nil {
		return "", err
	}
	return details.Address, nil
}

func (wallet *Wallet) getSubaccountBalance(subaccount uint64, includeUnconfirmed bool) (uint64, error) {
	outputs, err := wallet.getUnspentOutputs(subaccount, includeUnconfirmed)
	if err != nil {
		return 0, err
	}
	var sum uint64
	for _, outputs := range outputs.Unspent {
		for _, output := range outputs {
			amount, _ := output["satoshi"].(float64)
			sum += uint64(amount)
		}
	}

	return sum, nil
}

func (wallet *Wallet) GetSubaccountBalance(subaccount uint64) (*onchain.Balance, error) {
	total, err := wallet.getSubaccountBalance(subaccount, true)
	if err != nil {
		return nil, err
	}
	confirmed, err := wallet.getSubaccountBalance(subaccount, false)
	if err != nil {
		return nil, err
	}
	return &onchain.Balance{
		Total:       total,
		Confirmed:   confirmed,
		Unconfirmed: total - confirmed,
	}, nil
}

func (wallet *Wallet) GetBalance() (*onchain.Balance, error) {
	if wallet.subaccount == nil {
		return nil, ErrSubAccountNotSet
	}
	return wallet.GetSubaccountBalance(*wallet.subaccount)
}

func (wallet *Wallet) GetOutputs(address string) ([]*onchain.Output, error) {
	transactoins, err := wallet.GetTransactions(0, 0)
	if err != nil {
		return nil, err
	}
	var result []*onchain.Output
	for _, tx := range transactoins {
		for _, output := range tx.Outputs {
			if output.Address == address {
				result = append(result, &onchain.Output{
					TxId:  tx.Id,
					Value: output.Amount,
				})
			}
		}
	}
	return result, nil
}

type outputs struct {
	Unspent map[string][]map[string]any `json:"unspent_outputs"`
}

func (wallet *Wallet) getUnspentOutputs(subaccount uint64, includeUnconfirmed bool) (*outputs, error) {
	details := map[string]any{"subaccount": subaccount}
	if includeUnconfirmed {
		details["num_confs"] = 0
	} else {
		details["num_confs"] = 1
	}
	params, free := toJson(details)
	defer free()

	handler := new(AuthHandler)
	result := &outputs{}
	if err := withAuthHandler(C.GA_get_unspent_outputs(wallet.session, params, handler), handler, result); err != nil {
		return nil, err
	}
	wallet.spentOutputsLock.Lock()
	defer wallet.spentOutputsLock.Unlock()
	for spent := range wallet.spentOutputs {
		found := false
		for key, outputs := range result.Unspent {
			for i, output := range outputs {
				if output["txhash"] == spent {
					logger.Debugf("Ignoring output for tx %s since it is marked as spent", spent)
					result.Unspent[key] = append(outputs[:i], outputs[i+1:]...)
					found = true
					break
				}
			}
		}
		if !found {
			delete(wallet.spentOutputs, spent)
		}
	}

	return result, nil
}

func (wallet *Wallet) createTransaction(address string, amount uint64, satPerVbyte float64, sendAll bool) (map[string]any, error) {
	if wallet.Readonly {
		return nil, errors.New("wallet is readonly")
	}
	if wallet.subaccount == nil {
		return nil, ErrSubAccountNotSet
	}
	handler := new(AuthHandler)

	outputs, err := wallet.getUnspentOutputs(*wallet.subaccount, false)
	if err != nil {
		return nil, err
	}

	// Disable RBF
	for asset, current := range outputs.Unspent {
		if len(current) > int(config.MaxInputs) {
			if sendAll {
				outputs.Unspent[asset] = current[:config.MaxInputs]
			} else {
				return nil, errors.New("too many inputs")
			}
		}
		for _, output := range current {
			output["sequence"] = wire.MaxTxInSequenceNum - 1
		}
	}

	asset := ""
	if wallet.Currency == boltz.CurrencyLiquid {
		asset = config.Network.Liquid.AssetID
	}

	transactionDetails, free := toJson(map[string]any{
		// gdk uses sat/kVB
		"fee_rate": satPerVbyte * 1000,
		"addressees": []map[string]any{
			{
				"address":   address,
				"satoshi":   amount,
				"asset_id":  asset,
				"is_greedy": sendAll,
			},
		},
		"utxos": outputs.Unspent,
	})
	defer free()

	var result map[string]any
	if err := withAuthHandler(C.GA_create_transaction(wallet.session, transactionDetails, handler), handler, &result); err != nil {
		return nil, err
	}
	if err, ok := result["error"].(string); ok {
		if strings.Contains(err, "id_insufficient_funds") {
			return nil, wallet.InsufficientBalanceError(amount)
		}
	}
	return result, nil
}

func (wallet *Wallet) GetSendFee(address string, amount uint64, satPerVbyte float64, sendAll bool) (send uint64, fee uint64, err error) {
	result, err := wallet.createTransaction(address, amount, satPerVbyte, sendAll)
	if err != nil {
		return 0, 0, err
	}
	var createTx struct {
		Fee        uint64
		Addressees []struct {
			Satoshi uint64
		}
		Error string
	}
	if err := mapstructure.Decode(result, &createTx); err != nil {
		return 0, 0, err
	}
	if createTx.Error != "" {
		return 0, 0, errors.New(createTx.Error)
	}
	return createTx.Addressees[0].Satoshi, createTx.Fee, nil
}

func (wallet *Wallet) SendToAddress(address string, amount uint64, satPerVbyte float64, sendAll bool) (tx string, err error) {
	result, err := wallet.createTransaction(address, amount, satPerVbyte, sendAll)
	if err != nil {
		return "", err
	}

	wallet.spentOutputsLock.Lock()
	defer wallet.spentOutputsLock.Unlock()
	handler := new(AuthHandler)
	params, free := toJson(result)
	if err := withAuthHandler(C.GA_blind_transaction(wallet.session, params, handler), handler, &result); err != nil {
		return "", err
	}
	free()

	params, free = toJson(result)
	if err := withAuthHandler(C.GA_sign_transaction(wallet.session, params, handler), handler, &result); err != nil {
		return "", err
	}
	free()

	var signedTx struct {
		Transaction       string `mapstructure:"transaction"`
		Error             string `mapstructure:"error"`
		TransactionInputs []struct {
			TxId string `mapstructure:"txhash"`
		} `mapstructure:"transaction_inputs"`
	}
	if err := mapstructure.Decode(result, &signedTx); err != nil {
		return "", err
	}
	if signedTx.Error != "" {
		return "", fmt.Errorf("could not sign: %s", signedTx.Error)
	}

	if wallet.txProvider != nil {
		tx, err = wallet.txProvider.BroadcastTransaction(signedTx.Transaction)
	} else {
		params, free = toJson(result)
		var sendTx struct {
			TxHash string `json:"txhash"`
			Error  string `json:"error"`
		}
		if err := withAuthHandler(C.GA_send_transaction(wallet.session, params, handler), handler, &sendTx); err != nil {
			return "", err
		}
		free()

		if sendTx.Error != "" {
			err = errors.New(sendTx.Error)
		}
		tx = sendTx.TxHash
	}
	if err != nil {
		return "", fmt.Errorf("failed to broadcast: %w", err)
	}

	for _, input := range signedTx.TransactionInputs {
		wallet.spentOutputs[input.TxId] = true
	}

	return tx, nil
}

func (wallet *Wallet) SetSpentOutputs(outputs []string) {
	wallet.spentOutputsLock.Lock()
	defer wallet.spentOutputsLock.Unlock()
	wallet.spentOutputs = make(map[string]bool)
	for _, output := range outputs {
		wallet.spentOutputs[output] = true
	}
}

func (wallet *Wallet) GetTransactions(limit, offset uint64) ([]*onchain.WalletTransaction, error) {
	if wallet.subaccount == nil {
		return nil, ErrSubAccountNotSet
	}
	if limit == 0 {
		limit = 30
	}
	if limit > 30 {
		return nil, errors.New("limit cant be larger than 30")
	}
	params, free := toJson(map[string]any{
		"subaccount": *wallet.subaccount,
		"first":      offset,
		"count":      limit,
	})
	defer free()
	handler := new(AuthHandler)
	var result struct {
		Transactions []struct {
			BlockHeight uint32 `json:"block_height"`
			TxId        string `json:"txhash"`
			Inputs      []struct {
				Address    string `json:"address"`
				Satoshi    uint64 `json:"satoshi"`
				IsRelevant bool   `json:"is_relevant"`
			}
			Outputs []struct {
				Address    string `json:"address"`
				Satoshi    uint64 `json:"satoshi"`
				IsRelevant bool   `json:"is_relevant"`
			}
			Timestamp int64            `json:"created_at_ts"`
			Type      string           `json:"type"`
			Satoshi   map[string]int64 `json:"satoshi"`
		} `json:"transactions"`
	}
	if err := withAuthHandler(C.GA_get_transactions(wallet.session, params, handler), handler, &result); err != nil {
		return nil, err
	}

	asset := "btc"
	if wallet.Currency == boltz.CurrencyLiquid {
		asset = config.Network.Liquid.AssetID
	}

	var transactions []*onchain.WalletTransaction
	for _, tx := range result.Transactions {
		var outputs []onchain.TransactionOutput
		for _, out := range tx.Outputs {
			outputs = append(outputs, onchain.TransactionOutput{
				Address:      out.Address,
				Amount:       out.Satoshi,
				IsOurAddress: out.IsRelevant,
			})
		}
		transactions = append(transactions, &onchain.WalletTransaction{
			Id:              tx.TxId,
			Timestamp:       time.UnixMicro(tx.Timestamp),
			BlockHeight:     tx.BlockHeight,
			Outputs:         outputs,
			BalanceChange:   tx.Satoshi[asset],
			IsConsolidation: tx.Type == "redeposit",
		})
	}

	return transactions, nil
}

func (wallet *Wallet) Ready() bool {
	//return wallet.connected && wallet.subaccount != nil
	return wallet.connected
}

func (wallet *Wallet) GetWalletInfo() onchain.WalletInfo {
	return wallet.WalletInfo
}

func (wallet *Wallet) estimateFee() (float64, error) {
	var output Json
	var estimates struct {
		Fees []float64 `json:"fees"`
	}
	if err := withOutput(C.GA_get_fee_estimates(wallet.session, &output), output, &estimates); err != nil {
		return 0, err
	}
	// TODO: remove when proper floor is in gdk
	return max(estimates.Fees[0]/1000, 0.1), nil
}

func (wallet *Wallet) autoConsolidate() error {
	if wallet.subaccount != nil {
		unspent, err := wallet.getUnspentOutputs(*wallet.subaccount, false)
		if err != nil {
			return fmt.Errorf("failed to get unspent outputs: %v", err)
		}
		for _, outputs := range unspent.Unspent {
			if len(outputs) >= int(config.AutoConsolidateThreshold) {
				logger.Infof("Auto consolidating %d utxos", len(outputs))
				address, err := wallet.NewAddress()
				if err != nil {
					return fmt.Errorf("new address: %v", err)
				}
				feeRate, err := wallet.estimateFee()
				if err != nil {
					return fmt.Errorf("could not get estimate fee: %v", err)
				}
				logger.Infof("Using fee rate of %f sat/vbyte for consolidation", feeRate)
				if _, err := wallet.SendToAddress(address, 0, feeRate, true); err != nil {
					return fmt.Errorf("could not send: %v", err)
				}
			}
		}
	}
	return nil
}
