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
	"github.com/btcsuite/btcd/wire"
	"strings"
	"time"
	"unsafe"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
)

var ErrSubAccountNotSet = errors.New("subaccount not set")

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
	Pointer uint64 `json:"pointer"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Used    bool   `json:"bip44_discovered"`
}

type Wallet struct {
	onchain.WalletInfo
	subaccount         *uint64
	session            Session
	connected          bool
	blockHeight        uint32
	blockHeightChannel chan uint32
}

type Config struct {
	DataDir string
	Network *boltz.Network
	Debug   bool
}

var config *Config

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
	defer C.free(unsafe.Pointer(cStr))
	if C.GA_convert_json_to_string(output, &cStr) != C.GA_OK {
		return errors.New("failed to convert json to string")
	}
	freeJson(output)
	return json.Unmarshal([]byte(C.GoString(cStr)), value)
}

func withOutput[V any](ret C.int, output Json, value *V) error {
	err := toErr(ret)
	if err != nil {
		return err
	}
	return parseJson(output, value)
}

func withAuthHandler[R any](ret C.int, handler AuthHandler, result *R) (err error) {
	//defer C.GA_destroy_auth_handler(handler)
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
	if err := withOutput(C.GA_auth_handler_get_status(handler, &output), output, &handlerStatus); err != nil {
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

func newWallet() *Wallet {
	return &Wallet{}
}

func Initialized() bool {
	return config != nil
}

func Init(walletConfig Config) error {

	if config != nil {
		return errors.New("already initialized")
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

	return nil
}

func (wallet *Wallet) EncryptCredentials(mnemonic string, pin string) (map[string]any, error) {
	encrypt := map[string]any{
		"pin": pin,
		"plaintext": map[string]any{
			"mnemonic": mnemonic,
		},
	}
	credentialsJson, free := toJson(encrypt)
	defer free()
	var encrypted struct {
		PinData map[string]any `json:"pin_data"`
	}
	var handler AuthHandler
	if err := withAuthHandler(C.GA_encrypt_with_pin(wallet.session, credentialsJson, &handler), handler, &encrypted); err != nil {
		return nil, err
	}
	return encrypted.PinData, nil
}

func (wallet *Wallet) Connect() error {
	if wallet.connected {
		return nil
	}

	wallet.blockHeightChannel = make(chan uint32)

	registerHandler(blockNotification, func(data map[string]any) {
		blockHeight, ok := data["block_height"].(float64)
		if ok {
			wallet.blockHeight = uint32(blockHeight)
			select {
			case wallet.blockHeightChannel <- wallet.blockHeight:
				logger.Debugf("Sent block height update: %d", int(blockHeight))
			default:
			}
		} else {
			logger.Warnf("Could not parse block height from data: %v", data)
		}
	})

	if err := toErr(C.GA_create_session(&wallet.session)); err != nil {
		return err
	}

	if err := toErr(C.GA_set_notification_handler(wallet.session, C.GA_notification_handler(C.notification_handler), nil)); err != nil {
		return err
	}

	params := make(map[string]any)
	if wallet.Currency == boltz.CurrencyBtc {
		if config.Network == boltz.MainNet {
			params["name"] = "electrum-mainnet"
		} else if config.Network == boltz.TestNet {
			params["name"] = "electrum-testnet"
		} else if config.Network == boltz.Regtest {
			params["name"] = "electrum-localtest"
			params["electrum_url"] = "localhost:19001"
		} else {
			return errors.New("unknown network")
		}
	} else if wallet.Currency == boltz.CurrencyLiquid {
		if config.Network == boltz.MainNet {
			params["name"] = "electrum-liquid"
		} else if config.Network == boltz.TestNet {
			params["name"] = "electrum-testnet-liquid"
		} else if config.Network == boltz.Regtest {
			params["name"] = "electrum-localtest-liquid"
			params["electrum_url"] = "localhost:19002"
		} else {
			return errors.New("unknown network")
		}
	} else {
		return errors.New("unknown currency")
	}
	paramsJson, free := toJson(params)
	defer free()

	if err := toErr(C.GA_connect(wallet.session, paramsJson)); err != nil {
		return err
	}

	wallet.connected = true

	return nil
}

func (wallet *Wallet) GetSubaccounts(refresh bool) ([]*Subaccount, error) {
	details, free := toJson(map[string]any{"refresh": refresh})
	defer free()
	var handler AuthHandler
	var result struct {
		Subaccounts []*Subaccount `json:"subaccounts"`
	}

	err := withAuthHandler(C.GA_get_subaccounts(wallet.session, details, &handler), handler, &result)
	if err != nil {
		return nil, err
	}

	if refresh {
		for _, subaccount := range result.Subaccounts {
			// wait for subaccount to be synced
			logger.Debugf("%+v", subaccount)
			if subaccount.Used {
				var result struct {
					Transactions []any `json:"transactions"`
				}
				timeout := time.After(20 * time.Second)
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()
				for len(result.Transactions) == 0 {
					details, free = toJson(map[string]any{"subaccount": subaccount.Pointer, "first": 0, "count": 1})
					err := withAuthHandler(C.GA_get_transactions(wallet.session, details, &handler), handler, &result)
					free()
					if err != nil {
						return nil, fmt.Errorf("could not get transactions for subaccount %d: %w", subaccount.Pointer, err)
					}
					select {
					case <-ticker.C:
					case <-timeout:
						return nil, fmt.Errorf("timed out waiting for subaccount %d to sync", subaccount.Pointer)
					}
				}
			}
		}
	}
	return result.Subaccounts, nil
}

func (wallet *Wallet) SetSubaccount(subaccount *uint64) (*uint64, error) {
	accounts, err := wallet.GetSubaccounts(false)
	if err != nil {
		return nil, err
	}
	var handler AuthHandler
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
			err := withAuthHandler(C.GA_create_subaccount(wallet.session, details, &handler), handler, &result)
			if err != nil {
				return nil, err
			}

			subaccount = &result.Pointer
		}
	} else {
		var result any
		err := withAuthHandler(C.GA_get_subaccount(wallet.session, C.uint32_t(*subaccount), &handler), handler, &result)
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
	var handler AuthHandler
	err := withAuthHandler(C.GA_get_subaccount(wallet.session, C.uint32_t(pointer), &handler), handler, &result)
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
	wallet := &Wallet{WalletInfo: credentials.WalletInfo}
	login := make(map[string]any)

	if credentials.Mnemonic != "" {
		login["mnemonic"] = credentials.Mnemonic
		wallet.Readonly = false
	} else if credentials.Xpub != "" {
		login["slip132_extended_pubkeys"] = []string{credentials.Xpub}
		wallet.Readonly = true
	} else if credentials.CoreDescriptor != "" {
		login["core_descriptors"] = []string{credentials.CoreDescriptor}
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

	var handler AuthHandler
	var result any
	if err := withAuthHandler(C.GA_login_user(wallet.session, hwDevice, loginJson, &handler), handler, &result); err != nil {
		if strings.Contains(err.Error(), "checksum") {
			return nil, errors.New("invalid xpub")
		}
		return nil, err
	}
	logger.Debugf("Logged in: %v", result)

	if credentials.Subaccount != nil {
		if _, err := wallet.SetSubaccount(credentials.Subaccount); err != nil {
			logger.Errorf("Failed to set subaccount for wallet %s. You might have to resync it using GetSubaccounts: %v", credentials.Name, err)
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

func (wallet *Wallet) Remove() error {
	if !wallet.connected {
		return nil
	}

	var handler AuthHandler
	var result any
	if err := withAuthHandler(C.GA_remove_account(wallet.session, &handler), handler, &result); err != nil {
		return err
	}

	if err := toErr(C.GA_destroy_session(wallet.session)); err != nil {
		return err
	}
	wallet.connected = false
	wallet.session = nil

	removeHandler(blockNotification)
	close(wallet.blockHeightChannel)
	wallet.blockHeight = 0
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
	var handler AuthHandler
	var details struct {
		Address string `json:"address"`
	}
	err := withAuthHandler(C.GA_get_receive_address(wallet.session, params, &handler), handler, &details)
	if err != nil {
		return "", err
	}
	return details.Address, nil
}

func (wallet *Wallet) getSubaccountBalance(subaccount uint64, includeUnconfirmed bool) (uint64, error) {

	details := map[string]any{"subaccount": subaccount}
	if includeUnconfirmed {
		details["num_confs"] = 0
	} else {
		details["num_confs"] = 1
	}
	detailsJson, free := toJson(details)
	defer free()

	var handler AuthHandler
	var balances map[string]uint64
	err := withAuthHandler(C.GA_get_balance(wallet.session, detailsJson, &handler), handler, &balances)
	if err != nil {
		return 0, err
	}
	var sum uint64
	for _, balance := range balances {
		sum += balance
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

func (wallet *Wallet) SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error) {
	if wallet.Readonly {
		return "", errors.New("wallet is readonly")
	}
	if wallet.subaccount == nil {
		return "", ErrSubAccountNotSet
	}
	params, free := toJson(map[string]any{
		"subaccount": *wallet.subaccount,
		"num_confs":  0,
	})
	defer free()
	var handler AuthHandler

	var outputs struct {
		Unspent map[string][]map[string]any `json:"unspent_outputs"`
	}
	if err := withAuthHandler(C.GA_get_unspent_outputs(wallet.session, params, &handler), handler, &outputs); err != nil {
		return "", err
	}

	// Disable RBF
	for _, outputs := range outputs.Unspent {
		for _, output := range outputs {
			output["sequence"] = wire.MaxTxInSequenceNum - 1
		}
	}

	asset := ""
	if wallet.Currency == boltz.CurrencyLiquid {
		asset = config.Network.Liquid.AssetID
	}

	transactionDetails, free := toJson(map[string]any{
		"fee_rate": satPerVbyte * 1000,
		"addressees": []map[string]any{
			{
				"address":  address,
				"satoshi":  amount,
				"asset_id": asset,
			},
		},
		"utxos": outputs.Unspent,
	})
	defer free()

	var result any
	if err := withAuthHandler(C.GA_create_transaction(wallet.session, transactionDetails, &handler), handler, &result); err != nil {
		return "", err
	}

	params, free = toJson(result)
	if err := withAuthHandler(C.GA_blind_transaction(wallet.session, params, &handler), handler, &result); err != nil {
		return "", err
	}
	free()

	params, free = toJson(result)
	if err := withAuthHandler(C.GA_sign_transaction(wallet.session, params, &handler), handler, &result); err != nil {
		return "", err
	}
	free()

	params, free = toJson(result)
	var sendTx struct {
		TxHash string `json:"txhash"`
		Error  string `json:"error"`
	}
	if err := withAuthHandler(C.GA_send_transaction(wallet.session, params, &handler), handler, &sendTx); err != nil {
		return "", err
	}
	free()

	if sendTx.Error != "" {
		return "", errors.New(sendTx.Error)
	}
	return sendTx.TxHash, nil
}

func (wallet *Wallet) GetBlockHeight() (uint32, error) {
	// this is not perfect because it is zero until the first block is received, but there is no proper get_blockheight call
	if wallet.blockHeight == 0 {
		return 0, errors.New("block height not available yet. wait for first block")
	}
	return wallet.blockHeight, nil
}

func (wallet *Wallet) RegisterBlockListener(channel chan<- *onchain.BlockEpoch, stop <-chan bool) error {
	if !wallet.connected {
		return errors.New("wallet not connected")
	}
	for {
		select {
		case height, ok := <-wallet.blockHeightChannel:
			if !ok {
				return errors.New("wallet was removed")
			}
			channel <- &onchain.BlockEpoch{
				Height: height,
			}
		case <-stop:
			return nil
		}
	}
}

func (wallet *Wallet) Ready() bool {
	//return wallet.connected && wallet.subaccount != nil
	return wallet.connected
}

func (wallet *Wallet) GetWalletInfo() onchain.WalletInfo {
	return wallet.WalletInfo
}
