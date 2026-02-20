// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package erc20swap

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

// ERC20SwapBatchClaimEntry is an auto generated low-level Go binding around an user-defined struct.
type ERC20SwapBatchClaimEntry struct {
	Preimage      [32]byte
	Amount        *big.Int
	RefundAddress common.Address
	Timelock      *big.Int
	V             uint8
	R             [32]byte
	S             [32]byte
}

// Erc20swapMetaData contains all meta data concerning the Erc20swap contract.
var Erc20swapMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"DOMAIN_SEPARATOR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_CLAIM\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_COMMIT\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"TYPEHASH_REFUND\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"VERSION\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"checkCommitmentSignature\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimBatch\",\"inputs\":[{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"entries\",\"type\":\"tuple[]\",\"internalType\":\"structERC20Swap.BatchClaimEntry[]\",\"components\":[{\"name\":\"preimage\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimBatch\",\"inputs\":[{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"preimages\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"},{\"name\":\"refundAddresses\",\"type\":\"address[]\",\"internalType\":\"address[]\"},{\"name\":\"timelocks\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hashValues\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"result\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"lock\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"lock\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"lockPrepayMinerfee\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refundCooperative\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"refundCooperative\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"v\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"r\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"s\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"swaps\",\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"version\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"pure\"},{\"type\":\"event\",\"name\":\"Claim\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"preimage\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Lockup\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"tokenAddress\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"claimAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"refundAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"timelock\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Refund\",\"inputs\":[{\"name\":\"preimageHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"CommitmentCannotBeClaimedAsSwap\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidSignature\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapAlreadyExists\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapNotFound\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SwapNotTimedOut\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ZeroAmount\",\"inputs\":[]}]",
}

// Erc20swapABI is the input ABI used to generate the binding from.
// Deprecated: Use Erc20swapMetaData.ABI instead.
var Erc20swapABI = Erc20swapMetaData.ABI

// Erc20swap is an auto generated Go binding around an Ethereum contract.
type Erc20swap struct {
	Erc20swapCaller     // Read-only binding to the contract
	Erc20swapTransactor // Write-only binding to the contract
	Erc20swapFilterer   // Log filterer for contract events
}

// Erc20swapCaller is an auto generated read-only Go binding around an Ethereum contract.
type Erc20swapCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Erc20swapTransactor is an auto generated write-only Go binding around an Ethereum contract.
type Erc20swapTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Erc20swapFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type Erc20swapFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Erc20swapSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type Erc20swapSession struct {
	Contract     *Erc20swap        // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// Erc20swapCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type Erc20swapCallerSession struct {
	Contract *Erc20swapCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts    // Call options to use throughout this session
}

// Erc20swapTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type Erc20swapTransactorSession struct {
	Contract     *Erc20swapTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// Erc20swapRaw is an auto generated low-level Go binding around an Ethereum contract.
type Erc20swapRaw struct {
	Contract *Erc20swap // Generic contract binding to access the raw methods on
}

// Erc20swapCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type Erc20swapCallerRaw struct {
	Contract *Erc20swapCaller // Generic read-only contract binding to access the raw methods on
}

// Erc20swapTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type Erc20swapTransactorRaw struct {
	Contract *Erc20swapTransactor // Generic write-only contract binding to access the raw methods on
}

