// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package etherswap

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// EtherSwapBatchClaimEntry is an auto generated low-level Go binding around an user-defined struct.
type EtherSwapBatchClaimEntry struct {
	Preimage      [32]byte
	Amount        *big.Int
	RefundAddress common.Address
	Timelock      *big.Int
	V             uint8
	R             [32]byte
	S             [32]byte
}

// EtherswapMetaData contains all meta data concerning the Etherswap contract.
var EtherswapMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"DOMAIN_SEPARATOR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_CLAIM\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_COMMIT\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_REFUND\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"VERSION\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"checkCommitmentSignature\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimBatch\",\"inputs\":[{\"name\":\"preimages\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"},{\"name\":\"refundAddresses\",\"type\":\"address[]\",\"internalType\":\"address[]\"},{\"name\":\"timelocks\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimBatch\",\"inputs\":[{\"name\":\"entries\",\"type\":\"tuple[]\",\"internalType\":\"structEtherSwap.BatchClaimEntry[]\",\"components\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hashValues\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"result\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"lock\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"lock\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"lockPrepayMinerfee\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"prepayAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refundCooperative\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refundCooperative\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"swaps\",\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"version\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"pure\"},{\"type\":\"event\",\"name\":\"Claim\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"preimage\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Lockup\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"CommitmentCannotBeClaimedAsSwap\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidPrepayAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidSignature\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapAlreadyExists\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapNotFound\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapNotTimedOut\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ZeroAmount\",\"inputs\":[]}]",
}

// EtherswapABI is the input ABI used to generate the binding from.
// Deprecated: Use EtherswapMetaData.ABI instead.
var EtherswapABI = EtherswapMetaData.ABI

// Etherswap is an auto generated Go binding around an Ethereum contract.
type Etherswap struct {
	EtherswapCaller     // Read-only binding to the contract
	EtherswapTransactor // Write-only binding to the contract
	EtherswapFilterer   // Log filterer for contract events
}

// EtherswapCaller is an auto generated read-only Go binding around an Ethereum contract.
type EtherswapCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EtherswapTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EtherswapTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EtherswapFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EtherswapFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EtherswapSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EtherswapSession struct {
	Contract     *Etherswap        // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EtherswapCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EtherswapCallerSession struct {
	Contract *EtherswapCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts    // Call options to use throughout this session
}

// EtherswapTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EtherswapTransactorSession struct {
	Contract     *EtherswapTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// EtherswapRaw is an auto generated low-level Go binding around an Ethereum contract.
type EtherswapRaw struct {
	Contract *Etherswap // Generic contract binding to access the raw methods on
}

// EtherswapCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EtherswapCallerRaw struct {
	Contract *EtherswapCaller // Generic read-only contract binding to access the raw methods on
}

// EtherswapTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EtherswapTransactorRaw struct {
	Contract *EtherswapTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEtherswap creates a new instance of Etherswap, bound to a specific deployed contract.
