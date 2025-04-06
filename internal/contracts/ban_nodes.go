// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

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

// BanNodesMetaData contains all meta data concerning the BanNodes contract.
var BanNodesMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"addBlockedID\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"removeBlockedID\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getBlockedIds\",\"outputs\":[{\"internalType\":\"string[]\",\"name\":\"\",\"type\":\"string[]\"}],\"stateMutability\":\"view\",\"type\":\"function\",\"constant\":true}]",
	Bin: "0x608060405234801561000f575f80fd5b506104848061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c8063a65eac9314610043578063d722c37214610058578063e768a4541461006b575b5f80fd5b610056610051366004610211565b610089565b005b610056610066366004610211565b6100f1565b61007361012a565b60405161008091906102d8565b60405180910390f35b60405163ab04ebe760e01b815273__StringArray___________________________9063ab04ebe7906100c2905f90859060040161033a565b5f6040518083038186803b1580156100d8575f80fd5b505af41580156100ea573d5f803e3d5ffd5b5050505050565b60405163b395d77160e01b815273__StringArray___________________________9063b395d771906100c2905f90859060040161033a565b604051632d9e58dd60e01b81525f600482015260609073__StringArray___________________________90632d9e58dd906024015f60405180830381865af4158015610179573d5f803e3d5ffd5b505050506040513d5f823e601f3d908101601f191682016040526101a0919081019061035a565b905090565b634e487b7160e01b5f52604160045260245ffd5b604051601f8201601f1916810167ffffffffffffffff811182821017156101e2576101e26101a5565b604052919050565b5f67ffffffffffffffff821115610203576102036101a5565b50601f01601f191660200190565b5f60208284031215610221575f80fd5b813567ffffffffffffffff811115610237575f80fd5b8201601f81018413610247575f80fd5b803561025a610255826101ea565b6101b9565b81815285602083850101111561026e575f80fd5b816020840160208301375f91810160200191909152949350505050565b5f5b838110156102a557818101518382015260200161028d565b50505f910152565b5f81518084526102c481602086016020860161028b565b601f01601f19169290920160200192915050565b5f60208083016020845280855180835260408601915060408160051b8701019250602087015f5b8281101561032d57603f1988860301845261031b8583516102ad565b945092850192908501906001016102ff565b5092979650505050505050565b828152604060208201525f61035260408301846102ad565b949350505050565b5f602080838503121561036b575f80fd5b825167ffffffffffffffff80821115610382575f80fd5b818501915085601f830112610395575f80fd5b8151818111156103a7576103a76101a5565b8060051b6103b68582016101b9565b91825283810185019185810190898411156103cf575f80fd5b86860192505b83831015610441578251858111156103eb575f80fd5b8601603f81018b136103fb575f80fd5b87810151604061040d610255836101ea565b8281528d82848601011115610420575f80fd5b61042f838c830184870161028b565b855250505091860191908601906103d5565b999850505050505050505056fea26469706673582212207757ef25d8e3a0a5248f47f24fbb944e8b22524204426e853e3f2602574afd6664736f6c63430008160033",
}

// BanNodesABI is the input ABI used to generate the binding from.
// Deprecated: Use BanNodesMetaData.ABI instead.
var BanNodesABI = BanNodesMetaData.ABI

// BanNodesBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use BanNodesMetaData.Bin instead.
var BanNodesBin = BanNodesMetaData.Bin