// NewErc20swap creates a new instance of Erc20swap, bound to a specific deployed contract.
func NewErc20swap(address common.Address, backend bind.ContractBackend) (*Erc20swap, error) {
	contract, err := bindErc20swap(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Erc20swap{Erc20swapCaller: Erc20swapCaller{contract: contract}, Erc20swapTransactor: Erc20swapTransactor{contract: contract}, Erc20swapFilterer: Erc20swapFilterer{contract: contract}}, nil
}

// NewErc20swapCaller creates a new read-only instance of Erc20swap, bound to a specific deployed contract.
func NewErc20swapCaller(address common.Address, caller bind.ContractCaller) (*Erc20swapCaller, error) {
	contract, err := bindErc20swap(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Erc20swapCaller{contract: contract}, nil
}

// NewErc20swapTransactor creates a new write-only instance of Erc20swap, bound to a specific deployed contract.
func NewErc20swapTransactor(address common.Address, transactor bind.ContractTransactor) (*Erc20swapTransactor, error) {
	contract, err := bindErc20swap(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &Erc20swapTransactor{contract: contract}, nil
}

// NewErc20swapFilterer creates a new log filterer instance of Erc20swap, bound to a specific deployed contract.
func NewErc20swapFilterer(address common.Address, filterer bind.ContractFilterer) (*Erc20swapFilterer, error) {
	contract, err := bindErc20swap(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &Erc20swapFilterer{contract: contract}, nil
}

// bindErc20swap binds a generic wrapper to an already deployed contract.
func bindErc20swap(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := Erc20swapMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Erc20swap *Erc20swapRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Erc20swap.Contract.Erc20swapCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Erc20swap *Erc20swapRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Erc20swap.Contract.Erc20swapTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Erc20swap *Erc20swapRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Erc20swap.Contract.Erc20swapTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Erc20swap *Erc20swapCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Erc20swap.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Erc20swap *Erc20swapTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Erc20swap.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Erc20swap *Erc20swapTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Erc20swap.Contract.contract.Transact(opts, method, params...)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Erc20swap *Erc20swapCaller) DOMAINSEPARATOR(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "DOMAIN_SEPARATOR")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Erc20swap *Erc20swapSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _Erc20swap.Contract.DOMAINSEPARATOR(&_Erc20swap.CallOpts)
}

// DOMAINSEPARATOR is a free data retrieval call binding the contract method 0x3644e515.
//
// Solidity: function DOMAIN_SEPARATOR() view returns(bytes32)
func (_Erc20swap *Erc20swapCallerSession) DOMAINSEPARATOR() ([32]byte, error) {
	return _Erc20swap.Contract.DOMAINSEPARATOR(&_Erc20swap.CallOpts)
}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Erc20swap *Erc20swapCaller) TYPEHASHCLAIM(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "TYPEHASH_CLAIM")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Erc20swap *Erc20swapSession) TYPEHASHCLAIM() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHCLAIM(&_Erc20swap.CallOpts)
}

// TYPEHASHCLAIM is a free data retrieval call binding the contract method 0xebb7af92.
//
// Solidity: function TYPEHASH_CLAIM() view returns(bytes32)
func (_Erc20swap *Erc20swapCallerSession) TYPEHASHCLAIM() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHCLAIM(&_Erc20swap.CallOpts)
}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Erc20swap *Erc20swapCaller) TYPEHASHCOMMIT(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "TYPEHASH_COMMIT")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Erc20swap *Erc20swapSession) TYPEHASHCOMMIT() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHCOMMIT(&_Erc20swap.CallOpts)
}

// TYPEHASHCOMMIT is a free data retrieval call binding the contract method 0x5073c277.
//
// Solidity: function TYPEHASH_COMMIT() view returns(bytes32)
func (_Erc20swap *Erc20swapCallerSession) TYPEHASHCOMMIT() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHCOMMIT(&_Erc20swap.CallOpts)
}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Erc20swap *Erc20swapCaller) TYPEHASHREFUND(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "TYPEHASH_REFUND")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Erc20swap *Erc20swapSession) TYPEHASHREFUND() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHREFUND(&_Erc20swap.CallOpts)
}