func NewEtherswap(address common.Address, backend bind.ContractBackend) (*Etherswap, error) {
	contract, err := bindEtherswap(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Etherswap{EtherswapCaller: EtherswapCaller{contract: contract}, EtherswapTransactor: EtherswapTransactor{contract: contract}, EtherswapFilterer: EtherswapFilterer{contract: contract}}, nil
}

// NewEtherswapCaller creates a new read-only instance of Etherswap, bound to a specific deployed contract.
func NewEtherswapCaller(address common.Address, caller bind.ContractCaller) (*EtherswapCaller, error) {
	contract, err := bindEtherswap(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EtherswapCaller{contract: contract}, nil
}

// NewEtherswapTransactor creates a new write-only instance of Etherswap, bound to a specific deployed contract.
func NewEtherswapTransactor(address common.Address, transactor bind.ContractTransactor) (*EtherswapTransactor, error) {
	contract, err := bindEtherswap(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EtherswapTransactor{contract: contract}, nil
}

// NewEtherswapFilterer creates a new log filterer instance of Etherswap, bound to a specific deployed contract.
func NewEtherswapFilterer(address common.Address, filterer bind.ContractFilterer) (*EtherswapFilterer, error) {
	contract, err := bindEtherswap(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EtherswapFilterer{contract: contract}, nil
}

// bindEtherswap binds a generic wrapper to an already deployed contract.
func bindEtherswap(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EtherswapMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Etherswap *EtherswapRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Etherswap.Contract.EtherswapCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Etherswap *EtherswapRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Etherswap.Contract.EtherswapTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Etherswap *EtherswapRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Etherswap.Contract.EtherswapTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Etherswap *EtherswapCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Etherswap.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Etherswap *EtherswapTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Etherswap.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Etherswap *EtherswapTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Etherswap.Contract.contract.Transact(opts, method, params...)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Etherswap *EtherswapCaller) DOMAINSEPARATOR(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "DOMAIN_SEPARATOR")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Etherswap *EtherswapSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _Etherswap.Contract.DOMAINSEPARATOR(&_Etherswap.CallOpts)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Etherswap *EtherswapCallerSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _Etherswap.Contract.DOMAINSEPARATOR(&_Etherswap.CallOpts)
}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Etherswap *EtherswapCaller) TYPEHASHCLAIM(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "TYPEHASH_CLAIM")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Etherswap *EtherswapSession) TYPEHASHCLAIM() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHCLAIM(&_Etherswap.CallOpts)
}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Etherswap *EtherswapCallerSession) TYPEHASHCLAIM() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHCLAIM(&_Etherswap.CallOpts)
}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Etherswap *EtherswapCaller) TYPEHASHCOMMIT(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "TYPEHASH_COMMIT")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Etherswap *EtherswapSession) TYPEHASHCOMMIT() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHCOMMIT(&_Etherswap.CallOpts)
}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Etherswap *EtherswapCallerSession) TYPEHASHCOMMIT() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHCOMMIT(&_Etherswap.CallOpts)
}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Etherswap *EtherswapCaller) TYPEHASHREFUND(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "TYPEHASH_REFUND")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Etherswap *EtherswapSession) TYPEHASHREFUND() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHREFUND(&_Etherswap.CallOpts)
}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Etherswap *EtherswapCallerSession) TYPEHASHREFUND() ([32]byte, error) {
	return _Etherswap.Contract.TYPEHASHREFUND(&_Etherswap.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Etherswap *EtherswapCaller) VERSION(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "VERSION")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Etherswap *EtherswapSession) VERSION() (uint8, error) {
	return _Etherswap.Contract.VERSION(&_Etherswap.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Etherswap *EtherswapCallerSession) VERSION() (uint8, error) {
	return _Etherswap.Contract.VERSION(&_Etherswap.CallOpts)
}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x0685d21e.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Etherswap *EtherswapCaller) CheckCommitmentSignature(opts *bind.CallOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "checkCommitmentSignature", preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x0685d21e.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Etherswap *EtherswapSession) CheckCommitmentSignature(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	return _Etherswap.Contract.CheckCommitmentSignature(&_Etherswap.CallOpts, preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x0685d21e.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Etherswap *EtherswapCallerSession) CheckCommitmentSignature(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	return _Etherswap.Contract.CheckCommitmentSignature(&_Etherswap.CallOpts, preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// HashValues is a free data retrieval call binding the contract method 0x8b2f8f82.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Etherswap *EtherswapCaller) HashValues(opts *bind.CallOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "hashValues", preimageHash, amount, claimAddress, refundAddress, timelock)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// HashValues is a free data retrieval call binding the contract method 0x8b2f8f82.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Etherswap *EtherswapSession) HashValues(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	return _Etherswap.Contract.HashValues(&_Etherswap.CallOpts, preimageHash, amount, claimAddress, refundAddress, timelock)
}

// HashValues is a free data retrieval call binding the contract method 0x8b2f8f82.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Etherswap *EtherswapCallerSession) HashValues(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	return _Etherswap.Contract.HashValues(&_Etherswap.CallOpts, preimageHash, amount, claimAddress, refundAddress, timelock)
}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Etherswap *EtherswapCaller) Swaps(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "swaps", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Etherswap *EtherswapSession) Swaps(arg0 [32]byte) (bool, error) {
	return _Etherswap.Contract.Swaps(&_Etherswap.CallOpts, arg0)
}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Etherswap *EtherswapCallerSession) Swaps(arg0 [32]byte) (bool, error) {
	return _Etherswap.Contract.Swaps(&_Etherswap.CallOpts, arg0)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Etherswap *EtherswapCaller) Version(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Etherswap.contract.Call(opts, &out, "version")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Etherswap *EtherswapSession) Version() (uint8, error) {
	return _Etherswap.Contract.Version(&_Etherswap.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Etherswap *EtherswapCallerSession) Version() (uint8, error) {
	return _Etherswap.Contract.Version(&_Etherswap.CallOpts)
}

// Claim is a paid mutator transaction binding the contract method 0x3648a807.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Etherswap *EtherswapTransactor) Claim(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claim", preimage, amount, refundAddress, timelock, v, r, s)
}

