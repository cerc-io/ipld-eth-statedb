package ipld_eth_statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/temp/common"
)

/*
The portions of the EVM we want to leverage only use the following methods:

GetBalance
Snapshot
Exist
CreateAccount
SubBalance
AddBalance
GetCode
GetCodeHash
RevertToSnapshot
GetNonce
SetNonce
AddAddressToAccessList
SetCode

The rest can be left with panics for now
 */

var _ vm.StateDB = &StateDB{}

type StateDB struct {
}

func (s StateDB) CreateAccount(address common.Address) {
	panic("implement me")
}

func (s StateDB) SubBalance(address common.Address, b *big.Int) {
	panic("implement me")
}

func (s StateDB) AddBalance(address common.Address, b *big.Int) {
	panic("implement me")
}

func (s StateDB) GetBalance(address common.Address) *big.Int {
	panic("implement me")
}

func (s StateDB) GetNonce(address common.Address) uint64 {
	panic("implement me")
}

func (s StateDB) SetNonce(address common.Address, u uint64) {
	panic("implement me")
}

func (s StateDB) GetCodeHash(address common.Address) common.Hash {
	panic("implement me")
}

func (s StateDB) GetCode(address common.Address) []byte {
	panic("implement me")
}

func (s StateDB) SetCode(address common.Address, bytes []byte) {
	panic("implement me")
}

func (s StateDB) GetCodeSize(address common.Address) int {
	panic("implement me")
}

func (s StateDB) AddRefund(u uint64) {
	panic("implement me")
}

func (s StateDB) SubRefund(u uint64) {
	panic("implement me")
}

func (s StateDB) GetRefund() uint64 {
	panic("implement me")
}

func (s StateDB) GetCommittedState(address common.Address, hash common.Hash) common.Hash {
	panic("implement me")
}

func (s StateDB) GetState(address common.Address, hash common.Hash) common.Hash {
	panic("implement me")
}

func (s StateDB) SetState(address common.Address, hash common.Hash, hash2 common.Hash) {
	panic("implement me")
}

func (s StateDB) Suicide(address common.Address) bool {
	panic("implement me")
}

func (s StateDB) HasSuicided(address common.Address) bool {
	panic("implement me")
}

func (s StateDB) Exist(address common.Address) bool {
	panic("implement me")
}

func (s StateDB) Empty(address common.Address) bool {
	panic("implement me")
}

func (s StateDB) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	panic("implement me")
}

func (s StateDB) AddressInAccessList(addr common.Address) bool {
	panic("implement me")
}

func (s StateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	panic("implement me")
}

func (s StateDB) AddAddressToAccessList(addr common.Address) {
	panic("implement me")
}

func (s StateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	panic("implement me")
}

func (s StateDB) RevertToSnapshot(i int) {
	panic("implement me")
}

func (s StateDB) Snapshot() int {
	panic("implement me")
}

func (s StateDB) AddLog(log *types.Log) {
	panic("implement me")
}

func (s StateDB) AddPreimage(hash common.Hash, bytes []byte) {
	panic("implement me")
}

func (s StateDB) ForEachStorage(address common.Address, f func(common.Hash, common.Hash) bool) error {
	panic("implement me")
}