// TYPEHASHREFUND is a free data retrieval call binding the contract method 0xa9ab4d5b.
//
// Solidity: function TYPEHASH_REFUND() view returns(bytes32)
func (_Erc20swap *Erc20swapCallerSession) TYPEHASHREFUND() ([32]byte, error) {
	return _Erc20swap.Contract.TYPEHASHREFUND(&_Erc20swap.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Erc20swap *Erc20swapCaller) VERSION(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "VERSION")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Erc20swap *Erc20swapSession) VERSION() (uint8, error) {
	return _Erc20swap.Contract.VERSION(&_Erc20swap.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(uint8)
func (_Erc20swap *Erc20swapCallerSession) VERSION() (uint8, error) {
	return _Erc20swap.Contract.VERSION(&_Erc20swap.CallOpts)
}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x627b8bb7.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Erc20swap *Erc20swapCaller) CheckCommitmentSignature(opts *bind.CallOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "checkCommitmentSignature", preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x627b8bb7.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Erc20swap *Erc20swapSession) CheckCommitmentSignature(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	return _Erc20swap.Contract.CheckCommitmentSignature(&_Erc20swap.CallOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// CheckCommitmentSignature is a free data retrieval call binding the contract method 0x627b8bb7.
//
// Solidity: function checkCommitmentSignature(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) view returns(bool)
func (_Erc20swap *Erc20swapCallerSession) CheckCommitmentSignature(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (bool, error) {
	return _Erc20swap.Contract.CheckCommitmentSignature(&_Erc20swap.CallOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// HashValues is a free data retrieval call binding the contract method 0x7beb9d6d.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Erc20swap *Erc20swapCaller) HashValues(opts *bind.CallOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "hashValues", preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// HashValues is a free data retrieval call binding the contract method 0x7beb9d6d.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Erc20swap *Erc20swapSession) HashValues(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	return _Erc20swap.Contract.HashValues(&_Erc20swap.CallOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// HashValues is a free data retrieval call binding the contract method 0x7beb9d6d.
//
// Solidity: function hashValues(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) pure returns(bytes32 result)
func (_Erc20swap *Erc20swapCallerSession) HashValues(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) ([32]byte, error) {
	return _Erc20swap.Contract.HashValues(&_Erc20swap.CallOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Erc20swap *Erc20swapCaller) Swaps(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "swaps", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Erc20swap *Erc20swapSession) Swaps(arg0 [32]byte) (bool, error) {
	return _Erc20swap.Contract.Swaps(&_Erc20swap.CallOpts, arg0)
}

// Swaps is a free data retrieval call binding the contract method 0xeb84e7f2.
//
// Solidity: function swaps(bytes32 ) view returns(bool)
func (_Erc20swap *Erc20swapCallerSession) Swaps(arg0 [32]byte) (bool, error) {
	return _Erc20swap.Contract.Swaps(&_Erc20swap.CallOpts, arg0)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Erc20swap *Erc20swapCaller) Version(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Erc20swap.contract.Call(opts, &out, "version")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Erc20swap *Erc20swapSession) Version() (uint8, error) {
	return _Erc20swap.Contract.Version(&_Erc20swap.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() pure returns(uint8)
func (_Erc20swap *Erc20swapCallerSession) Version() (uint8, error) {
	return _Erc20swap.Contract.Version(&_Erc20swap.CallOpts)
}

// Claim is a paid mutator transaction binding the contract method 0x107e1bb3.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactor) Claim(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claim", preimage, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim is a paid mutator transaction binding the contract method 0x107e1bb3.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapSession) Claim(preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim is a paid mutator transaction binding the contract method 0x107e1bb3.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactorSession) Claim(preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Erc20swap *Erc20swapTransactor) Claim0(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claim0", preimage, amount, tokenAddress, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Erc20swap *Erc20swapSession) Claim0(preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim0(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, refundAddress, timelock, v, r, s)
}

// Claim0 is a paid mutator transaction binding the contract method 0xb2b78df8.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns(address)
func (_Erc20swap *Erc20swapTransactorSession) Claim0(preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim0(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, refundAddress, timelock, v, r, s)
}

// Claim1 is a paid mutator transaction binding the contract method 0xbc586b28.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Claim1(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claim1", preimage, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Claim1 is a paid mutator transaction binding the contract method 0xbc586b28.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Claim1(preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim1(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Claim1 is a paid mutator transaction binding the contract method 0xbc586b28.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Claim1(preimage [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim1(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Claim2(opts *bind.TransactOpts, preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claim2", preimage, amount, tokenAddress, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Claim2(preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim2(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, refundAddress, timelock)
}

// Claim2 is a paid mutator transaction binding the contract method 0xcd413efa.
//
// Solidity: function claim(bytes32 preimage, uint256 amount, address tokenAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Claim2(preimage [32]byte, amount *big.Int, tokenAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Claim2(&_Erc20swap.TransactOpts, preimage, amount, tokenAddress, refundAddress, timelock)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0x41bc6370.
//
// Solidity: function claimBatch(address tokenAddress, (bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Erc20swap *Erc20swapTransactor) ClaimBatch(opts *bind.TransactOpts, tokenAddress common.Address, entries []ERC20SwapBatchClaimEntry) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claimBatch", tokenAddress, entries)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0x41bc6370.
//
// Solidity: function claimBatch(address tokenAddress, (bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Erc20swap *Erc20swapSession) ClaimBatch(tokenAddress common.Address, entries []ERC20SwapBatchClaimEntry) (*types.Transaction, error) {
	return _Erc20swap.Contract.ClaimBatch(&_Erc20swap.TransactOpts, tokenAddress, entries)
}

// ClaimBatch is a paid mutator transaction binding the contract method 0x41bc6370.
//
// Solidity: function claimBatch(address tokenAddress, (bytes32,uint256,address,uint256,uint8,bytes32,bytes32)[] entries) returns()
func (_Erc20swap *Erc20swapTransactorSession) ClaimBatch(tokenAddress common.Address, entries []ERC20SwapBatchClaimEntry) (*types.Transaction, error) {
	return _Erc20swap.Contract.ClaimBatch(&_Erc20swap.TransactOpts, tokenAddress, entries)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0x8579dc5f.
//
// Solidity: function claimBatch(address tokenAddress, bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Erc20swap *Erc20swapTransactor) ClaimBatch0(opts *bind.TransactOpts, tokenAddress common.Address, preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "claimBatch0", tokenAddress, preimages, amounts, refundAddresses, timelocks)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0x8579dc5f.
//
// Solidity: function claimBatch(address tokenAddress, bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Erc20swap *Erc20swapSession) ClaimBatch0(tokenAddress common.Address, preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.ClaimBatch0(&_Erc20swap.TransactOpts, tokenAddress, preimages, amounts, refundAddresses, timelocks)
}

// ClaimBatch0 is a paid mutator transaction binding the contract method 0x8579dc5f.
//
// Solidity: function claimBatch(address tokenAddress, bytes32[] preimages, uint256[] amounts, address[] refundAddresses, uint256[] timelocks) returns()
func (_Erc20swap *Erc20swapTransactorSession) ClaimBatch0(tokenAddress common.Address, preimages [][32]byte, amounts []*big.Int, refundAddresses []common.Address, timelocks []*big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.ClaimBatch0(&_Erc20swap.TransactOpts, tokenAddress, preimages, amounts, refundAddresses, timelocks)
}

// Lock is a paid mutator transaction binding the contract method 0x91644b2b.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Lock(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "lock", preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Lock is a paid mutator transaction binding the contract method 0x91644b2b.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Lock(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Lock(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Lock is a paid mutator transaction binding the contract method 0x91644b2b.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Lock(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Lock(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0xe64fafcc.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Lock0(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "lock0", preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0xe64fafcc.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Lock0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Lock0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Lock0 is a paid mutator transaction binding the contract method 0xe64fafcc.
//
// Solidity: function lock(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Lock0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Lock0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0xb8080ab8.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) payable returns()
func (_Erc20swap *Erc20swapTransactor) LockPrepayMinerfee(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "lockPrepayMinerfee", preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0xb8080ab8.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) payable returns()
func (_Erc20swap *Erc20swapSession) LockPrepayMinerfee(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.LockPrepayMinerfee(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// LockPrepayMinerfee is a paid mutator transaction binding the contract method 0xb8080ab8.
//
// Solidity: function lockPrepayMinerfee(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) payable returns()
func (_Erc20swap *Erc20swapTransactorSession) LockPrepayMinerfee(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.LockPrepayMinerfee(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Refund is a paid mutator transaction binding the contract method 0x0e5bbd59.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Refund(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "refund", preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Refund is a paid mutator transaction binding the contract method 0x0e5bbd59.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Refund(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Refund(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Refund is a paid mutator transaction binding the contract method 0x0e5bbd59.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Refund(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Refund(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactor) Refund0(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "refund0", preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapSession) Refund0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Refund0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// Refund0 is a paid mutator transaction binding the contract method 0x36504721.
//
// Solidity: function refund(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock) returns()
func (_Erc20swap *Erc20swapTransactorSession) Refund0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int) (*types.Transaction, error) {
	return _Erc20swap.Contract.Refund0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0x8b4f3c23.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactor) RefundCooperative(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "refundCooperative", preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0x8b4f3c23.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapSession) RefundCooperative(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.RefundCooperative(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative is a paid mutator transaction binding the contract method 0x8b4f3c23.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, address refundAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactorSession) RefundCooperative(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, refundAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.RefundCooperative(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, refundAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactor) RefundCooperative0(opts *bind.TransactOpts, preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.contract.Transact(opts, "refundCooperative0", preimageHash, amount, tokenAddress, claimAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapSession) RefundCooperative0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.RefundCooperative0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock, v, r, s)
}

// RefundCooperative0 is a paid mutator transaction binding the contract method 0xfb35dd96.
//
// Solidity: function refundCooperative(bytes32 preimageHash, uint256 amount, address tokenAddress, address claimAddress, uint256 timelock, uint8 v, bytes32 r, bytes32 s) returns()
func (_Erc20swap *Erc20swapTransactorSession) RefundCooperative0(preimageHash [32]byte, amount *big.Int, tokenAddress common.Address, claimAddress common.Address, timelock *big.Int, v uint8, r [32]byte, s [32]byte) (*types.Transaction, error) {
	return _Erc20swap.Contract.RefundCooperative0(&_Erc20swap.TransactOpts, preimageHash, amount, tokenAddress, claimAddress, timelock, v, r, s)
}

// Erc20swapClaimIterator is returned from FilterClaim and is used to iterate over the raw logs and unpacked data for Claim events raised by the Erc20swap contract.
type Erc20swapClaimIterator struct {
	Event *Erc20swapClaim // Event containing the contract specifics and raw log

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
func (it *Erc20swapClaimIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Erc20swapClaim)
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
		it.Event = new(Erc20swapClaim)
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
func (it *Erc20swapClaimIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Erc20swapClaimIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Erc20swapClaim represents a Claim event raised by the Erc20swap contract.
type Erc20swapClaim struct {
	PreimageHash [32]byte
	Preimage     [32]byte
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterClaim is a free log retrieval operation binding the contract event 0x5664142af3dcfc3dc3de45a43f75c746bd1d8c11170a5037fdf98bdb35775137.
//
// Solidity: event Claim(bytes32 indexed preimageHash, bytes32 preimage)
func (_Erc20swap *Erc20swapFilterer) FilterClaim(opts *bind.FilterOpts, preimageHash [][32]byte) (*Erc20swapClaimIterator, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Erc20swap.contract.FilterLogs(opts, "Claim", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return &Erc20swapClaimIterator{contract: _Erc20swap.contract, event: "Claim", logs: logs, sub: sub}, nil
}

// WatchClaim is a free log subscription operation binding the contract event 0x5664142af3dcfc3dc3de45a43f75c746bd1d8c11170a5037fdf98bdb35775137.
//
// Solidity: event Claim(bytes32 indexed preimageHash, bytes32 preimage)
func (_Erc20swap *Erc20swapFilterer) WatchClaim(opts *bind.WatchOpts, sink chan<- *Erc20swapClaim, preimageHash [][32]byte) (event.Subscription, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Erc20swap.contract.WatchLogs(opts, "Claim", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Erc20swapClaim)
				if err := _Erc20swap.contract.UnpackLog(event, "Claim", log); err != nil {
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
func (_Erc20swap *Erc20swapFilterer) ParseClaim(log types.Log) (*Erc20swapClaim, error) {
	event := new(Erc20swapClaim)
	if err := _Erc20swap.contract.UnpackLog(event, "Claim", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// Erc20swapLockupIterator is returned from FilterLockup and is used to iterate over the raw logs and unpacked data for Lockup events raised by the Erc20swap contract.
type Erc20swapLockupIterator struct {
	Event *Erc20swapLockup // Event containing the contract specifics and raw log

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
func (it *Erc20swapLockupIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Erc20swapLockup)
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
		it.Event = new(Erc20swapLockup)
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
func (it *Erc20swapLockupIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Erc20swapLockupIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Erc20swapLockup represents a Lockup event raised by the Erc20swap contract.
type Erc20swapLockup struct {
	PreimageHash  [32]byte
	Amount        *big.Int
	TokenAddress  common.Address
	ClaimAddress  common.Address
	RefundAddress common.Address
	Timelock      *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterLockup is a free log retrieval operation binding the contract event 0xa98eaa2bd8230d87a1a4c356f5c1d41cb85ff88131122ec8b1931cb9d31ae145.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address tokenAddress, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Erc20swap *Erc20swapFilterer) FilterLockup(opts *bind.FilterOpts, preimageHash [][32]byte, claimAddress []common.Address, refundAddress []common.Address) (*Erc20swapLockupIterator, error) {

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

	logs, sub, err := _Erc20swap.contract.FilterLogs(opts, "Lockup", preimageHashRule, claimAddressRule, refundAddressRule)
	if err != nil {
		return nil, err
	}
	return &Erc20swapLockupIterator{contract: _Erc20swap.contract, event: "Lockup", logs: logs, sub: sub}, nil
}

// WatchLockup is a free log subscription operation binding the contract event 0xa98eaa2bd8230d87a1a4c356f5c1d41cb85ff88131122ec8b1931cb9d31ae145.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address tokenAddress, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Erc20swap *Erc20swapFilterer) WatchLockup(opts *bind.WatchOpts, sink chan<- *Erc20swapLockup, preimageHash [][32]byte, claimAddress []common.Address, refundAddress []common.Address) (event.Subscription, error) {

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

	logs, sub, err := _Erc20swap.contract.WatchLogs(opts, "Lockup", preimageHashRule, claimAddressRule, refundAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Erc20swapLockup)
				if err := _Erc20swap.contract.UnpackLog(event, "Lockup", log); err != nil {
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

// ParseLockup is a log parse operation binding the contract event 0xa98eaa2bd8230d87a1a4c356f5c1d41cb85ff88131122ec8b1931cb9d31ae145.
//
// Solidity: event Lockup(bytes32 indexed preimageHash, uint256 amount, address tokenAddress, address indexed claimAddress, address indexed refundAddress, uint256 timelock)
func (_Erc20swap *Erc20swapFilterer) ParseLockup(log types.Log) (*Erc20swapLockup, error) {
	event := new(Erc20swapLockup)
	if err := _Erc20swap.contract.UnpackLog(event, "Lockup", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// Erc20swapRefundIterator is returned from FilterRefund and is used to iterate over the raw logs and unpacked data for Refund events raised by the Erc20swap contract.
type Erc20swapRefundIterator struct {
	Event *Erc20swapRefund // Event containing the contract specifics and raw log

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
func (it *Erc20swapRefundIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Erc20swapRefund)
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
		it.Event = new(Erc20swapRefund)
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
func (it *Erc20swapRefundIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Erc20swapRefundIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Erc20swapRefund represents a Refund event raised by the Erc20swap contract.
type Erc20swapRefund struct {
	PreimageHash [32]byte
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRefund is a free log retrieval operation binding the contract event 0x3fbd469ec3a5ce074f975f76ce27e727ba21c99176917b97ae2e713695582a12.
//
// Solidity: event Refund(bytes32 indexed preimageHash)
func (_Erc20swap *Erc20swapFilterer) FilterRefund(opts *bind.FilterOpts, preimageHash [][32]byte) (*Erc20swapRefundIterator, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Erc20swap.contract.FilterLogs(opts, "Refund", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return &Erc20swapRefundIterator{contract: _Erc20swap.contract, event: "Refund", logs: logs, sub: sub}, nil
}

// WatchRefund is a free log subscription operation binding the contract event 0x3fbd469ec3a5ce074f975f76ce27e727ba21c99176917b97ae2e713695582a12.
//
// Solidity: event Refund(bytes32 indexed preimageHash)
func (_Erc20swap *Erc20swapFilterer) WatchRefund(opts *bind.WatchOpts, sink chan<- *Erc20swapRefund, preimageHash [][32]byte) (event.Subscription, error) {

	var preimageHashRule []interface{}
	for _, preimageHashItem := range preimageHash {
		preimageHashRule = append(preimageHashRule, preimageHashItem)
	}

	logs, sub, err := _Erc20swap.contract.WatchLogs(opts, "Refund", preimageHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Erc20swapRefund)
				if err := _Erc20swap.contract.UnpackLog(event, "Refund", log); err != nil {
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
func (_Erc20swap *Erc20swapFilterer) ParseRefund(log types.Log) (*Erc20swapRefund, error) {
	event := new(Erc20swapRefund)
	if err := _Erc20swap.contract.UnpackLog(event, "Refund", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