// Claim is a paid mutator transaction binding the contract method 0x3648a807.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Etherswap *EtherswapSession) Claim(preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim(&_Etherswap.TransactOpts, preimage, amount, refundAddress, timelock, v, r, s)
}

// Claim is a paid mutator transaction binding the contract method 0x3648a807.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Etherswap *EtherswapTransactorSession) Claim(preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim(&_Etherswap.TransactOpts, preimage, amount, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactor) Claim0(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claim0", preimage, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapSession) Claim0(preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim0(&_Etherswap.TransactOpts, preimage, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactorSession) Claim0(preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim0(&_Etherswap.TransactOpts, preimage, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim1 is a paid mutator transaction binding the contract method 0xc3c37fbc.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactor) Claim1(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claim1", preimage, amount, refundAddress, timelock)
}

// Claim1 is a paid mutator transaction binding the contract method 0xc3c37fbc.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapSession) Claim1(preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim1(&_Etherswap.TransactOpts, preimage, amount, refundAddress, timelock)
}

// Claim1 is a paid mutator transaction binding the contract method 0xc3c37fbc.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactorSession) Claim1(preimage [32]byte, amount *big.Int, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim1(&_Etherswap.TransactOpts, preimage, amount, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactor) Claim2(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claim2", preimage, amount, claimAddress, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapSession) Claim2(preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim2(&_Etherswap.TransactOpts, preimage, amount, claimAddress, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactorSession) Claim2(preimage [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Claim2(&_Etherswap.TransactOpts, preimage, amount, claimAddress, refundAddress, timelock)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0xc2c3a8c9.
//
// Solidity: function claimBatch(bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Etherswap *EtherswapTransactor) ClaimBatch(opts *bind.TransactOpts, preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claimBatch", preimages, amounts, refundAddresses, timelocks)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0xc2c3a8c9.
//
// Solidity: function claimBatch(bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Etherswap *EtherswapSession) ClaimBatch(preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.ClaimBatch(&_Etherswap.TransactOpts, preimages, amounts, refundAddresses, timelocks)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0xc2c3a8c9.
//
// Solidity: function claimBatch(bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Etherswap *EtherswapTransactorSession) ClaimBatch(preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.ClaimBatch(&_Etherswap.TransactOpts, preimages, amounts, refundAddresses, timelocks)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0xf3382d57.
//
// Solidity: function claimBatch((bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Etherswap *EtherswapTransactor) ClaimBatch0(opts *bind.TransactOpts, entries []EtherSwapBatchClaimEntry) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "claimBatch0", entries)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0xf3382d57.
//
// Solidity: function claimBatch((bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Etherswap *EtherswapSession) ClaimBatch0(entries []EtherSwapBatchClaimEntry) (*types.Transaction, error) {
	return _Etherswap.Contract.ClaimBatch0(&_Etherswap.TransactOpts, entries)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0xf3382d57.
