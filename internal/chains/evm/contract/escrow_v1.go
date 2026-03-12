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

// PayData is an auto generated low-level Go binding around an user-defined struct.
type PayData struct {
	Destinations []common.Address
	Amounts      []*big.Int
}

// EscrowMetaData contains all meta data concerning the Escrow contract.
var EscrowMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"addresspayable[]\",\"name\":\"destinations\",\"type\":\"address[]\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"name\":\"Executed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Funded\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"transactionCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"transactions\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"payerAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"lastModified\",\"type\":\"uint256\"},{\"internalType\":\"enumEscrow.Status\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"enumEscrow.TransactionType\",\"name\":\"transactionType\",\"type\":\"uint8\"},{\"internalType\":\"uint8\",\"name\":\"threshold\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"timeoutHours\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"buyer\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"moderator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"released\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"noOfReleases\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"enumEscrow.TransactionType\",\"name\":\"transactionType\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"transferLockedFunds\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"buyer\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"moderator\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"threshold\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"timeoutHours\",\"type\":\"uint32\"},{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes20\",\"name\":\"uniqueId\",\"type\":\"bytes20\"}],\"name\":\"addTransaction\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"buyer\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"moderator\",\"type\":\"address\"},{\"internalType\":\"uint8\",\"name\":\"threshold\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"timeoutHours\",\"type\":\"uint32\"},{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"bytes20\",\"name\":\"uniqueId\",\"type\":\"bytes20\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"addTokenTransaction\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"}],\"name\":\"checkBeneficiary\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"party\",\"type\":\"address\"}],\"name\":\"checkVote\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"partyAddress\",\"type\":\"address\"}],\"name\":\"getAllTransactionsForParty\",\"outputs\":[{\"internalType\":\"bytes32[]\",\"name\":\"\",\"type\":\"bytes32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8[]\",\"name\":\"sigV\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes32[]\",\"name\":\"sigR\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes32[]\",\"name\":\"sigS\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"addresspayable[]\",\"name\":\"destinations\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"internalType\":\"structPayData\",\"name\":\"payData\",\"type\":\"tuple\"}],\"name\":\"execute\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"scriptHash\",\"type\":\"bytes32\"},{\"internalType\":\"addresspayable[]\",\"name\":\"destinations\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"name\":\"getTransactionHash\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"uniqueId\",\"type\":\"bytes20\"},{\"internalType\":\"uint8\",\"name\":\"threshold\",\"type\":\"uint8\"},{\"internalType\":\"uint32\",\"name\":\"timeoutHours\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"buyer\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"moderator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"calculateRedeemScriptHash\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// EscrowABI is the input ABI used to generate the binding from.
// Deprecated: Use EscrowMetaData.ABI instead.
var EscrowABI = EscrowMetaData.ABI

// Escrow is an auto generated Go binding around an Ethereum contract.
type Escrow struct {
	EscrowCaller     // Read-only binding to the contract
	EscrowTransactor // Write-only binding to the contract
	EscrowFilterer   // Log filterer for contract events
}

