// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

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

// RegistryMetaData contains all meta data concerning the Registry contract.
var RegistryMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"RecommendedVersionRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"VersionAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"VersionRecommended\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"enumContractManager.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"enumContractManager.BugLevel\",\"name\":\"bugLevel\",\"type\":\"uint8\"}],\"name\":\"VersionUpdated\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"},{\"internalType\":\"enumContractManager.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"addVersion\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"},{\"internalType\":\"enumContractManager.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"enumContractManager.BugLevel\",\"name\":\"bugLevel\",\"type\":\"uint8\"}],\"name\":\"updateVersion\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"markRecommendedVersion\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"getRecommendedVersion\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"},{\"internalType\":\"enumContractManager.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"enumContractManager.BugLevel\",\"name\":\"bugLevel\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"dateAdded\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"removeRecommendedVersion\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getTotalContractCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"}],\"name\":\"getVersionCountForContract\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getContractAtIndex\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getVersionAtIndex\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"contractName\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"versionName\",\"type\":\"string\"}],\"name\":\"getVersionDetails\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"versionString\",\"type\":\"string\"},{\"internalType\":\"enumContractManager.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"enumContractManager.BugLevel\",\"name\":\"bugLevel\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"dateAdded\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true}]",
}

// RegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use RegistryMetaData.ABI instead.
var RegistryABI = RegistryMetaData.ABI

// Registry is an auto generated Go binding around an Ethereum contract.
type Registry struct {
	RegistryCaller     // Read-only binding to the contract
	RegistryTransactor // Write-only binding to the contract
	RegistryFilterer   // Log filterer for contract events
}

// RegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type RegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RegistrySession struct {
	Contract     *Registry         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RegistryCallerSession struct {
	Contract *RegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// RegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RegistryTransactorSession struct {
	Contract     *RegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// RegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type RegistryRaw struct {
	Contract *Registry // Generic contract binding to access the raw methods on
}

// RegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RegistryCallerRaw struct {
	Contract *RegistryCaller // Generic read-only contract binding to access the raw methods on
}

// RegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RegistryTransactorRaw struct {
	Contract *RegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRegistry creates a new instance of Registry, bound to a specific deployed contract.
func NewRegistry(address common.Address, backend bind.ContractBackend) (*Registry, error) {
	contract, err := bindRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Registry{RegistryCaller: RegistryCaller{contract: contract}, RegistryTransactor: RegistryTransactor{contract: contract}, RegistryFilterer: RegistryFilterer{contract: contract}}, nil
}

// NewRegistryCaller creates a new read-only instance of Registry, bound to a specific deployed contract.
func NewRegistryCaller(address common.Address, caller bind.ContractCaller) (*RegistryCaller, error) {
	contract, err := bindRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RegistryCaller{contract: contract}, nil
}

// NewRegistryTransactor creates a new write-only instance of Registry, bound to a specific deployed contract.
func NewRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*RegistryTransactor, error) {
	contract, err := bindRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RegistryTransactor{contract: contract}, nil
}

// NewRegistryFilterer creates a new log filterer instance of Registry, bound to a specific deployed contract.
func NewRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*RegistryFilterer, error) {
	contract, err := bindRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RegistryFilterer{contract: contract}, nil
}

// bindRegistry binds a generic wrapper to an already deployed contract.
func bindRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Registry *RegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Registry.Contract.RegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Registry *RegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.Contract.RegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Registry *RegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Registry.Contract.RegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Registry *RegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Registry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Registry *RegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Registry *RegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Registry.Contract.contract.Transact(opts, method, params...)
}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) view returns(string contractName)
func (_Registry *RegistryCaller) GetContractAtIndex(opts *bind.CallOpts, index *big.Int) (string, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getContractAtIndex", index)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) view returns(string contractName)
func (_Registry *RegistrySession) GetContractAtIndex(index *big.Int) (string, error) {
	return _Registry.Contract.GetContractAtIndex(&_Registry.CallOpts, index)
}

// GetContractAtIndex is a free data retrieval call binding the contract method 0x9b534f18.
//
// Solidity: function getContractAtIndex(uint256 index) view returns(string contractName)
func (_Registry *RegistryCallerSession) GetContractAtIndex(index *big.Int) (string, error) {
	return _Registry.Contract.GetContractAtIndex(&_Registry.CallOpts, index)
}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) view returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCaller) GetRecommendedVersion(opts *bind.CallOpts, contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getRecommendedVersion", contractName)

	outstruct := new(struct {
		VersionName    string
		Status         uint8
		BugLevel       uint8
		Implementation common.Address
		DateAdded      *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.VersionName = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.Status = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.BugLevel = *abi.ConvertType(out[2], new(uint8)).(*uint8)
	outstruct.Implementation = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.DateAdded = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) view returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistrySession) GetRecommendedVersion(contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetRecommendedVersion(&_Registry.CallOpts, contractName)
}

// GetRecommendedVersion is a free data retrieval call binding the contract method 0x7e27634e.
//
// Solidity: function getRecommendedVersion(string contractName) view returns(string versionName, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCallerSession) GetRecommendedVersion(contractName string) (struct {
	VersionName    string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetRecommendedVersion(&_Registry.CallOpts, contractName)
}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() view returns(uint256 count)
func (_Registry *RegistryCaller) GetTotalContractCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getTotalContractCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() view returns(uint256 count)
func (_Registry *RegistrySession) GetTotalContractCount() (*big.Int, error) {
	return _Registry.Contract.GetTotalContractCount(&_Registry.CallOpts)
}

