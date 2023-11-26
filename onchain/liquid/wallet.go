package liquid

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
	"os"
	"path"
	"time"
	"unsafe"

	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"

	"github.com/BoltzExchange/boltz-client/boltz"
)

type AuthHandler = *C.struct_GA_auth_handler
type Json = *C.GA_json
type Session = *C.struct_GA_session

type backup struct {
	Mnemonic   string `json:"mnemonic"`
	Subaccount uint64 `json:"subaccount"`
}

type Subaccount struct {
	Pointer uint64 `json:"pointer"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Used    bool   `json:"bip44_discovered"`
}

type Wallet struct {
	Network            *boltz.Network
	walletDir          string
	subaccount         uint64
	session            Session
	connected          bool
	blockHeight        uint32
	blockHeightChannel chan uint32
}

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
	defer C.GA_destroy_auth_handler(handler)
	var output Json

	if err := toErr(ret); err != nil {
		return err
	}
	var handlerStatus struct {
		Status string `json:"status"`
		Result R      `json:"result"`
		Error  string `json:"error"`
	}
	if err := withOutput(C.GA_auth_handler_get_status(handler, &output), output, &handlerStatus); err != nil {
		return err
	}

	if handlerStatus.Status == "error" {
		return errors.New(handlerStatus.Error)
	}
	if result != nil {
		*result = handlerStatus.Result
	}
	return nil
}

func InitWallet(dataDir string, network *boltz.Network, debug bool) (*Wallet, error) {
	wallet := &Wallet{
		Network:   network,
		walletDir: path.Join(dataDir + "/liquid-wallet"),
	}

	params := map[string]any{
		"datadir":   wallet.walletDir,
		"log_level": "error",
	}
	if debug {
		params["log_level"] = "debug"
	}
	paramsJson, free := toJson(params)
	defer free()
	if err := toErr(C.GA_init(paramsJson)); err != nil {
		return nil, err
	}

	return wallet, nil
}

func (wallet *Wallet) Exists() bool {
	menmonic, err := wallet.GetMnemonic()
	return err == nil && menmonic != ""
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

	var networkName string
	if wallet.Network == boltz.MainNet {
		networkName = "electrum-liquid"
	} else if wallet.Network == boltz.TestNet {
		networkName = "electrum-testnet-liquid"
	} else if wallet.Network == boltz.Regtest {
		networkName = "electrum-localtest-liquid"
	} else {
		return errors.New("unknown network")
	}
	params, free := toJson(map[string]any{
		"name": networkName,
	})
	defer free()

	if err := toErr(C.GA_connect(wallet.session, params)); err != nil {
		return err
	}

	wallet.connected = true

	return nil
}

func (wallet *Wallet) Register() (string, error) {
	if err := wallet.Connect(); err != nil {
		return "", err
	}
	buffer := C.CString("")
	defer C.free(unsafe.Pointer(buffer))
	if err := toErr(C.GA_generate_mnemonic(&buffer)); err != nil {
		return "", errors.New("failed to generate mnemonic: " + err.Error())
	}
	mnemonic := C.GoString(buffer)

	login, freeLogin := toJson(map[string]string{"mnemonic": mnemonic})
	defer freeLogin()
	hwDevice, freeDevice := toJson(nil)
	defer freeDevice()

	var handler AuthHandler
	var result any
	err := withAuthHandler(C.GA_register_user(wallet.session, hwDevice, login, &handler), handler, &result)
	if err != nil {
		return "", err
	}

	return mnemonic, wallet.ImportMnemonic(mnemonic)
}

func (wallet *Wallet) ImportMnemonic(mnemonic string) error {
	err := wallet.login(mnemonic)
	if err != nil {
		return err
	}

	return wallet.backup(backup{
		Mnemonic: mnemonic,
	})
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
			if subaccount.Used {
				var result struct {
					Transactions []any `json:"transactions"`
				}
				timeout := time.After(5 * time.Second)
				ticker := time.NewTicker(200 * time.Millisecond)
				defer ticker.Stop()
				for len(result.Transactions) == 0 {
					select {
					case <-ticker.C:
						details, free = toJson(map[string]any{"subaccount": subaccount.Pointer, "first": 0, "count": 1})
						err := withAuthHandler(C.GA_get_transactions(wallet.session, details, &handler), handler, &result)
						free()
						if err != nil {
							return nil, fmt.Errorf("could not get transactions for subaccount %d: %w", subaccount.Pointer, err)
						}
					case <-timeout:
						return nil, fmt.Errorf("timed out waiting for subaccount %d to sync", subaccount.Pointer)
					}
				}
			}
		}
	}
	return result.Subaccounts, nil
}

func (wallet *Wallet) SetSubaccount(subaccount *uint64) error {
	accounts, err := wallet.GetSubaccounts(false)
	if err != nil {
		return err
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
				return err
			}

			subaccount = &result.Pointer
		}
	} else {
		var result any
		err := withAuthHandler(C.GA_get_subaccount(wallet.session, C.uint32_t(*subaccount), &handler), handler, &result)
		if err != nil {
			return err
		}
	}
	backup, err := wallet.getBackup()
	if err != nil {
		return errors.New("could not read current backup: " + err.Error())
	}
	backup.Subaccount = *subaccount
	return wallet.backup(backup)
}

func (wallet *Wallet) GetMnemonic() (string, error) {
	backup, err := wallet.getBackup()
	if err != nil {
		return "", errors.New("could not read mnemonic: " + err.Error())
	}
	return backup.Mnemonic, nil
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

func (wallet *Wallet) CurrentSubaccount() uint64 {
	return wallet.subaccount
}

func (wallet *Wallet) getBackup() (backup backup, err error) {
	raw, err := os.ReadFile(wallet.mnemonicBackupFile())
	if err != nil {
		return backup, errors.New("failed to read mnemonic backup: " + err.Error())
	}
	err = json.Unmarshal(raw, &backup)
	return backup, err
}

func (wallet *Wallet) backup(backup backup) error {
	wallet.subaccount = backup.Subaccount
	encoded, err := json.Marshal(backup)
	if err != nil {
		return errors.New("failed to encode backup: " + err.Error())
	}
	if err := os.WriteFile(wallet.mnemonicBackupFile(), encoded, 0644); err != nil {
		return errors.New("failed to write mnemonic backup: " + err.Error())
	}
	return nil
}

func (wallet *Wallet) Login() error {
	if err := wallet.Connect(); err != nil {
		return err
	}
	backup, err := wallet.getBackup()
	if err != nil {
		return errors.New("could not read mnemonic: " + err.Error())
	}

	wallet.subaccount = backup.Subaccount
	return wallet.login(backup.Mnemonic)
}

func (wallet *Wallet) login(mnemonic string) error {
	if err := wallet.Connect(); err != nil {
		return err
	}

	login, freeLogin := toJson(map[string]string{"mnemonic": mnemonic})
	defer freeLogin()
	hwDevice, freeDevice := toJson(nil)
	defer freeDevice()

	var handler AuthHandler
	var result any
	if err := withAuthHandler(C.GA_login_user(wallet.session, hwDevice, login, &handler), handler, &result); err != nil {
		return err
	}
	logger.Debugf("Logged in: %v", result)
	return nil
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

	if err := os.Remove(wallet.mnemonicBackupFile()); err != nil {
		return errors.New("failed to remove mnemonic backup: " + err.Error())
	}
	return nil
}

func (wallet *Wallet) NewAddress() (string, error) {
	params, free := toJson(map[string]any{
		"subaccount": wallet.subaccount,
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

type Balance struct {
	Total       uint64
	Confirmed   uint64
	Unconfirmed uint64
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

func (wallet *Wallet) GetSubaccountBalance(subaccount uint64) (balance Balance, err error) {
	balance.Total, err = wallet.getSubaccountBalance(subaccount, true)
	if err != nil {
		return balance, err
	}
	balance.Confirmed, err = wallet.getSubaccountBalance(subaccount, false)
	if err != nil {
		return balance, err
	}
	balance.Unconfirmed = balance.Total - balance.Confirmed
	return balance, err
}

func (wallet *Wallet) GetBalance() (Balance, error) {
	return wallet.GetSubaccountBalance(wallet.subaccount)
}

func (wallet *Wallet) SendToAddress(address string, amount uint64, satPerVbyte float64) (string, error) {
	params, free := toJson(map[string]any{
		"subaccount": wallet.subaccount,
		"num_confs":  0,
	})
	defer free()
	var handler AuthHandler

	var outputs struct {
		Unspent any `json:"unspent_outputs"`
	}
	if err := withAuthHandler(C.GA_get_unspent_outputs(wallet.session, params, &handler), handler, &outputs); err != nil {
		return "", err
	}

	transactionDetails, free := toJson(map[string]any{
		"fee_rate": satPerVbyte * 1000,
		"addressees": []map[string]any{
			{
				"address":  address,
				"satoshi":  amount,
				"asset_id": wallet.Network.Liquid.AssetID,
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

func (wallet *Wallet) EstimateFee(confTarget int32) (float64, error) {
	var output Json
	var estimates struct {
		Fees []float64 `json:"fees"`
	}
	if err := withOutput(C.GA_get_fee_estimates(wallet.session, &output), output, &estimates); err != nil {
		return 0, err
	}
	if confTarget > 24 {
		confTarget = 24
	}
	return estimates.Fees[confTarget] / 1000, nil
}

func (wallet *Wallet) GetBlockHeight() (uint32, error) {
	// this is not perfect because it is zero until the first block is received, but there is no proper get_blockheight call
	if wallet.blockHeight == 0 {
		return 0, errors.New("block height not available yet. wait for first block")
	}
	return wallet.blockHeight, nil
}

func (wallet *Wallet) RegisterBlockListener(channel chan *onchain.BlockEpoch) error {
	if !wallet.connected {
		return errors.New("wallet not connected")

	}
	for height := range wallet.blockHeightChannel {
		channel <- &onchain.BlockEpoch{
			Height: height,
		}
	}
	return errors.New("wallet was removed")
}

func (wallet *Wallet) Ready() bool {
	return wallet.Exists()
}

func (wallet *Wallet) mnemonicBackupFile() string {
	return path.Join(wallet.walletDir, "mnemonic.json")
}