// EscrowCaller is an auto generated read-only Go binding around an Ethereum contract.
type EscrowCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EscrowTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EscrowTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EscrowFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EscrowFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EscrowSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EscrowSession struct {
	Contract     *Escrow           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EscrowCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EscrowCallerSession struct {
	Contract *EscrowCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// EscrowTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EscrowTransactorSession struct {
	Contract     *EscrowTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EscrowRaw is an auto generated low-level Go binding around an Ethereum contract.
type EscrowRaw struct {
	Contract *Escrow // Generic contract binding to access the raw methods on
}

// EscrowCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EscrowCallerRaw struct {
	Contract *EscrowCaller // Generic read-only contract binding to access the raw methods on
}

// EscrowTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EscrowTransactorRaw struct {
	Contract *EscrowTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEscrow creates a new instance of Escrow, bound to a specific deployed contract.
func NewEscrow(address common.Address, backend bind.ContractBackend) (*Escrow, error) {
	contract, err := bindEscrow(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Escrow{EscrowCaller: EscrowCaller{contract: contract}, EscrowTransactor: EscrowTransactor{contract: contract}, EscrowFilterer: EscrowFilterer{contract: contract}}, nil
}

// NewEscrowCaller creates a new read-only instance of Escrow, bound to a specific deployed contract.
func NewEscrowCaller(address common.Address, caller bind.ContractCaller) (*EscrowCaller, error) {
	contract, err := bindEscrow(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EscrowCaller{contract: contract}, nil
}

// NewEscrowTransactor creates a new write-only instance of Escrow, bound to a specific deployed contract.
func NewEscrowTransactor(address common.Address, transactor bind.ContractTransactor) (*EscrowTransactor, error) {
	contract, err := bindEscrow(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EscrowTransactor{contract: contract}, nil
}

// NewEscrowFilterer creates a new log filterer instance of Escrow, bound to a specific deployed contract.
func NewEscrowFilterer(address common.Address, filterer bind.ContractFilterer) (*EscrowFilterer, error) {
	contract, err := bindEscrow(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EscrowFilterer{contract: contract}, nil
}

// bindEscrow binds a generic wrapper to an already deployed contract.
func bindEscrow(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EscrowMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Escrow *EscrowRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Escrow.Contract.EscrowCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Escrow *EscrowRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Escrow.Contract.EscrowTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Escrow *EscrowRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Escrow.Contract.EscrowTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Escrow *EscrowCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Escrow.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Escrow *EscrowTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Escrow.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Escrow *EscrowTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Escrow.Contract.contract.Transact(opts, method, params...)
}

// CalculateRedeemScriptHash is a free data retrieval call binding the contract method 0x46fbcdeb.
//
// Solidity: function calculateRedeemScriptHash(bytes20 uniqueId, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address moderator, address tokenAddress) view returns(bytes32)
func (_Escrow *EscrowCaller) CalculateRedeemScriptHash(opts *bind.CallOpts, uniqueId [20]byte, threshold uint8, timeoutHours uint32, buyer common.Address, seller common.Address, moderator common.Address, tokenAddress common.Address) ([32]byte, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "calculateRedeemScriptHash", uniqueId, threshold, timeoutHours, buyer, seller, moderator, tokenAddress)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// CalculateRedeemScriptHash is a free data retrieval call binding the contract method 0x46fbcdeb.
//
// Solidity: function calculateRedeemScriptHash(bytes20 uniqueId, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address moderator, address tokenAddress) view returns(bytes32)
func (_Escrow *EscrowSession) CalculateRedeemScriptHash(uniqueId [20]byte, threshold uint8, timeoutHours uint32, buyer common.Address, seller common.Address, moderator common.Address, tokenAddress common.Address) ([32]byte, error) {
	return _Escrow.Contract.CalculateRedeemScriptHash(&_Escrow.CallOpts, uniqueId, threshold, timeoutHours, buyer, seller, moderator, tokenAddress)
}

// CalculateRedeemScriptHash is a free data retrieval call binding the contract method 0x46fbcdeb.
//
// Solidity: function calculateRedeemScriptHash(bytes20 uniqueId, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address moderator, address tokenAddress) view returns(bytes32)
func (_Escrow *EscrowCallerSession) CalculateRedeemScriptHash(uniqueId [20]byte, threshold uint8, timeoutHours uint32, buyer common.Address, seller common.Address, moderator common.Address, tokenAddress common.Address) ([32]byte, error) {
	return _Escrow.Contract.CalculateRedeemScriptHash(&_Escrow.CallOpts, uniqueId, threshold, timeoutHours, buyer, seller, moderator, tokenAddress)
}

// CheckBeneficiary is a free data retrieval call binding the contract method 0xb0550c66.
//
// Solidity: function checkBeneficiary(bytes32 scriptHash, address beneficiary) view returns(bool)
func (_Escrow *EscrowCaller) CheckBeneficiary(opts *bind.CallOpts, scriptHash [32]byte, beneficiary common.Address) (bool, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "checkBeneficiary", scriptHash, beneficiary)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CheckBeneficiary is a free data retrieval call binding the contract method 0xb0550c66.
//
// Solidity: function checkBeneficiary(bytes32 scriptHash, address beneficiary) view returns(bool)
func (_Escrow *EscrowSession) CheckBeneficiary(scriptHash [32]byte, beneficiary common.Address) (bool, error) {
	return _Escrow.Contract.CheckBeneficiary(&_Escrow.CallOpts, scriptHash, beneficiary)
}

// CheckBeneficiary is a free data retrieval call binding the contract method 0xb0550c66.
//
// Solidity: function checkBeneficiary(bytes32 scriptHash, address beneficiary) view returns(bool)
func (_Escrow *EscrowCallerSession) CheckBeneficiary(scriptHash [32]byte, beneficiary common.Address) (bool, error) {
	return _Escrow.Contract.CheckBeneficiary(&_Escrow.CallOpts, scriptHash, beneficiary)
}

// CheckVote is a free data retrieval call binding the contract method 0xf0786562.
//
// Solidity: function checkVote(bytes32 scriptHash, address party) view returns(bool)
func (_Escrow *EscrowCaller) CheckVote(opts *bind.CallOpts, scriptHash [32]byte, party common.Address) (bool, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "checkVote", scriptHash, party)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CheckVote is a free data retrieval call binding the contract method 0xf0786562.
//
// Solidity: function checkVote(bytes32 scriptHash, address party) view returns(bool)
func (_Escrow *EscrowSession) CheckVote(scriptHash [32]byte, party common.Address) (bool, error) {
	return _Escrow.Contract.CheckVote(&_Escrow.CallOpts, scriptHash, party)
}

// CheckVote is a free data retrieval call binding the contract method 0xf0786562.
//
// Solidity: function checkVote(bytes32 scriptHash, address party) view returns(bool)
func (_Escrow *EscrowCallerSession) CheckVote(scriptHash [32]byte, party common.Address) (bool, error) {
	return _Escrow.Contract.CheckVote(&_Escrow.CallOpts, scriptHash, party)
}

// GetAllTransactionsForParty is a free data retrieval call binding the contract method 0xbe84ceaf.
//
// Solidity: function getAllTransactionsForParty(address partyAddress) view returns(bytes32[])
func (_Escrow *EscrowCaller) GetAllTransactionsForParty(opts *bind.CallOpts, partyAddress common.Address) ([][32]byte, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "getAllTransactionsForParty", partyAddress)

	if err != nil {
		return *new([][32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([][32]byte)).(*[][32]byte)

	return out0, err

}

// GetAllTransactionsForParty is a free data retrieval call binding the contract method 0xbe84ceaf.
//
// Solidity: function getAllTransactionsForParty(address partyAddress) view returns(bytes32[])
func (_Escrow *EscrowSession) GetAllTransactionsForParty(partyAddress common.Address) ([][32]byte, error) {
	return _Escrow.Contract.GetAllTransactionsForParty(&_Escrow.CallOpts, partyAddress)
}

// GetAllTransactionsForParty is a free data retrieval call binding the contract method 0xbe84ceaf.
//
// Solidity: function getAllTransactionsForParty(address partyAddress) view returns(bytes32[])
func (_Escrow *EscrowCallerSession) GetAllTransactionsForParty(partyAddress common.Address) ([][32]byte, error) {
	return _Escrow.Contract.GetAllTransactionsForParty(&_Escrow.CallOpts, partyAddress)
}

// GetTransactionHash is a free data retrieval call binding the contract method 0x51be2688.
//
// Solidity: function getTransactionHash(bytes32 scriptHash, address[] destinations, uint256[] amounts) view returns(bytes32)
func (_Escrow *EscrowCaller) GetTransactionHash(opts *bind.CallOpts, scriptHash [32]byte, destinations []common.Address, amounts []*big.Int) ([32]byte, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "getTransactionHash", scriptHash, destinations, amounts)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetTransactionHash is a free data retrieval call binding the contract method 0x51be2688.
//
// Solidity: function getTransactionHash(bytes32 scriptHash, address[] destinations, uint256[] amounts) view returns(bytes32)
func (_Escrow *EscrowSession) GetTransactionHash(scriptHash [32]byte, destinations []common.Address, amounts []*big.Int) ([32]byte, error) {
	return _Escrow.Contract.GetTransactionHash(&_Escrow.CallOpts, scriptHash, destinations, amounts)
}

// GetTransactionHash is a free data retrieval call binding the contract method 0x51be2688.
//
// Solidity: function getTransactionHash(bytes32 scriptHash, address[] destinations, uint256[] amounts) view returns(bytes32)
func (_Escrow *EscrowCallerSession) GetTransactionHash(scriptHash [32]byte, destinations []common.Address, amounts []*big.Int) ([32]byte, error) {
	return _Escrow.Contract.GetTransactionHash(&_Escrow.CallOpts, scriptHash, destinations, amounts)
}

// TransactionCount is a free data retrieval call binding the contract method 0xb77bf600.
//
// Solidity: function transactionCount() view returns(uint256)
func (_Escrow *EscrowCaller) TransactionCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "transactionCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TransactionCount is a free data retrieval call binding the contract method 0xb77bf600.
//
// Solidity: function transactionCount() view returns(uint256)
func (_Escrow *EscrowSession) TransactionCount() (*big.Int, error) {
	return _Escrow.Contract.TransactionCount(&_Escrow.CallOpts)
}

// TransactionCount is a free data retrieval call binding the contract method 0xb77bf600.
//
// Solidity: function transactionCount() view returns(uint256)
func (_Escrow *EscrowCallerSession) TransactionCount() (*big.Int, error) {
	return _Escrow.Contract.TransactionCount(&_Escrow.CallOpts)
}

// Transactions is a free data retrieval call binding the contract method 0x642f2eaf.
//
// Solidity: function transactions(bytes32 ) view returns(uint256 value, address payerAddress, uint256 lastModified, uint8 status, uint8 transactionType, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address tokenAddress, address moderator, uint256 released, uint256 noOfReleases)
func (_Escrow *EscrowCaller) Transactions(opts *bind.CallOpts, arg0 [32]byte) (struct {
	Value           *big.Int
	PayerAddress    common.Address
	LastModified    *big.Int
	Status          uint8
	TransactionType uint8
	Threshold       uint8
	TimeoutHours    uint32
	Buyer           common.Address
	Seller          common.Address
	TokenAddress    common.Address
	Moderator       common.Address
	Released        *big.Int
	NoOfReleases    *big.Int
}, error) {
	var out []interface{}
	err := _Escrow.contract.Call(opts, &out, "transactions", arg0)

	outstruct := new(struct {
		Value           *big.Int
		PayerAddress    common.Address
		LastModified    *big.Int
		Status          uint8
		TransactionType uint8
		Threshold       uint8
		TimeoutHours    uint32
		Buyer           common.Address
		Seller          common.Address
		TokenAddress    common.Address
		Moderator       common.Address
		Released        *big.Int
		NoOfReleases    *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Value = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.PayerAddress = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.LastModified = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.Status = *abi.ConvertType(out[3], new(uint8)).(*uint8)
	outstruct.TransactionType = *abi.ConvertType(out[4], new(uint8)).(*uint8)
	outstruct.Threshold = *abi.ConvertType(out[5], new(uint8)).(*uint8)
	outstruct.TimeoutHours = *abi.ConvertType(out[6], new(uint32)).(*uint32)
	outstruct.Buyer = *abi.ConvertType(out[7], new(common.Address)).(*common.Address)
	outstruct.Seller = *abi.ConvertType(out[8], new(common.Address)).(*common.Address)
	outstruct.TokenAddress = *abi.ConvertType(out[9], new(common.Address)).(*common.Address)
	outstruct.Moderator = *abi.ConvertType(out[10], new(common.Address)).(*common.Address)
	outstruct.Released = *abi.ConvertType(out[11], new(*big.Int)).(**big.Int)
	outstruct.NoOfReleases = *abi.ConvertType(out[12], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Transactions is a free data retrieval call binding the contract method 0x642f2eaf.
//
// Solidity: function transactions(bytes32 ) view returns(uint256 value, address payerAddress, uint256 lastModified, uint8 status, uint8 transactionType, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address tokenAddress, address moderator, uint256 released, uint256 noOfReleases)
func (_Escrow *EscrowSession) Transactions(arg0 [32]byte) (struct {
	Value           *big.Int
	PayerAddress    common.Address
	LastModified    *big.Int
	Status          uint8
	TransactionType uint8
	Threshold       uint8
	TimeoutHours    uint32
	Buyer           common.Address
	Seller          common.Address
	TokenAddress    common.Address
	Moderator       common.Address
	Released        *big.Int
	NoOfReleases    *big.Int
}, error) {
	return _Escrow.Contract.Transactions(&_Escrow.CallOpts, arg0)
}

// Transactions is a free data retrieval call binding the contract method 0x642f2eaf.
//
// Solidity: function transactions(bytes32 ) view returns(uint256 value, address payerAddress, uint256 lastModified, uint8 status, uint8 transactionType, uint8 threshold, uint32 timeoutHours, address buyer, address seller, address tokenAddress, address moderator, uint256 released, uint256 noOfReleases)
func (_Escrow *EscrowCallerSession) Transactions(arg0 [32]byte) (struct {
	Value           *big.Int
	PayerAddress    common.Address
	LastModified    *big.Int
	Status          uint8
	TransactionType uint8
	Threshold       uint8
	TimeoutHours    uint32
	Buyer           common.Address
	Seller          common.Address
	TokenAddress    common.Address
	Moderator       common.Address
	Released        *big.Int
	NoOfReleases    *big.Int
}, error) {
	return _Escrow.Contract.Transactions(&_Escrow.CallOpts, arg0)
}

// AddTokenTransaction is a paid mutator transaction binding the contract method 0x57bced76.
//
// Solidity: function addTokenTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, uint256 value, bytes20 uniqueId, address tokenAddress) returns()
func (_Escrow *EscrowTransactor) AddTokenTransaction(opts *bind.TransactOpts, buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, value *big.Int, uniqueId [20]byte, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.contract.Transact(opts, "addTokenTransaction", buyer, seller, moderator, threshold, timeoutHours, scriptHash, value, uniqueId, tokenAddress)
}

// AddTokenTransaction is a paid mutator transaction binding the contract method 0x57bced76.
//
// Solidity: function addTokenTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, uint256 value, bytes20 uniqueId, address tokenAddress) returns()
func (_Escrow *EscrowSession) AddTokenTransaction(buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, value *big.Int, uniqueId [20]byte, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.Contract.AddTokenTransaction(&_Escrow.TransactOpts, buyer, seller, moderator, threshold, timeoutHours, scriptHash, value, uniqueId, tokenAddress)
}

// AddTokenTransaction is a paid mutator transaction binding the contract method 0x57bced76.
//
// Solidity: function addTokenTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, uint256 value, bytes20 uniqueId, address tokenAddress) returns()
func (_Escrow *EscrowTransactorSession) AddTokenTransaction(buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, value *big.Int, uniqueId [20]byte, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.Contract.AddTokenTransaction(&_Escrow.TransactOpts, buyer, seller, moderator, threshold, timeoutHours, scriptHash, value, uniqueId, tokenAddress)
}

// AddTransaction is a paid mutator transaction binding the contract method 0x23b6fd3f.
//
// Solidity: function addTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, bytes20 uniqueId) payable returns()
func (_Escrow *EscrowTransactor) AddTransaction(opts *bind.TransactOpts, buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, uniqueId [20]byte) (*types.Transaction, error) {
	return _Escrow.contract.Transact(opts, "addTransaction", buyer, seller, moderator, threshold, timeoutHours, scriptHash, uniqueId)
}

// AddTransaction is a paid mutator transaction binding the contract method 0x23b6fd3f.
//
// Solidity: function addTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, bytes20 uniqueId) payable returns()
func (_Escrow *EscrowSession) AddTransaction(buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, uniqueId [20]byte) (*types.Transaction, error) {
	return _Escrow.Contract.AddTransaction(&_Escrow.TransactOpts, buyer, seller, moderator, threshold, timeoutHours, scriptHash, uniqueId)
}

// AddTransaction is a paid mutator transaction binding the contract method 0x23b6fd3f.
//
// Solidity: function addTransaction(address buyer, address seller, address moderator, uint8 threshold, uint32 timeoutHours, bytes32 scriptHash, bytes20 uniqueId) payable returns()
func (_Escrow *EscrowTransactorSession) AddTransaction(buyer common.Address, seller common.Address, moderator common.Address, threshold uint8, timeoutHours uint32, scriptHash [32]byte, uniqueId [20]byte) (*types.Transaction, error) {
	return _Escrow.Contract.AddTransaction(&_Escrow.TransactOpts, buyer, seller, moderator, threshold, timeoutHours, scriptHash, uniqueId)
}

// Execute is a paid mutator transaction binding the contract method 0x94b70f73.
//
// Solidity: function execute(uint8[] sigV, bytes32[] sigR, bytes32[] sigS, bytes32 scriptHash, (address[],uint256[]) payData) returns()
func (_Escrow *EscrowTransactor) Execute(opts *bind.TransactOpts, sigV []uint8, sigR [][32]byte, sigS [][32]byte, scriptHash [32]byte, payData PayData) (*types.Transaction, error) {
	return _Escrow.contract.Transact(opts, "execute", sigV, sigR, sigS, scriptHash, payData)
}

// Execute is a paid mutator transaction binding the contract method 0x94b70f73.
//
// Solidity: function execute(uint8[] sigV, bytes32[] sigR, bytes32[] sigS, bytes32 scriptHash, (address[],uint256[]) payData) returns()
func (_Escrow *EscrowSession) Execute(sigV []uint8, sigR [][32]byte, sigS [][32]byte, scriptHash [32]byte, payData PayData) (*types.Transaction, error) {
	return _Escrow.Contract.Execute(&_Escrow.TransactOpts, sigV, sigR, sigS, scriptHash, payData)
}

// Execute is a paid mutator transaction binding the contract method 0x94b70f73.
//
// Solidity: function execute(uint8[] sigV, bytes32[] sigR, bytes32[] sigS, bytes32 scriptHash, (address[],uint256[]) payData) returns()
func (_Escrow *EscrowTransactorSession) Execute(sigV []uint8, sigR [][32]byte, sigS [][32]byte, scriptHash [32]byte, payData PayData) (*types.Transaction, error) {
	return _Escrow.Contract.Execute(&_Escrow.TransactOpts, sigV, sigR, sigS, scriptHash, payData)
}

// TransferLockedFunds is a paid mutator transaction binding the contract method 0x8fd18ad5.
//
// Solidity: function transferLockedFunds(address receiver, uint256 value, uint8 transactionType, address tokenAddress) returns()
func (_Escrow *EscrowTransactor) TransferLockedFunds(opts *bind.TransactOpts, receiver common.Address, value *big.Int, transactionType uint8, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.contract.Transact(opts, "transferLockedFunds", receiver, value, transactionType, tokenAddress)
}

// TransferLockedFunds is a paid mutator transaction binding the contract method 0x8fd18ad5.
//
// Solidity: function transferLockedFunds(address receiver, uint256 value, uint8 transactionType, address tokenAddress) returns()
func (_Escrow *EscrowSession) TransferLockedFunds(receiver common.Address, value *big.Int, transactionType uint8, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.Contract.TransferLockedFunds(&_Escrow.TransactOpts, receiver, value, transactionType, tokenAddress)
}

// TransferLockedFunds is a paid mutator transaction binding the contract method 0x8fd18ad5.
//
// Solidity: function transferLockedFunds(address receiver, uint256 value, uint8 transactionType, address tokenAddress) returns()
func (_Escrow *EscrowTransactorSession) TransferLockedFunds(receiver common.Address, value *big.Int, transactionType uint8, tokenAddress common.Address) (*types.Transaction, error) {
	return _Escrow.Contract.TransferLockedFunds(&_Escrow.TransactOpts, receiver, value, transactionType, tokenAddress)
}

// EscrowExecutedIterator is returned from FilterExecuted and is used to iterate over the raw logs and unpacked data for Executed events raised by the Escrow contract.
type EscrowExecutedIterator struct {
	Event *EscrowExecuted // Event containing the contract specifics and raw log

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
func (it *EscrowExecutedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EscrowExecuted)
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
		it.Event = new(EscrowExecuted)
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
func (it *EscrowExecutedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EscrowExecutedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EscrowExecuted represents a Executed event raised by the Escrow contract.
type EscrowExecuted struct {
	ScriptHash   [32]byte
	Destinations []common.Address
	Amounts      []*big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterExecuted is a free log retrieval operation binding the contract event 0x688e2a1b34445bcd47b0e11ba2a9c8c4d850a1831b64199b59d1c70e29701545.
//
// Solidity: event Executed(bytes32 indexed scriptHash, address[] destinations, uint256[] amounts)
func (_Escrow *EscrowFilterer) FilterExecuted(opts *bind.FilterOpts, scriptHash [][32]byte) (*EscrowExecutedIterator, error) {

	var scriptHashRule []interface{}
	for _, scriptHashItem := range scriptHash {
		scriptHashRule = append(scriptHashRule, scriptHashItem)
	}

	logs, sub, err := _Escrow.contract.FilterLogs(opts, "Executed", scriptHashRule)
	if err != nil {
		return nil, err
	}
	return &EscrowExecutedIterator{contract: _Escrow.contract, event: "Executed", logs: logs, sub: sub}, nil
}

// WatchExecuted is a free log subscription operation binding the contract event 0x688e2a1b34445bcd47b0e11ba2a9c8c4d850a1831b64199b59d1c70e29701545.
//
// Solidity: event Executed(bytes32 indexed scriptHash, address[] destinations, uint256[] amounts)
func (_Escrow *EscrowFilterer) WatchExecuted(opts *bind.WatchOpts, sink chan<- *EscrowExecuted, scriptHash [][32]byte) (event.Subscription, error) {

	var scriptHashRule []interface{}
	for _, scriptHashItem := range scriptHash {
		scriptHashRule = append(scriptHashRule, scriptHashItem)
	}

	logs, sub, err := _Escrow.contract.WatchLogs(opts, "Executed", scriptHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EscrowExecuted)
				if err := _Escrow.contract.UnpackLog(event, "Executed", log); err != nil {
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

// ParseExecuted is a log parse operation binding the contract event 0x688e2a1b34445bcd47b0e11ba2a9c8c4d850a1831b64199b59d1c70e29701545.
//
// Solidity: event Executed(bytes32 indexed scriptHash, address[] destinations, uint256[] amounts)
func (_Escrow *EscrowFilterer) ParseExecuted(log types.Log) (*EscrowExecuted, error) {
	event := new(EscrowExecuted)
	if err := _Escrow.contract.UnpackLog(event, "Executed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EscrowFundedIterator is returned from FilterFunded and is used to iterate over the raw logs and unpacked data for Funded events raised by the Escrow contract.
type EscrowFundedIterator struct {
	Event *EscrowFunded // Event containing the contract specifics and raw log

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
func (it *EscrowFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EscrowFunded)
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
		it.Event = new(EscrowFunded)
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
func (it *EscrowFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EscrowFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EscrowFunded represents a Funded event raised by the Escrow contract.
type EscrowFunded struct {
	ScriptHash [32]byte
	From       common.Address
	Value      *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterFunded is a free log retrieval operation binding the contract event 0xce7089d0668849fb9ca29778c0cbf1e764d9efb048d81fd71fb34c94f26db368.
//
// Solidity: event Funded(bytes32 indexed scriptHash, address indexed from, uint256 value)
func (_Escrow *EscrowFilterer) FilterFunded(opts *bind.FilterOpts, scriptHash [][32]byte, from []common.Address) (*EscrowFundedIterator, error) {

	var scriptHashRule []interface{}
	for _, scriptHashItem := range scriptHash {
		scriptHashRule = append(scriptHashRule, scriptHashItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _Escrow.contract.FilterLogs(opts, "Funded", scriptHashRule, fromRule)
	if err != nil {
		return nil, err
	}
	return &EscrowFundedIterator{contract: _Escrow.contract, event: "Funded", logs: logs, sub: sub}, nil
}

// WatchFunded is a free log subscription operation binding the contract event 0xce7089d0668849fb9ca29778c0cbf1e764d9efb048d81fd71fb34c94f26db368.
//
// Solidity: event Funded(bytes32 indexed scriptHash, address indexed from, uint256 value)
func (_Escrow *EscrowFilterer) WatchFunded(opts *bind.WatchOpts, sink chan<- *EscrowFunded, scriptHash [][32]byte, from []common.Address) (event.Subscription, error) {

	var scriptHashRule []interface{}
	for _, scriptHashItem := range scriptHash {
		scriptHashRule = append(scriptHashRule, scriptHashItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _Escrow.contract.WatchLogs(opts, "Funded", scriptHashRule, fromRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EscrowFunded)
				if err := _Escrow.contract.UnpackLog(event, "Funded", log); err != nil {
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

// ParseFunded is a log parse operation binding the contract event 0xce7089d0668849fb9ca29778c0cbf1e764d9efb048d81fd71fb34c94f26db368.
//
// Solidity: event Funded(bytes32 indexed scriptHash, address indexed from, uint256 value)
func (_Escrow *EscrowFilterer) ParseFunded(log types.Log) (*EscrowFunded, error) {
	event := new(EscrowFunded)
	if err := _Escrow.contract.UnpackLog(event, "Funded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
