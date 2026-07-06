// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package registry

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

// QDayAnchorRegistryMetaData contains all meta data concerning the QDayAnchorRegistry contract.
var QDayAnchorRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"anchorOf\",\"inputs\":[{\"name\":\"stateRoot\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"batchNumber\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"abelTxHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"abelHeight\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"recordedAt\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isAnchored\",\"inputs\":[{\"name\":\"stateRoot\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recordAnchor\",\"inputs\":[{\"name\":\"batchNumber\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"stateRoot\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"abelTxHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"abelHeight\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"relayers\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"rootOfBatch\",\"inputs\":[{\"name\":\"\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setRelayer\",\"inputs\":[{\"name\":\"r\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"allowed\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Anchored\",\"inputs\":[{\"name\":\"batchNumber\",\"type\":\"uint64\",\"indexed\":true,\"internalType\":\"uint64\"},{\"name\":\"stateRoot\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"abelTxHash\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"},{\"name\":\"abelHeight\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RelayerSet\",\"inputs\":[{\"name\":\"relayer\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"allowed\",\"type\":\"bool\",\"indexed\":false,\"internalType\":\"bool\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AlreadyAnchored\",\"inputs\":[{\"name\":\"stateRoot\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"BatchAlreadyAnchored\",\"inputs\":[{\"name\":\"batchNumber\",\"type\":\"uint64\",\"internalType\":\"uint64\"}]},{\"type\":\"error\",\"name\":\"NotOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotRelayer\",\"inputs\":[]}]",
	Bin: "0x608060405234801561001057600080fd5b50600080546001600160a01b03191633179055610583806100326000396000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c80638143edc11161005b5780638143edc1146101065780638da5cb5b14610134578063cf4ff9011461015f578063ecd8dc3a146101fe57600080fd5b80634f0b5801146100825780635300f841146100ce5780636a8329f0146100f1575b600080fd5b6100b9610090366004610441565b60009081526002602081905260409091200154600160401b900467ffffffffffffffff16151590565b60405190151581526020015b60405180910390f35b6100b96100dc366004610476565b60016020526000908152604090205460ff1681565b6101046100ff3660046104b0565b610211565b005b6101266101143660046104f6565b60036020526000908152604090205481565b6040519081526020016100c5565b600054610147906001600160a01b031681565b6040516001600160a01b0390911681526020016100c5565b6101ca61016d366004610441565b6000908152600260208181526040928390208351608081018552815467ffffffffffffffff90811680835260018401549483018590529290940154808516958201869052600160401b900490931660609093018390529390929190565b6040805167ffffffffffffffff958616815260208101949094529184169183019190915290911660608201526080016100c5565b61010461020c366004610511565b6103b3565b3360009081526001602052604090205460ff166102415760405163c64891a560e01b815260040160405180910390fd5b60008381526002602081905260409091200154600160401b900467ffffffffffffffff161561028b576040516330d2381360e01b8152600481018490526024015b60405180910390fd5b67ffffffffffffffff8416600090815260036020526040902054156102cf576040516312aca80f60e21b815267ffffffffffffffff85166004820152602401610282565b6040805160808101825267ffffffffffffffff86811680835260208084018781528684168587018181524286166060880190815260008c815260028087528a822099518a54908a1667ffffffffffffffff19909116178a55945160018a0155915197909301805493518716600160401b026fffffffffffffffffffffffffffffffff199094169790961696909617919091179093558184526003815292849020879055835186815292830191909152859290917ff51678697efba5dcba8619d6783f7c5774ca036c8498164ee176defcdbdd4698910160405180910390a350505050565b6000546001600160a01b031633146103de576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216600081815260016020908152604091829020805460ff19168515159081179091558251938452908301527fee42947bc760229eeff964017ac68eddb00ba98b0defcd90a1bc85a5afceb057910160405180910390a15050565b60006020828403121561045357600080fd5b5035919050565b80356001600160a01b038116811461047157600080fd5b919050565b60006020828403121561048857600080fd5b6104918261045a565b9392505050565b803567ffffffffffffffff8116811461047157600080fd5b600080600080608085870312156104c657600080fd5b6104cf85610498565b935060208501359250604085013591506104eb60608601610498565b905092959194509250565b60006020828403121561050857600080fd5b61049182610498565b6000806040838503121561052457600080fd5b61052d8361045a565b91506020830135801515811461054257600080fd5b80915050925092905056fea26469706673582212204c8f5fa89c743b690b77feaa77b7be7fa01975858c6fc0366ff85e04f159ed2d64736f6c63430008180033",
}

// QDayAnchorRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use QDayAnchorRegistryMetaData.ABI instead.
var QDayAnchorRegistryABI = QDayAnchorRegistryMetaData.ABI

// QDayAnchorRegistryBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use QDayAnchorRegistryMetaData.Bin instead.
var QDayAnchorRegistryBin = QDayAnchorRegistryMetaData.Bin

// DeployQDayAnchorRegistry deploys a new Ethereum contract, binding an instance of QDayAnchorRegistry to it.
func DeployQDayAnchorRegistry(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *QDayAnchorRegistry, error) {
	parsed, err := QDayAnchorRegistryMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(QDayAnchorRegistryBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &QDayAnchorRegistry{QDayAnchorRegistryCaller: QDayAnchorRegistryCaller{contract: contract}, QDayAnchorRegistryTransactor: QDayAnchorRegistryTransactor{contract: contract}, QDayAnchorRegistryFilterer: QDayAnchorRegistryFilterer{contract: contract}}, nil
}

// QDayAnchorRegistry is an auto generated Go binding around an Ethereum contract.
type QDayAnchorRegistry struct {
	QDayAnchorRegistryCaller     // Read-only binding to the contract
	QDayAnchorRegistryTransactor // Write-only binding to the contract
	QDayAnchorRegistryFilterer   // Log filterer for contract events
}

// QDayAnchorRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type QDayAnchorRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// QDayAnchorRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type QDayAnchorRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// QDayAnchorRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type QDayAnchorRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// QDayAnchorRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type QDayAnchorRegistrySession struct {
	Contract     *QDayAnchorRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// QDayAnchorRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type QDayAnchorRegistryCallerSession struct {
	Contract *QDayAnchorRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// QDayAnchorRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type QDayAnchorRegistryTransactorSession struct {
	Contract     *QDayAnchorRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// QDayAnchorRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type QDayAnchorRegistryRaw struct {
	Contract *QDayAnchorRegistry // Generic contract binding to access the raw methods on
}

// QDayAnchorRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type QDayAnchorRegistryCallerRaw struct {
	Contract *QDayAnchorRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// QDayAnchorRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type QDayAnchorRegistryTransactorRaw struct {
	Contract *QDayAnchorRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewQDayAnchorRegistry creates a new instance of QDayAnchorRegistry, bound to a specific deployed contract.
func NewQDayAnchorRegistry(address common.Address, backend bind.ContractBackend) (*QDayAnchorRegistry, error) {
	contract, err := bindQDayAnchorRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistry{QDayAnchorRegistryCaller: QDayAnchorRegistryCaller{contract: contract}, QDayAnchorRegistryTransactor: QDayAnchorRegistryTransactor{contract: contract}, QDayAnchorRegistryFilterer: QDayAnchorRegistryFilterer{contract: contract}}, nil
}

// NewQDayAnchorRegistryCaller creates a new read-only instance of QDayAnchorRegistry, bound to a specific deployed contract.
func NewQDayAnchorRegistryCaller(address common.Address, caller bind.ContractCaller) (*QDayAnchorRegistryCaller, error) {
	contract, err := bindQDayAnchorRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistryCaller{contract: contract}, nil
}

// NewQDayAnchorRegistryTransactor creates a new write-only instance of QDayAnchorRegistry, bound to a specific deployed contract.
func NewQDayAnchorRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*QDayAnchorRegistryTransactor, error) {
	contract, err := bindQDayAnchorRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistryTransactor{contract: contract}, nil
}