// GetTotalContractCount is a free data retrieval call binding the contract method 0x751f592b.
//
// Solidity: function getTotalContractCount() view returns(uint256 count)
func (_Registry *RegistryCallerSession) GetTotalContractCount() (*big.Int, error) {
	return _Registry.Contract.GetTotalContractCount(&_Registry.CallOpts)
}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) view returns(string versionName)
func (_Registry *RegistryCaller) GetVersionAtIndex(opts *bind.CallOpts, contractName string, index *big.Int) (string, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getVersionAtIndex", contractName, index)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) view returns(string versionName)
func (_Registry *RegistrySession) GetVersionAtIndex(contractName string, index *big.Int) (string, error) {
	return _Registry.Contract.GetVersionAtIndex(&_Registry.CallOpts, contractName, index)
}

// GetVersionAtIndex is a free data retrieval call binding the contract method 0xaf2c7fa3.
//
// Solidity: function getVersionAtIndex(string contractName, uint256 index) view returns(string versionName)
func (_Registry *RegistryCallerSession) GetVersionAtIndex(contractName string, index *big.Int) (string, error) {
	return _Registry.Contract.GetVersionAtIndex(&_Registry.CallOpts, contractName, index)
}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) view returns(uint256 count)
func (_Registry *RegistryCaller) GetVersionCountForContract(opts *bind.CallOpts, contractName string) (*big.Int, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getVersionCountForContract", contractName)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) view returns(uint256 count)
func (_Registry *RegistrySession) GetVersionCountForContract(contractName string) (*big.Int, error) {
	return _Registry.Contract.GetVersionCountForContract(&_Registry.CallOpts, contractName)
}

// GetVersionCountForContract is a free data retrieval call binding the contract method 0xff93dab4.
//
// Solidity: function getVersionCountForContract(string contractName) view returns(uint256 count)
func (_Registry *RegistryCallerSession) GetVersionCountForContract(contractName string) (*big.Int, error) {
	return _Registry.Contract.GetVersionCountForContract(&_Registry.CallOpts, contractName)
}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) view returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCaller) GetVersionDetails(opts *bind.CallOpts, contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "getVersionDetails", contractName, versionName)

	outstruct := new(struct {
		VersionString  string
		Status         uint8
		BugLevel       uint8
		Implementation common.Address
		DateAdded      *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.VersionString = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.Status = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.BugLevel = *abi.ConvertType(out[2], new(uint8)).(*uint8)
	outstruct.Implementation = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.DateAdded = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) view returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistrySession) GetVersionDetails(contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetVersionDetails(&_Registry.CallOpts, contractName, versionName)
}

// GetVersionDetails is a free data retrieval call binding the contract method 0x8ad030c1.
//
// Solidity: function getVersionDetails(string contractName, string versionName) view returns(string versionString, uint8 status, uint8 bugLevel, address implementation, uint256 dateAdded)
func (_Registry *RegistryCallerSession) GetVersionDetails(contractName string, versionName string) (struct {
	VersionString  string
	Status         uint8
	BugLevel       uint8
	Implementation common.Address
	DateAdded      *big.Int
}, error) {
	return _Registry.Contract.GetVersionDetails(&_Registry.CallOpts, contractName, versionName)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Registry *RegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Registry.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Registry *RegistrySession) Owner() (common.Address, error) {
	return _Registry.Contract.Owner(&_Registry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Registry *RegistryCallerSession) Owner() (common.Address, error) {
	return _Registry.Contract.Owner(&_Registry.CallOpts)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistryTransactor) AddVersion(opts *bind.TransactOpts, contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "addVersion", contractName, versionName, status, implementation)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistrySession) AddVersion(contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.Contract.AddVersion(&_Registry.TransactOpts, contractName, versionName, status, implementation)
}