//
// Solidity: function claimBatch((bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Etherswap *EtherswapTransactorSession) ClaimBatch0(entries []EtherSwapBatchClaimEntry) (*types.Transaction, error) {
	return _Etherswap.Contract.ClaimBatch0(&_Etherswap.TransactOpts, entries)
}

// Lock is a paid mutator transaction binding the contract method 0x0899146b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapTransactor) Lock(opts *bind.TransactOpts, preimageHash [32]byte, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "lock", preimageHash, claimAddress, timelock)
}

// Lock is a paid mutator transaction binding the contract method 0x0899146b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapSession) Lock(preimageHash [32]byte, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Lock(&_Etherswap.TransactOpts, preimageHash, claimAddress, timelock)
}

// Lock is a paid mutator transaction binding the contract method 0x0899146b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapTransactorSession) Lock(preimageHash [32]byte, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Lock(&_Etherswap.TransactOpts, preimageHash, claimAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0x799f212b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, address refundAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapTransactor) Lock0(opts *bind.TransactOpts, preimageHash [32]byte, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "lock0", preimageHash, claimAddress, refundAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0x799f212b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, address refundAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapSession) Lock0(preimageHash [32]byte, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Lock0(&_Etherswap.TransactOpts, preimageHash, claimAddress, refundAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0x799f212b.
//
// Solidity: function lock(bytes32 preimageHash, address claimAddress, address refundAddress, uint256 timelock) payable returns()
func (_Etherswap *EtherswapTransactorSession) Lock0(preimageHash [32]byte, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Lock0(&_Etherswap.TransactOpts, preimageHash, claimAddress, refundAddress, timelock)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0x6fa4ae60.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, address claimAddress, uint256 timelock, uint256 prepayAmount) payable returns()
func (_Etherswap *EtherswapTransactor) LockPrepayMinerfee(opts *bind.TransactOpts, preimageHash [32]byte, claimAddress common.Address, timelock *big.Int, prepayAmount *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "lockPrepayMinerfee", preimageHash, claimAddress, timelock, prepayAmount)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0x6fa4ae60.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, address claimAddress, uint256 timelock, uint256 prepayAmount) payable returns()
func (_Etherswap *EtherswapSession) LockPrepayMinerfee(preimageHash [32]byte, claimAddress common.Address, timelock *big.Int, prepayAmount *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.LockPrepayMinerfee(&_Etherswap.TransactOpts, preimageHash, claimAddress, timelock, prepayAmount)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0x6fa4ae60.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, address claimAddress, uint256 timelock, uint256 prepayAmount) payable returns()
func (_Etherswap *EtherswapTransactorSession) LockPrepayMinerfee(preimageHash [32]byte, claimAddress common.Address, timelock *big.Int, prepayAmount *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.LockPrepayMinerfee(&_Etherswap.TransactOpts, preimageHash, claimAddress, timelock, prepayAmount)
}

// Refund is a paid mutator transaction binding the contract method 0x35cd4ccb.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactor) Refund(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "refund", preimageHash, amount, claimAddress, timelock)
}

// Refund is a paid mutator transaction binding the contract method 0x35cd4ccb.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapSession) Refund(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Refund(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, timelock)
}

// Refund is a paid mutator transaction binding the contract method 0x35cd4ccb.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactorSession) Refund(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Refund(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactor) Refund0(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "refund0", preimageHash, amount, claimAddress, refundAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapSession) Refund0(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Refund0(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, refundAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Etherswap *EtherswapTransactorSession) Refund0(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Etherswap.Contract.Refund0(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, refundAddress, timelock)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactor) RefundCooperative(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "refundCooperative", preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapSession) RefundCooperative(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.RefundCooperative(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactorSession) RefundCooperative(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.RefundCooperative(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfe237d45.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactor) RefundCooperative0(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.contract.Transact(opts, "refundCooperative0", preimageHash, amount, claimAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfe237d45.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapSession) RefundCooperative0(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.RefundCooperative0(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfe237d45.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Etherswap *EtherswapTransactorSession) RefundCooperative0(preimageHash [32]byte, amount *big.Int, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Etherswap.Contract.RefundCooperative0(&_Etherswap.TransactOpts, preimageHash, amount, claimAddress, timelock, v, r, s)
}