// NewQDayAnchorRegistryFilterer creates a new log filterer instance of QDayAnchorRegistry, bound to a specific deployed contract.
func NewQDayAnchorRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*QDayAnchorRegistryFilterer, error) {
	contract, err := bindQDayAnchorRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistryFilterer{contract: contract}, nil
}

// bindQDayAnchorRegistry binds a generic wrapper to an already deployed contract.
func bindQDayAnchorRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := QDayAnchorRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_QDayAnchorRegistry *QDayAnchorRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _QDayAnchorRegistry.Contract.QDayAnchorRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_QDayAnchorRegistry *QDayAnchorRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.QDayAnchorRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_QDayAnchorRegistry *QDayAnchorRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.QDayAnchorRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _QDayAnchorRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.contract.Transact(opts, method, params...)
}

// AnchorOf is a free data retrieval call binding the contract method 0xcf4ff901.
//
// Solidity: function anchorOf(bytes32 stateRoot) view returns(uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt)
func (_QDayAnchorRegistry *QDayAnchorRegistryCaller) AnchorOf(opts *bind.CallOpts, stateRoot [32]byte) (struct {
	BatchNumber uint64
	AbelTxHash  [32]byte
	AbelHeight  uint64
	RecordedAt  uint64
}, error) {
	var out []interface{}
	err := _QDayAnchorRegistry.contract.Call(opts, &out, "anchorOf", stateRoot)

	outstruct := new(struct {
		BatchNumber uint64
		AbelTxHash  [32]byte
		AbelHeight  uint64
		RecordedAt  uint64
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.BatchNumber = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.AbelTxHash = *abi.ConvertType(out[1], new([32]byte)).(*[32]byte)
	outstruct.AbelHeight = *abi.ConvertType(out[2], new(uint64)).(*uint64)
	outstruct.RecordedAt = *abi.ConvertType(out[3], new(uint64)).(*uint64)

	return *outstruct, err

}

// AnchorOf is a free data retrieval call binding the contract method 0xcf4ff901.
//
// Solidity: function anchorOf(bytes32 stateRoot) view returns(uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt)
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) AnchorOf(stateRoot [32]byte) (struct {
	BatchNumber uint64
	AbelTxHash  [32]byte
	AbelHeight  uint64
	RecordedAt  uint64
}, error) {
	return _QDayAnchorRegistry.Contract.AnchorOf(&_QDayAnchorRegistry.CallOpts, stateRoot)
}

// AnchorOf is a free data retrieval call binding the contract method 0xcf4ff901.
//
// Solidity: function anchorOf(bytes32 stateRoot) view returns(uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt)
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerSession) AnchorOf(stateRoot [32]byte) (struct {
	BatchNumber uint64
	AbelTxHash  [32]byte
	AbelHeight  uint64
	RecordedAt  uint64
}, error) {
	return _QDayAnchorRegistry.Contract.AnchorOf(&_QDayAnchorRegistry.CallOpts, stateRoot)
}

// IsAnchored is a free data retrieval call binding the contract method 0x4f0b5801.
//
// Solidity: function isAnchored(bytes32 stateRoot) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistryCaller) IsAnchored(opts *bind.CallOpts, stateRoot [32]byte) (bool, error) {
	var out []interface{}
	err := _QDayAnchorRegistry.contract.Call(opts, &out, "isAnchored", stateRoot)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAnchored is a free data retrieval call binding the contract method 0x4f0b5801.
//
// Solidity: function isAnchored(bytes32 stateRoot) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) IsAnchored(stateRoot [32]byte) (bool, error) {
	return _QDayAnchorRegistry.Contract.IsAnchored(&_QDayAnchorRegistry.CallOpts, stateRoot)
}