// AddVersion is a paid mutator transaction binding the contract method 0x3de5311a.
//
// Solidity: function addVersion(string contractName, string versionName, uint8 status, address implementation) returns()
func (_Registry *RegistryTransactorSession) AddVersion(contractName string, versionName string, status uint8, implementation common.Address) (*types.Transaction, error) {
	return _Registry.Contract.AddVersion(&_Registry.TransactOpts, contractName, versionName, status, implementation)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistryTransactor) MarkRecommendedVersion(opts *bind.TransactOpts, contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "markRecommendedVersion", contractName, versionName)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistrySession) MarkRecommendedVersion(contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.Contract.MarkRecommendedVersion(&_Registry.TransactOpts, contractName, versionName)
}

// MarkRecommendedVersion is a paid mutator transaction binding the contract method 0xca2e7cf3.
//
// Solidity: function markRecommendedVersion(string contractName, string versionName) returns()
func (_Registry *RegistryTransactorSession) MarkRecommendedVersion(contractName string, versionName string) (*types.Transaction, error) {
	return _Registry.Contract.MarkRecommendedVersion(&_Registry.TransactOpts, contractName, versionName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistryTransactor) RemoveRecommendedVersion(opts *bind.TransactOpts, contractName string) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "removeRecommendedVersion", contractName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistrySession) RemoveRecommendedVersion(contractName string) (*types.Transaction, error) {
	return _Registry.Contract.RemoveRecommendedVersion(&_Registry.TransactOpts, contractName)
}

