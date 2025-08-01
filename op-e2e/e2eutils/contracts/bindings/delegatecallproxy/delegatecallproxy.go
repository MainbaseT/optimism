// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package delegatecallproxy

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

// DelegatecallproxyMetaData contains all meta data concerning the Delegatecallproxy contract.
var DelegatecallproxyMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"executeDelegateCall\",\"inputs\":[{\"name\":\"_target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"_proxyAdmin\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"error\",\"name\":\"NotOwner\",\"inputs\":[]}]",
	Bin: "0x60a0604052348015600e575f5ffd5b506040516105ae3803806105ae833981016040819052602b91603b565b6001600160a01b03166080526066565b5f60208284031215604a575f5ffd5b81516001600160a01b0381168114605f575f5ffd5b9392505050565b60805161052b6100835f395f8181605d015260e1015261052b5ff3fe608060405234801561000f575f5ffd5b506004361061003f575f3560e01c80636d435421146100435780638da5cb5b14610058578063b68df16d146100a9575b5f5ffd5b610056610051366004610356565b6100c9565b005b61007f7f000000000000000000000000000000000000000000000000000000000000000081565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b6100bc6100b73660046103b4565b6102ae565b6040516100a091906104b5565b3373ffffffffffffffffffffffffffffffffffffffff7f00000000000000000000000000000000000000000000000000000000000000001614610138576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60405173ffffffffffffffffffffffffffffffffffffffff821660248201525f90604401604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529181526020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167ff2fde38b00000000000000000000000000000000000000000000000000000000179052519091505f9073ffffffffffffffffffffffffffffffffffffffff8516906101fb908490610508565b5f604051808303815f865af19150503d805f8114610234576040519150601f19603f3d011682016040523d82523d5f602084013e610239565b606091505b50509050806102a8576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601960248201527f5472616e736665724f776e6572736869703a206661696c656400000000000000604482015260640160405180910390fd5b50505050565b60605f5f8473ffffffffffffffffffffffffffffffffffffffff16846040516102d79190610508565b5f60405180830381855af49150503d805f811461030f576040519150601f19603f3d011682016040523d82523d5f602084013e610314565b606091505b50915091508161032657805160208201fd5b949350505050565b803573ffffffffffffffffffffffffffffffffffffffff81168114610351575f5ffd5b919050565b5f5f60408385031215610367575f5ffd5b6103708361032e565b915061037e6020840161032e565b90509250929050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b5f5f604083850312156103c5575f5ffd5b6103ce8361032e565b9150602083013567ffffffffffffffff8111156103e9575f5ffd5b8301601f810185136103f9575f5ffd5b803567ffffffffffffffff81111561041357610413610387565b6040517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0603f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f8501160116810181811067ffffffffffffffff8211171561047f5761047f610387565b604052818152828201602001871015610496575f5ffd5b816020840160208301375f602083830101528093505050509250929050565b602081525f82518060208401528060208501604085015e5f6040828501015260407fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f83011684010191505092915050565b5f82518060208501845e5f92019182525091905056fea164736f6c634300081b000a",
}

// DelegatecallproxyABI is the input ABI used to generate the binding from.
// Deprecated: Use DelegatecallproxyMetaData.ABI instead.
var DelegatecallproxyABI = DelegatecallproxyMetaData.ABI

// DelegatecallproxyBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use DelegatecallproxyMetaData.Bin instead.
var DelegatecallproxyBin = DelegatecallproxyMetaData.Bin

// DeployDelegatecallproxy deploys a new Ethereum contract, binding an instance of Delegatecallproxy to it.
func DeployDelegatecallproxy(auth *bind.TransactOpts, backend bind.ContractBackend, _owner common.Address) (common.Address, *types.Transaction, *Delegatecallproxy, error) {
	parsed, err := DelegatecallproxyMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(DelegatecallproxyBin), backend, _owner)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Delegatecallproxy{DelegatecallproxyCaller: DelegatecallproxyCaller{contract: contract}, DelegatecallproxyTransactor: DelegatecallproxyTransactor{contract: contract}, DelegatecallproxyFilterer: DelegatecallproxyFilterer{contract: contract}}, nil
}

// Delegatecallproxy is an auto generated Go binding around an Ethereum contract.
type Delegatecallproxy struct {
	DelegatecallproxyCaller     // Read-only binding to the contract
	DelegatecallproxyTransactor // Write-only binding to the contract
	DelegatecallproxyFilterer   // Log filterer for contract events
}

// DelegatecallproxyCaller is an auto generated read-only Go binding around an Ethereum contract.
type DelegatecallproxyCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatecallproxyTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DelegatecallproxyTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatecallproxyFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DelegatecallproxyFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatecallproxySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DelegatecallproxySession struct {
	Contract     *Delegatecallproxy // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// DelegatecallproxyCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DelegatecallproxyCallerSession struct {
	Contract *DelegatecallproxyCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// DelegatecallproxyTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DelegatecallproxyTransactorSession struct {
	Contract     *DelegatecallproxyTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// DelegatecallproxyRaw is an auto generated low-level Go binding around an Ethereum contract.
type DelegatecallproxyRaw struct {
	Contract *Delegatecallproxy // Generic contract binding to access the raw methods on
}

// DelegatecallproxyCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DelegatecallproxyCallerRaw struct {
	Contract *DelegatecallproxyCaller // Generic read-only contract binding to access the raw methods on
}

// DelegatecallproxyTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DelegatecallproxyTransactorRaw struct {
	Contract *DelegatecallproxyTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDelegatecallproxy creates a new instance of Delegatecallproxy, bound to a specific deployed contract.
func NewDelegatecallproxy(address common.Address, backend bind.ContractBackend) (*Delegatecallproxy, error) {
	contract, err := bindDelegatecallproxy(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Delegatecallproxy{DelegatecallproxyCaller: DelegatecallproxyCaller{contract: contract}, DelegatecallproxyTransactor: DelegatecallproxyTransactor{contract: contract}, DelegatecallproxyFilterer: DelegatecallproxyFilterer{contract: contract}}, nil
}

// NewDelegatecallproxyCaller creates a new read-only instance of Delegatecallproxy, bound to a specific deployed contract.
func NewDelegatecallproxyCaller(address common.Address, caller bind.ContractCaller) (*DelegatecallproxyCaller, error) {
	contract, err := bindDelegatecallproxy(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DelegatecallproxyCaller{contract: contract}, nil
}

// NewDelegatecallproxyTransactor creates a new write-only instance of Delegatecallproxy, bound to a specific deployed contract.
func NewDelegatecallproxyTransactor(address common.Address, transactor bind.ContractTransactor) (*DelegatecallproxyTransactor, error) {
	contract, err := bindDelegatecallproxy(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DelegatecallproxyTransactor{contract: contract}, nil
}

// NewDelegatecallproxyFilterer creates a new log filterer instance of Delegatecallproxy, bound to a specific deployed contract.
func NewDelegatecallproxyFilterer(address common.Address, filterer bind.ContractFilterer) (*DelegatecallproxyFilterer, error) {
	contract, err := bindDelegatecallproxy(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DelegatecallproxyFilterer{contract: contract}, nil
}

// bindDelegatecallproxy binds a generic wrapper to an already deployed contract.
func bindDelegatecallproxy(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := DelegatecallproxyMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Delegatecallproxy *DelegatecallproxyRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Delegatecallproxy.Contract.DelegatecallproxyCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Delegatecallproxy *DelegatecallproxyRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.DelegatecallproxyTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Delegatecallproxy *DelegatecallproxyRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.DelegatecallproxyTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Delegatecallproxy *DelegatecallproxyCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Delegatecallproxy.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Delegatecallproxy *DelegatecallproxyTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Delegatecallproxy *DelegatecallproxyTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.contract.Transact(opts, method, params...)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Delegatecallproxy *DelegatecallproxyCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Delegatecallproxy.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Delegatecallproxy *DelegatecallproxySession) Owner() (common.Address, error) {
	return _Delegatecallproxy.Contract.Owner(&_Delegatecallproxy.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Delegatecallproxy *DelegatecallproxyCallerSession) Owner() (common.Address, error) {
	return _Delegatecallproxy.Contract.Owner(&_Delegatecallproxy.CallOpts)
}

// ExecuteDelegateCall is a paid mutator transaction binding the contract method 0xb68df16d.
//
// Solidity: function executeDelegateCall(address _target, bytes _data) returns(bytes)
func (_Delegatecallproxy *DelegatecallproxyTransactor) ExecuteDelegateCall(opts *bind.TransactOpts, _target common.Address, _data []byte) (*types.Transaction, error) {
	return _Delegatecallproxy.contract.Transact(opts, "executeDelegateCall", _target, _data)
}

// ExecuteDelegateCall is a paid mutator transaction binding the contract method 0xb68df16d.
//
// Solidity: function executeDelegateCall(address _target, bytes _data) returns(bytes)
func (_Delegatecallproxy *DelegatecallproxySession) ExecuteDelegateCall(_target common.Address, _data []byte) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.ExecuteDelegateCall(&_Delegatecallproxy.TransactOpts, _target, _data)
}

// ExecuteDelegateCall is a paid mutator transaction binding the contract method 0xb68df16d.
//
// Solidity: function executeDelegateCall(address _target, bytes _data) returns(bytes)
func (_Delegatecallproxy *DelegatecallproxyTransactorSession) ExecuteDelegateCall(_target common.Address, _data []byte) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.ExecuteDelegateCall(&_Delegatecallproxy.TransactOpts, _target, _data)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0x6d435421.
//
// Solidity: function transferOwnership(address _proxyAdmin, address _newOwner) returns()
func (_Delegatecallproxy *DelegatecallproxyTransactor) TransferOwnership(opts *bind.TransactOpts, _proxyAdmin common.Address, _newOwner common.Address) (*types.Transaction, error) {
	return _Delegatecallproxy.contract.Transact(opts, "transferOwnership", _proxyAdmin, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0x6d435421.
//
// Solidity: function transferOwnership(address _proxyAdmin, address _newOwner) returns()
func (_Delegatecallproxy *DelegatecallproxySession) TransferOwnership(_proxyAdmin common.Address, _newOwner common.Address) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.TransferOwnership(&_Delegatecallproxy.TransactOpts, _proxyAdmin, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0x6d435421.
//
// Solidity: function transferOwnership(address _proxyAdmin, address _newOwner) returns()
func (_Delegatecallproxy *DelegatecallproxyTransactorSession) TransferOwnership(_proxyAdmin common.Address, _newOwner common.Address) (*types.Transaction, error) {
	return _Delegatecallproxy.Contract.TransferOwnership(&_Delegatecallproxy.TransactOpts, _proxyAdmin, _newOwner)
}