// IsAnchored is a free data retrieval call binding the contract method 0x4f0b5801.
//
// Solidity: function isAnchored(bytes32 stateRoot) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerSession) IsAnchored(stateRoot [32]byte) (bool, error) {
	return _QDayAnchorRegistry.Contract.IsAnchored(&_QDayAnchorRegistry.CallOpts, stateRoot)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_QDayAnchorRegistry *QDayAnchorRegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _QDayAnchorRegistry.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) Owner() (common.Address, error) {
	return _QDayAnchorRegistry.Contract.Owner(&_QDayAnchorRegistry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerSession) Owner() (common.Address, error) {
	return _QDayAnchorRegistry.Contract.Owner(&_QDayAnchorRegistry.CallOpts)
}

// Relayers is a free data retrieval call binding the contract method 0x5300f841.
//
// Solidity: function relayers(address ) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistryCaller) Relayers(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _QDayAnchorRegistry.contract.Call(opts, &out, "relayers", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Relayers is a free data retrieval call binding the contract method 0x5300f841.
//
// Solidity: function relayers(address ) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) Relayers(arg0 common.Address) (bool, error) {
	return _QDayAnchorRegistry.Contract.Relayers(&_QDayAnchorRegistry.CallOpts, arg0)
}

// Relayers is a free data retrieval call binding the contract method 0x5300f841.
//
// Solidity: function relayers(address ) view returns(bool)
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerSession) Relayers(arg0 common.Address) (bool, error) {
	return _QDayAnchorRegistry.Contract.Relayers(&_QDayAnchorRegistry.CallOpts, arg0)
}

// RootOfBatch is a free data retrieval call binding the contract method 0x8143edc1.
//
// Solidity: function rootOfBatch(uint64 ) view returns(bytes32)
func (_QDayAnchorRegistry *QDayAnchorRegistryCaller) RootOfBatch(opts *bind.CallOpts, arg0 uint64) ([32]byte, error) {
	var out []interface{}
	err := _QDayAnchorRegistry.contract.Call(opts, &out, "rootOfBatch", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// RootOfBatch is a free data retrieval call binding the contract method 0x8143edc1.
//
// Solidity: function rootOfBatch(uint64 ) view returns(bytes32)
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) RootOfBatch(arg0 uint64) ([32]byte, error) {
	return _QDayAnchorRegistry.Contract.RootOfBatch(&_QDayAnchorRegistry.CallOpts, arg0)
}

// RootOfBatch is a free data retrieval call binding the contract method 0x8143edc1.
//
// Solidity: function rootOfBatch(uint64 ) view returns(bytes32)
func (_QDayAnchorRegistry *QDayAnchorRegistryCallerSession) RootOfBatch(arg0 uint64) ([32]byte, error) {
	return _QDayAnchorRegistry.Contract.RootOfBatch(&_QDayAnchorRegistry.CallOpts, arg0)
}

// RecordAnchor is a paid mutator transaction binding the contract method 0x6a8329f0.
//
// Solidity: function recordAnchor(uint64 batchNumber, bytes32 stateRoot, bytes32 abelTxHash, uint64 abelHeight) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactor) RecordAnchor(opts *bind.TransactOpts, batchNumber uint64, stateRoot [32]byte, abelTxHash [32]byte, abelHeight uint64) (*types.Transaction, error) {
	return _QDayAnchorRegistry.contract.Transact(opts, "recordAnchor", batchNumber, stateRoot, abelTxHash, abelHeight)
}

// RecordAnchor is a paid mutator transaction binding the contract method 0x6a8329f0.
//
// Solidity: function recordAnchor(uint64 batchNumber, bytes32 stateRoot, bytes32 abelTxHash, uint64 abelHeight) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) RecordAnchor(batchNumber uint64, stateRoot [32]byte, abelTxHash [32]byte, abelHeight uint64) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.RecordAnchor(&_QDayAnchorRegistry.TransactOpts, batchNumber, stateRoot, abelTxHash, abelHeight)
}