// EtherswapClaimIterator is returned from FilterClaim and is used to iterate over the raw logs and unpacked data for Claim events raised by the Etherswap contract.
type EtherswapClaimIterator struct {
	Event *EtherswapClaim // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EtherswapClaimIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EtherswapClaim)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EtherswapClaim)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EtherswapClaimIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EtherswapClaimIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EtherswapClaim represents a Claim event raised by the Etherswap contract.
type EtherswapClaim struct {
	PreimageHash [32]byte
	Preimage     [32]byte
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterClaim is a free log retrieval operation binding the contract event 0x5664142af3dcfc3dc3de45a43f75c746bd1d8c11170a5037fdf98bdb35775137.
//
// Solidity: event Claim(bytes32 indexed preimageHash, bytes32 preimage)
func (_Etherswap *EtherswapFilterer) FilterClaim(opts *bind.FilterOpts, preimageHash [][32]byte) (*EtherswapClaimIterator, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Etherswap.contract.FilterLogs(opts, "Claim", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return &EtherswapClaimIterator{contract: _Etherswap.contract, event: "Claim", logs: logs, sub: sub}, nil
}

// WatchClaim is a free log subscription operation binding the contract event 0x5664142af3dcfc3dc3de45a43f75c746bd1d8c11170a5037fdf98bdb35775137.
//
// Solidity: event Claim(bytes32 indexed preimageHash, bytes32 preimage)
func (_Etherswap *EtherswapFilterer) WatchClaim(opts *bind.WatchOpts, sink chan<- *EtherswapClaim, preimageHash [][32]byte) (event.Subscription, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Etherswap.contract.WatchLogs(opts, "Claim", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EtherswapClaim)
				if err := _Etherswap.contract.UnpackLog(event, "Claim", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseClaim is a log parse operation binding the contract event 0x5664142af3dcfc3dc3de45a43f75c746bd1d8c11170a5037fdf98bdb35775137.
//
// Solidity: event Claim(bytes32 indexed preimageHash, bytes32 preimage)
func (_Etherswap *EtherswapFilterer) ParseClaim(log types.Log) (*EtherswapClaim, error) {
	event := new(EtherswapClaim)
	if err := _Etherswap.contract.UnpackLog(event, "Claim", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EtherswapLockupIterator is returned from FilterLockup and is used to iterate over the raw logs and unpacked data for Lockup events raised by the Etherswap contract.
type EtherswapLockupIterator struct {
	Event *EtherswapLockup // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EtherswapLockupIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EtherswapLockup)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EtherswapLockup)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EtherswapLockupIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EtherswapLockupIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EtherswapLockup represents a Lockup event raised by the Etherswap contract.
type EtherswapLockup struct {
	PreimageHash  [32]byte
	Amount        *big.Int
	ClaimAddress  common.Address
	RefundAddress common.Address
	Timelock      *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterLockup is a free log retrieval operation binding the contract event 0x15b4b8206809535e547317cd5cedc86cff6e7d203551f93701786ddaf14fd9f9.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Etherswap *EtherswapFilterer) FilterLockup(opts *bind.FilterOpts, preimageHash [][32]byte, claimAddress []common.Address, refundAddress []common.Address) (*EtherswapLockupIterator, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	var claimAddressRule []interface{}
	for _, claimAddressItem := range claimAddress {
		claimAddressRule = append(claimAddressRule, claimAddressItem)
	}
	var refundAddressRule []interface{}
	for _, refundAddressItem := range refundAddress {
		refundAddressRule = append(refundAddressRule, refundAddressItem)
	}

	logs, sub, err := _Etherswap.contract.FilterLogs(opts, "Lockup", preimageHashRule, claimAddressRule, refundAddressRule)
	if err != nil {
		return nil, err
	}
	return &EtherswapLockupIterator{contract: _Etherswap.contract, event: "Lockup", logs: logs, sub: sub}, nil
}

// WatchLockup is a free log subscription operation binding the contract event 0x15b4b8206809535e547317cd5cedc86cff6e7d203551f93701786ddaf14fd9f9.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Etherswap *EtherswapFilterer) WatchLockup(opts *bind.WatchOpts, sink chan<- *EtherswapLockup, preimageHash [][32]byte, claimAddress []common.Address, refundAddress []common.Address) (event.Subscription, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	var claimAddressRule []interface{}
	for _, claimAddressItem := range claimAddress {
		claimAddressRule = append(claimAddressRule, claimAddressItem)
	}
	var refundAddressRule []interface{}
	for _, refundAddressItem := range refundAddress {
		refundAddressRule = append(refundAddressRule, refundAddressItem)
	}

	logs, sub, err := _Etherswap.contract.WatchLogs(opts, "Lockup", preimageHashRule, claimAddressRule, refundAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EtherswapLockup)
				if err := _Etherswap.contract.UnpackLog(event, "Lockup", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLockup is a log parse operation binding the contract event 0x15b4b8206809535e547317cd5cedc86cff6e7d203551f93701786ddaf14fd9f9.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Etherswap *EtherswapFilterer) ParseLockup(log types.Log) (*EtherswapLockup, error) {
	event := new(EtherswapLockup)
	if err := _Etherswap.contract.UnpackLog(event, "Lockup", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EtherswapRefundIterator is returned from FilterRefund and is used to iterate over the raw logs and unpacked data for Refund events raised by the Etherswap contract.
type EtherswapRefundIterator struct {
	Event *EtherswapRefund // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EtherswapRefundIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EtherswapRefund)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EtherswapRefund)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EtherswapRefundIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EtherswapRefundIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EtherswapRefund represents a Refund event raised by the Etherswap contract.
type EtherswapRefund struct {
	PreimageHash [32]byte
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRefund is a free log retrieval operation binding the contract event 0x3fbd469ec3a5ce074f975f76ce27e727ba21c99176917b97ae2e713695582a12.
//
// Solidity: event Refund(bytes32 indexed preimageHash)
func (_Etherswap *EtherswapFilterer) FilterRefund(opts *bind.FilterOpts, preimageHash [][32]byte) (*EtherswapRefundIterator, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Etherswap.contract.FilterLogs(opts, "Refund", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return &EtherswapRefundIterator{contract: _Etherswap.contract, event: "Refund", logs: logs, sub: sub}, nil
}

// WatchRefund is a free log subscription operation binding the contract event 0x3fbd469ec3a5ce074f975f76ce27e727ba21c99176917b97ae2e713695582a12.
//
// Solidity: event Refund(bytes32 indexed preimageHash)
func (_Etherswap *EtherswapFilterer) WatchRefund(opts *bind.WatchOpts, sink chan<- *EtherswapRefund, preimageHash [][32]byte) (event.Subscription, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Etherswap.contract.WatchLogs(opts, "Refund", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EtherswapRefund)
				if err := _Etherswap.contract.UnpackLog(event, "Refund", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRefund is a log parse operation binding the contract event 0x3fbd469ec3a5ce074f975f76ce27e727ba21c99176917b97ae2e713695582a12.
//
// Solidity: event Refund(bytes32 indexed preimageHash)
func (_Etherswap *EtherswapFilterer) ParseRefund(log types.Log) (*EtherswapRefund, error) {
	event := new(EtherswapRefund)
	if err := _Etherswap.contract.UnpackLog(event, "Refund", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