// DeployBanNodes deploys a new Ethereum contract, binding an instance of BanNodes to it.
func DeployBanNodes(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *BanNodes, error) {
	parsed, err := BanNodesMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(BanNodesBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &BanNodes{BanNodesCaller: BanNodesCaller{contract: contract}, BanNodesTransactor: BanNodesTransactor{contract: contract}, BanNodesFilterer: BanNodesFilterer{contract: contract}}, nil
}

// BanNodes is an auto generated Go binding around an Ethereum contract.
type BanNodes struct {
	BanNodesCaller     // Read-only binding to the contract
	BanNodesTransactor // Write-only binding to the contract
	BanNodesFilterer   // Log filterer for contract events
}

// BanNodesCaller is an auto generated read-only Go binding around an Ethereum contract.
type BanNodesCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BanNodesTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BanNodesTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BanNodesFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BanNodesFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BanNodesSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BanNodesSession struct {
	Contract     *BanNodes         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BanNodesCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BanNodesCallerSession struct {
	Contract *BanNodesCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// BanNodesTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BanNodesTransactorSession struct {
	Contract     *BanNodesTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// BanNodesRaw is an auto generated low-level Go binding around an Ethereum contract.
type BanNodesRaw struct {
	Contract *BanNodes // Generic contract binding to access the raw methods on
}

// BanNodesCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BanNodesCallerRaw struct {
	Contract *BanNodesCaller // Generic read-only contract binding to access the raw methods on
}

// BanNodesTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BanNodesTransactorRaw struct {
	Contract *BanNodesTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBanNodes creates a new instance of BanNodes, bound to a specific deployed contract.
func NewBanNodes(address common.Address, backend bind.ContractBackend) (*BanNodes, error) {
	contract, err := bindBanNodes(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BanNodes{BanNodesCaller: BanNodesCaller{contract: contract}, BanNodesTransactor: BanNodesTransactor{contract: contract}, BanNodesFilterer: BanNodesFilterer{contract: contract}}, nil
}

// NewBanNodesCaller creates a new read-only instance of BanNodes, bound to a specific deployed contract.
func NewBanNodesCaller(address common.Address, caller bind.ContractCaller) (*BanNodesCaller, error) {
	contract, err := bindBanNodes(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BanNodesCaller{contract: contract}, nil
}

// NewBanNodesTransactor creates a new write-only instance of BanNodes, bound to a specific deployed contract.
func NewBanNodesTransactor(address common.Address, transactor bind.ContractTransactor) (*BanNodesTransactor, error) {
	contract, err := bindBanNodes(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BanNodesTransactor{contract: contract}, nil
}

// NewBanNodesFilterer creates a new log filterer instance of BanNodes, bound to a specific deployed contract.
func NewBanNodesFilterer(address common.Address, filterer bind.ContractFilterer) (*BanNodesFilterer, error) {
	contract, err := bindBanNodes(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BanNodesFilterer{contract: contract}, nil
}

// bindBanNodes binds a generic wrapper to an already deployed contract.
func bindBanNodes(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BanNodesMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BanNodes *BanNodesRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BanNodes.Contract.BanNodesCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BanNodes *BanNodesRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BanNodes.Contract.BanNodesTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BanNodes *BanNodesRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BanNodes.Contract.BanNodesTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BanNodes *BanNodesCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BanNodes.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BanNodes *BanNodesTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BanNodes.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BanNodes *BanNodesTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BanNodes.Contract.contract.Transact(opts, method, params...)
}

// GetBlockedIds is a free data retrieval call binding the contract method 0xe768a454.
//
// Solidity: function getBlockedIds() view returns(string[])
func (_BanNodes *BanNodesCaller) GetBlockedIds(opts *bind.CallOpts) ([]string, error) {
	var out []interface{}
	err := _BanNodes.contract.Call(opts, &out, "getBlockedIds")

	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err

}

// GetBlockedIds is a free data retrieval call binding the contract method 0xe768a454.
//
// Solidity: function getBlockedIds() view returns(string[])
func (_BanNodes *BanNodesSession) GetBlockedIds() ([]string, error) {
	return _BanNodes.Contract.GetBlockedIds(&_BanNodes.CallOpts)
}

// GetBlockedIds is a free data retrieval call binding the contract method 0xe768a454.
//
// Solidity: function getBlockedIds() view returns(string[])
func (_BanNodes *BanNodesCallerSession) GetBlockedIds() ([]string, error) {
	return _BanNodes.Contract.GetBlockedIds(&_BanNodes.CallOpts)
}

// AddBlockedID is a paid mutator transaction binding the contract method 0xa65eac93.
//
// Solidity: function addBlockedID(string value) returns()
func (_BanNodes *BanNodesTransactor) AddBlockedID(opts *bind.TransactOpts, value string) (*types.Transaction, error) {
	return _BanNodes.contract.Transact(opts, "addBlockedID", value)
}

// AddBlockedID is a paid mutator transaction binding the contract method 0xa65eac93.
//
// Solidity: function addBlockedID(string value) returns()
func (_BanNodes *BanNodesSession) AddBlockedID(value string) (*types.Transaction, error) {
	return _BanNodes.Contract.AddBlockedID(&_BanNodes.TransactOpts, value)
}

// AddBlockedID is a paid mutator transaction binding the contract method 0xa65eac93.
//
// Solidity: function addBlockedID(string value) returns()
func (_BanNodes *BanNodesTransactorSession) AddBlockedID(value string) (*types.Transaction, error) {
	return _BanNodes.Contract.AddBlockedID(&_BanNodes.TransactOpts, value)
}

// RemoveBlockedID is a paid mutator transaction binding the contract method 0xd722c372.
//
// Solidity: function removeBlockedID(string value) returns()
func (_BanNodes *BanNodesTransactor) RemoveBlockedID(opts *bind.TransactOpts, value string) (*types.Transaction, error) {
	return _BanNodes.contract.Transact(opts, "removeBlockedID", value)
}

// RemoveBlockedID is a paid mutator transaction binding the contract method 0xd722c372.
//
// Solidity: function removeBlockedID(string value) returns()
func (_BanNodes *BanNodesSession) RemoveBlockedID(value string) (*types.Transaction, error) {
	return _BanNodes.Contract.RemoveBlockedID(&_BanNodes.TransactOpts, value)
}

// RemoveBlockedID is a paid mutator transaction binding the contract method 0xd722c372.
//
// Solidity: function removeBlockedID(string value) returns()
func (_BanNodes *BanNodesTransactorSession) RemoveBlockedID(value string) (*types.Transaction, error) {
	return _BanNodes.Contract.RemoveBlockedID(&_BanNodes.TransactOpts, value)
}