// RecordAnchor is a paid mutator transaction binding the contract method 0x6a8329f0.
//
// Solidity: function recordAnchor(uint64 batchNumber, bytes32 stateRoot, bytes32 abelTxHash, uint64 abelHeight) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactorSession) RecordAnchor(batchNumber uint64, stateRoot [32]byte, abelTxHash [32]byte, abelHeight uint64) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.RecordAnchor(&_QDayAnchorRegistry.TransactOpts, batchNumber, stateRoot, abelTxHash, abelHeight)
}

// SetRelayer is a paid mutator transaction binding the contract method 0xecd8dc3a.
//
// Solidity: function setRelayer(address r, bool allowed) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactor) SetRelayer(opts *bind.TransactOpts, r common.Address, allowed bool) (*types.Transaction, error) {
	return _QDayAnchorRegistry.contract.Transact(opts, "setRelayer", r, allowed)
}

// SetRelayer is a paid mutator transaction binding the contract method 0xecd8dc3a.
//
// Solidity: function setRelayer(address r, bool allowed) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistrySession) SetRelayer(r common.Address, allowed bool) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.SetRelayer(&_QDayAnchorRegistry.TransactOpts, r, allowed)
}

// SetRelayer is a paid mutator transaction binding the contract method 0xecd8dc3a.
//
// Solidity: function setRelayer(address r, bool allowed) returns()
func (_QDayAnchorRegistry *QDayAnchorRegistryTransactorSession) SetRelayer(r common.Address, allowed bool) (*types.Transaction, error) {
	return _QDayAnchorRegistry.Contract.SetRelayer(&_QDayAnchorRegistry.TransactOpts, r, allowed)
}