// RemoveRecommendedVersion is a paid mutator transaction binding the contract method 0xacd820a8.
//
// Solidity: function removeRecommendedVersion(string contractName) returns()
func (_Registry *RegistryTransactorSession) RemoveRecommendedVersion(contractName string) (*types.Transaction, error) {
	return _Registry.Contract.RemoveRecommendedVersion(&_Registry.TransactOpts, contractName)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistryTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistrySession) RenounceOwnership() (*types.Transaction, error) {
	return _Registry.Contract.RenounceOwnership(&_Registry.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Registry *RegistryTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Registry.Contract.RenounceOwnership(&_Registry.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Registry *RegistryTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Registry *RegistrySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Registry.Contract.TransferOwnership(&_Registry.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Registry *RegistryTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Registry.Contract.TransferOwnership(&_Registry.TransactOpts, newOwner)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistryTransactor) UpdateVersion(opts *bind.TransactOpts, contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.contract.Transact(opts, "updateVersion", contractName, versionName, status, bugLevel)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistrySession) UpdateVersion(contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.Contract.UpdateVersion(&_Registry.TransactOpts, contractName, versionName, status, bugLevel)
}

// UpdateVersion is a paid mutator transaction binding the contract method 0x5c047fa2.
//
// Solidity: function updateVersion(string contractName, string versionName, uint8 status, uint8 bugLevel) returns()
func (_Registry *RegistryTransactorSession) UpdateVersion(contractName string, versionName string, status uint8, bugLevel uint8) (*types.Transaction, error) {
	return _Registry.Contract.UpdateVersion(&_Registry.TransactOpts, contractName, versionName, status, bugLevel)
}

// RegistryOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Registry contract.
type RegistryOwnershipTransferredIterator struct {
	Event *RegistryOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *RegistryOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryOwnershipTransferred)
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
		it.Event = new(RegistryOwnershipTransferred)
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
func (it *RegistryOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryOwnershipTransferred represents a OwnershipTransferred event raised by the Registry contract.
type RegistryOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Registry *RegistryFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*RegistryOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Registry.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &RegistryOwnershipTransferredIterator{contract: _Registry.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Registry *RegistryFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *RegistryOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Registry.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryOwnershipTransferred)
				if err := _Registry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Registry *RegistryFilterer) ParseOwnershipTransferred(log types.Log) (*RegistryOwnershipTransferred, error) {
	event := new(RegistryOwnershipTransferred)
	if err := _Registry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RegistryRecommendedVersionRemovedIterator is returned from FilterRecommendedVersionRemoved and is used to iterate over the raw logs and unpacked data for RecommendedVersionRemoved events raised by the Registry contract.
type RegistryRecommendedVersionRemovedIterator struct {
	Event *RegistryRecommendedVersionRemoved // Event containing the contract specifics and raw log

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
func (it *RegistryRecommendedVersionRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryRecommendedVersionRemoved)
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
		it.Event = new(RegistryRecommendedVersionRemoved)
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
func (it *RegistryRecommendedVersionRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryRecommendedVersionRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryRecommendedVersionRemoved represents a RecommendedVersionRemoved event raised by the Registry contract.
type RegistryRecommendedVersionRemoved struct {
	ContractName string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRecommendedVersionRemoved is a free log retrieval operation binding the contract event 0x07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f2.
//
// Solidity: event RecommendedVersionRemoved(string contractName)
func (_Registry *RegistryFilterer) FilterRecommendedVersionRemoved(opts *bind.FilterOpts) (*RegistryRecommendedVersionRemovedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "RecommendedVersionRemoved")
	if err != nil {
		return nil, err
	}
	return &RegistryRecommendedVersionRemovedIterator{contract: _Registry.contract, event: "RecommendedVersionRemoved", logs: logs, sub: sub}, nil
}

// WatchRecommendedVersionRemoved is a free log subscription operation binding the contract event 0x07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f2.
//
// Solidity: event RecommendedVersionRemoved(string contractName)
func (_Registry *RegistryFilterer) WatchRecommendedVersionRemoved(opts *bind.WatchOpts, sink chan<- *RegistryRecommendedVersionRemoved) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "RecommendedVersionRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryRecommendedVersionRemoved)
				if err := _Registry.contract.UnpackLog(event, "RecommendedVersionRemoved", log); err != nil {
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

// ParseRecommendedVersionRemoved is a log parse operation binding the contract event 0x07b20feb74e0118ee3c73d4cb8d0eb4da169604c68aa233293b094cedcd225f2.
//
// Solidity: event RecommendedVersionRemoved(string contractName)
func (_Registry *RegistryFilterer) ParseRecommendedVersionRemoved(log types.Log) (*RegistryRecommendedVersionRemoved, error) {
	event := new(RegistryRecommendedVersionRemoved)
	if err := _Registry.contract.UnpackLog(event, "RecommendedVersionRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RegistryVersionAddedIterator is returned from FilterVersionAdded and is used to iterate over the raw logs and unpacked data for VersionAdded events raised by the Registry contract.
type RegistryVersionAddedIterator struct {
	Event *RegistryVersionAdded // Event containing the contract specifics and raw log

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
func (it *RegistryVersionAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionAdded)
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
		it.Event = new(RegistryVersionAdded)
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
func (it *RegistryVersionAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionAdded represents a VersionAdded event raised by the Registry contract.
type RegistryVersionAdded struct {
	ContractName   string
	VersionName    string
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterVersionAdded is a free log retrieval operation binding the contract event 0x337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2.
//
// Solidity: event VersionAdded(string contractName, string versionName, address indexed implementation)
func (_Registry *RegistryFilterer) FilterVersionAdded(opts *bind.FilterOpts, implementation []common.Address) (*RegistryVersionAddedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionAdded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &RegistryVersionAddedIterator{contract: _Registry.contract, event: "VersionAdded", logs: logs, sub: sub}, nil
}

// WatchVersionAdded is a free log subscription operation binding the contract event 0x337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2.
//
// Solidity: event VersionAdded(string contractName, string versionName, address indexed implementation)
func (_Registry *RegistryFilterer) WatchVersionAdded(opts *bind.WatchOpts, sink chan<- *RegistryVersionAdded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionAdded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionAdded)
				if err := _Registry.contract.UnpackLog(event, "VersionAdded", log); err != nil {
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

// ParseVersionAdded is a log parse operation binding the contract event 0x337b109e3f497728f2bdd27545c9ed1cb52ed4a4103cc94da88b868879c982e2.
//
// Solidity: event VersionAdded(string contractName, string versionName, address indexed implementation)
func (_Registry *RegistryFilterer) ParseVersionAdded(log types.Log) (*RegistryVersionAdded, error) {
	event := new(RegistryVersionAdded)
	if err := _Registry.contract.UnpackLog(event, "VersionAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RegistryVersionRecommendedIterator is returned from FilterVersionRecommended and is used to iterate over the raw logs and unpacked data for VersionRecommended events raised by the Registry contract.
type RegistryVersionRecommendedIterator struct {
	Event *RegistryVersionRecommended // Event containing the contract specifics and raw log

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
func (it *RegistryVersionRecommendedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionRecommended)
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
		it.Event = new(RegistryVersionRecommended)
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
func (it *RegistryVersionRecommendedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionRecommendedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionRecommended represents a VersionRecommended event raised by the Registry contract.
type RegistryVersionRecommended struct {
	ContractName string
	VersionName  string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterVersionRecommended is a free log retrieval operation binding the contract event 0xb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6.
//
// Solidity: event VersionRecommended(string contractName, string versionName)
func (_Registry *RegistryFilterer) FilterVersionRecommended(opts *bind.FilterOpts) (*RegistryVersionRecommendedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionRecommended")
	if err != nil {
		return nil, err
	}
	return &RegistryVersionRecommendedIterator{contract: _Registry.contract, event: "VersionRecommended", logs: logs, sub: sub}, nil
}

// WatchVersionRecommended is a free log subscription operation binding the contract event 0xb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6.
//
// Solidity: event VersionRecommended(string contractName, string versionName)
func (_Registry *RegistryFilterer) WatchVersionRecommended(opts *bind.WatchOpts, sink chan<- *RegistryVersionRecommended) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionRecommended")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionRecommended)
				if err := _Registry.contract.UnpackLog(event, "VersionRecommended", log); err != nil {
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

// ParseVersionRecommended is a log parse operation binding the contract event 0xb318550bf93edf51de4bae84db3deabd2a866cc407435a72317ca2503e2a07a6.
//
// Solidity: event VersionRecommended(string contractName, string versionName)
func (_Registry *RegistryFilterer) ParseVersionRecommended(log types.Log) (*RegistryVersionRecommended, error) {
	event := new(RegistryVersionRecommended)
	if err := _Registry.contract.UnpackLog(event, "VersionRecommended", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RegistryVersionUpdatedIterator is returned from FilterVersionUpdated and is used to iterate over the raw logs and unpacked data for VersionUpdated events raised by the Registry contract.
type RegistryVersionUpdatedIterator struct {
	Event *RegistryVersionUpdated // Event containing the contract specifics and raw log

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
func (it *RegistryVersionUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RegistryVersionUpdated)
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
		it.Event = new(RegistryVersionUpdated)
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
func (it *RegistryVersionUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RegistryVersionUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RegistryVersionUpdated represents a VersionUpdated event raised by the Registry contract.
type RegistryVersionUpdated struct {
	ContractName string
	VersionName  string
	Status       uint8
	BugLevel     uint8
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterVersionUpdated is a free log retrieval operation binding the contract event 0x0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d751.
//
// Solidity: event VersionUpdated(string contractName, string versionName, uint8 status, uint8 bugLevel)
func (_Registry *RegistryFilterer) FilterVersionUpdated(opts *bind.FilterOpts) (*RegistryVersionUpdatedIterator, error) {

	logs, sub, err := _Registry.contract.FilterLogs(opts, "VersionUpdated")
	if err != nil {
		return nil, err
	}
	return &RegistryVersionUpdatedIterator{contract: _Registry.contract, event: "VersionUpdated", logs: logs, sub: sub}, nil
}

// WatchVersionUpdated is a free log subscription operation binding the contract event 0x0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d751.
//
// Solidity: event VersionUpdated(string contractName, string versionName, uint8 status, uint8 bugLevel)
func (_Registry *RegistryFilterer) WatchVersionUpdated(opts *bind.WatchOpts, sink chan<- *RegistryVersionUpdated) (event.Subscription, error) {

	logs, sub, err := _Registry.contract.WatchLogs(opts, "VersionUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RegistryVersionUpdated)
				if err := _Registry.contract.UnpackLog(event, "VersionUpdated", log); err != nil {
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

// ParseVersionUpdated is a log parse operation binding the contract event 0x0acf3e1a00b57bfc05ebf65957f42293847dc0938bfa1744660d6df56036d751.
//
// Solidity: event VersionUpdated(string contractName, string versionName, uint8 status, uint8 bugLevel)
func (_Registry *RegistryFilterer) ParseVersionUpdated(log types.Log) (*RegistryVersionUpdated, error) {
	event := new(RegistryVersionUpdated)
	if err := _Registry.contract.UnpackLog(event, "VersionUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