// QDayAnchorRegistryAnchoredIterator is returned from FilterAnchored and is used to iterate over the raw logs and unpacked data for Anchored events raised by the QDayAnchorRegistry contract.
type QDayAnchorRegistryAnchoredIterator struct {
	Event *QDayAnchorRegistryAnchored // Event containing the contract specifics and raw log

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
func (it *QDayAnchorRegistryAnchoredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(QDayAnchorRegistryAnchored)
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
		it.Event = new(QDayAnchorRegistryAnchored)
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
func (it *QDayAnchorRegistryAnchoredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *QDayAnchorRegistryAnchoredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// QDayAnchorRegistryAnchored represents a Anchored event raised by the QDayAnchorRegistry contract.
type QDayAnchorRegistryAnchored struct {
	BatchNumber uint64
	StateRoot   [32]byte
	AbelTxHash  [32]byte
	AbelHeight  uint64
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterAnchored is a free log retrieval operation binding the contract event 0xf51678697efba5dcba8619d6783f7c5774ca036c8498164ee176defcdbdd4698.
//
// Solidity: event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot, bytes32 abelTxHash, uint64 abelHeight)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) FilterAnchored(opts *bind.FilterOpts, batchNumber []uint64, stateRoot [][32]byte) (*QDayAnchorRegistryAnchoredIterator, error) {

	var batchNumberRule []interface{}
	for _, batchNumberItem := range batchNumber {
		batchNumberRule = append(batchNumberRule, batchNumberItem)
	}
	var stateRootRule []interface{}
	for _, stateRootItem := range stateRoot {
		stateRootRule = append(stateRootRule, stateRootItem)
	}

	logs, sub, err := _QDayAnchorRegistry.contract.FilterLogs(opts, "Anchored", batchNumberRule, stateRootRule)
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistryAnchoredIterator{contract: _QDayAnchorRegistry.contract, event: "Anchored", logs: logs, sub: sub}, nil
}

// WatchAnchored is a free log subscription operation binding the contract event 0xf51678697efba5dcba8619d6783f7c5774ca036c8498164ee176defcdbdd4698.
//
// Solidity: event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot, bytes32 abelTxHash, uint64 abelHeight)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) WatchAnchored(opts *bind.WatchOpts, sink chan<- *QDayAnchorRegistryAnchored, batchNumber []uint64, stateRoot [][32]byte) (event.Subscription, error) {

	var batchNumberRule []interface{}
	for _, batchNumberItem := range batchNumber {
		batchNumberRule = append(batchNumberRule, batchNumberItem)
	}
	var stateRootRule []interface{}
	for _, stateRootItem := range stateRoot {
		stateRootRule = append(stateRootRule, stateRootItem)
	}

	logs, sub, err := _QDayAnchorRegistry.contract.WatchLogs(opts, "Anchored", batchNumberRule, stateRootRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(QDayAnchorRegistryAnchored)
				if err := _QDayAnchorRegistry.contract.UnpackLog(event, "Anchored", log); err != nil {
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

// ParseAnchored is a log parse operation binding the contract event 0xf51678697efba5dcba8619d6783f7c5774ca036c8498164ee176defcdbdd4698.
//
// Solidity: event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot, bytes32 abelTxHash, uint64 abelHeight)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) ParseAnchored(log types.Log) (*QDayAnchorRegistryAnchored, error) {
	event := new(QDayAnchorRegistryAnchored)
	if err := _QDayAnchorRegistry.contract.UnpackLog(event, "Anchored", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// QDayAnchorRegistryRelayerSetIterator is returned from FilterRelayerSet and is used to iterate over the raw logs and unpacked data for RelayerSet events raised by the QDayAnchorRegistry contract.
type QDayAnchorRegistryRelayerSetIterator struct {
	Event *QDayAnchorRegistryRelayerSet // Event containing the contract specifics and raw log

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
func (it *QDayAnchorRegistryRelayerSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(QDayAnchorRegistryRelayerSet)
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
		it.Event = new(QDayAnchorRegistryRelayerSet)
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
func (it *QDayAnchorRegistryRelayerSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *QDayAnchorRegistryRelayerSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// QDayAnchorRegistryRelayerSet represents a RelayerSet event raised by the QDayAnchorRegistry contract.
type QDayAnchorRegistryRelayerSet struct {
	Relayer common.Address
	Allowed bool
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRelayerSet is a free log retrieval operation binding the contract event 0xee42947bc760229eeff964017ac68eddb00ba98b0defcd90a1bc85a5afceb057.
//
// Solidity: event RelayerSet(address relayer, bool allowed)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) FilterRelayerSet(opts *bind.FilterOpts) (*QDayAnchorRegistryRelayerSetIterator, error) {

	logs, sub, err := _QDayAnchorRegistry.contract.FilterLogs(opts, "RelayerSet")
	if err != nil {
		return nil, err
	}
	return &QDayAnchorRegistryRelayerSetIterator{contract: _QDayAnchorRegistry.contract, event: "RelayerSet", logs: logs, sub: sub}, nil
}

// WatchRelayerSet is a free log subscription operation binding the contract event 0xee42947bc760229eeff964017ac68eddb00ba98b0defcd90a1bc85a5afceb057.
//
// Solidity: event RelayerSet(address relayer, bool allowed)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) WatchRelayerSet(opts *bind.WatchOpts, sink chan<- *QDayAnchorRegistryRelayerSet) (event.Subscription, error) {

	logs, sub, err := _QDayAnchorRegistry.contract.WatchLogs(opts, "RelayerSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(QDayAnchorRegistryRelayerSet)
				if err := _QDayAnchorRegistry.contract.UnpackLog(event, "RelayerSet", log); err != nil {
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

// ParseRelayerSet is a log parse operation binding the contract event 0xee42947bc760229eeff964017ac68eddb00ba98b0defcd90a1bc85a5afceb057.
//
// Solidity: event RelayerSet(address relayer, bool allowed)
func (_QDayAnchorRegistry *QDayAnchorRegistryFilterer) ParseRelayerSet(log types.Log) (*QDayAnchorRegistryRelayerSet, error) {
	event := new(QDayAnchorRegistryRelayerSet)
	if err := _QDayAnchorRegistry.contract.UnpackLog(event, "RelayerSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
